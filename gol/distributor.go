package gol

import (
	"fmt"
	"net"
	"net/rpc"
	"os"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	ioKeyPress <-chan rune
}

var (
	distributorRegistered bool
	channels              distributorChannels
	pauseFlag             bool
)

type Distributor struct{}

func startGame(p Params, c distributorChannels) {
	worldSlice := createWorld(p.ImageHeight, p.ImageWidth)
	initialWorld := getImage(p, c, worldSlice)

	c.events <- CellsFlipped{CompletedTurns: 0, Cells: getAliveCells(initialWorld, p.ImageWidth, p.ImageHeight)}
	c.events <- StateChange{0, Executing}

	finalWorld, turn := gameOfLifeController(p, c, initialWorld)

	if turn == p.Turns {
		aliveCells := getAliveCells(finalWorld, p.ImageWidth, p.ImageHeight)
		c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: aliveCells}
		writeImage(p, c, p.Turns, finalWorld)

		// Ensure IO completion before exiting
		c.ioCommand <- ioCheckIdle
		<-c.ioIdle

		c.events <- StateChange{p.Turns, Quitting}
		close(c.events)
	}
}

func gameOfLifeController(p Params, c distributorChannels, initialWorld [][]uint8) ([][]uint8, int) {
	defer func() {
		pauseFlag = false
	}()
	ticker := time.NewTicker(2 * time.Second)
	client, _ := rpc.Dial("tcp", "127.0.0.1:8083")
	defer client.Close()

	request := stubs.Request{
		World: initialWorld,
		Params: stubs.Params{
			Turns:       p.Turns,
			Threads:     p.Threads,
			ImageWidth:  p.ImageWidth,
			ImageHeight: p.ImageHeight,
		},
	}
	response := new(stubs.Response)
	done := client.Go(stubs.BrokerHandler, request, response, nil)

	for {
		select {
		case <-done.Done:
			ticker.Stop()
			return response.World, response.Turn
		case <-ticker.C:
			sendAliveCellsCount(client, c)
		case key := <-c.ioKeyPress:
			res := handleKeyPress(p, c, client, key)
			if key == 'q' || key == 'k' {
				ticker.Stop()
				return res.CurrentWorld, res.Turn
			}
		}
	}
}

func sendAliveCellsCount(client *rpc.Client, c distributorChannels) {
	request := stubs.BlankRequest{}
	response := new(stubs.CurrentStateResponse)
	err := client.Call(stubs.GetCurrentState, request, response)
	if err != nil {
		fmt.Printf("Error GetCurrentState -> %s\n", err.Error())
		os.Exit(1)
	}
	c.events <- AliveCellsCount{CompletedTurns: response.Turn, CellsCount: response.AliveCellsCount}
}

func handleKeyPress(p Params, c distributorChannels, client *rpc.Client, key rune) *stubs.CurrentStateResponse {
	switch key {
	case 's':
		saveCurrentState(client, p, c)
	case 'q', 'k':
		return quitOrShutdownGame(p, c, client, key)
	case 'p':
		pauseGame(p, c, client)
	default:
		fmt.Println("Invalid key")
	}

	return nil
}

func saveCurrentState(client *rpc.Client, p Params, c distributorChannels) {
	request := stubs.BlankRequest{}
	response := new(stubs.CurrentStateResponse)
	err := client.Call(stubs.GetCurrentState, request, response)
	if err != nil {
		fmt.Printf("Error GetCurrentState -> %s\n", err.Error())
		os.Exit(1)
	}
	fmt.Println("ssssssssssssssssssssave")
	if response.Turn == 0 {
		writeImage(p, c, 0, createWorld(p.ImageHeight, p.ImageWidth))
	} else {
		writeImage(p, c, response.Turn, response.CurrentWorld)
	}

}

func quitOrShutdownGame(p Params, c distributorChannels, client *rpc.Client, key rune) *stubs.CurrentStateResponse {
	keyRequest := stubs.KeyRequest{Key: "q"}
	keyResponse := new(stubs.CurrentStateResponse)
	err := client.Call(stubs.HandleKey, keyRequest, keyResponse)
	if err != nil {
		fmt.Printf("Error HandleKey -> %s\n", err.Error())
		os.Exit(1)
	}
	fmt.Println("qqqqqqqqqqqqqqqqqqsave")
	if keyResponse.Turn == 0 {
		writeImage(p, c, 0, createWorld(p.ImageHeight, p.ImageWidth))
	} else {
		writeImage(p, c, keyResponse.Turn, keyResponse.CurrentWorld)
	}

	c.events <- StateChange{CompletedTurns: keyResponse.Turn, NewState: Quitting}
	// close(c.events)

	if key == 'k' {
		shutdownBrokerAndNodes(client)
	}

	return keyResponse
}

func shutdownBrokerAndNodes(client *rpc.Client) {
	shutDownRequest := stubs.KeyRequest{Key: "k"}
	shutDownResponse := new(stubs.CurrentStateResponse)
	done := client.Go(stubs.HandleKey, shutDownRequest, shutDownResponse, nil)
	<-done.Done
	time.Sleep(500 * time.Millisecond)
}

func pauseGame(p Params, c distributorChannels, client *rpc.Client) {
	// togglePause(client)
	// c.events <- StateChange{CompletedTurns: p.Turns, NewState: Paused}
	// fmt.Println("Game paused")

	// for {
	// 	if <-c.ioKeyPress == 'p' {
	// 		togglePause(client)
	// 		c.events <- StateChange{CompletedTurns: p.Turns, NewState: Executing}
	// 		fmt.Println("Game resumed")
	// 		break
	// 	}
	// }
	pauseFlag = !pauseFlag
	if pauseFlag {
		trun := togglePause(client)
		c.events <- StateChange{CompletedTurns: trun, NewState: Paused}
		fmt.Println("Game paused")
	} else {
		trun := togglePause(client)
		c.events <- StateChange{CompletedTurns: trun, NewState: Executing}
		fmt.Println("Game resumed")
	}

}

func togglePause(client *rpc.Client) int {
	request := stubs.KeyRequest{Key: "p"}
	response := new(stubs.CurrentStateResponse)
	err := client.Call(stubs.HandleKey, request, response)
	if err != nil {
		fmt.Printf("Error HandleKey -> %s\n", err.Error())
		os.Exit(1)
	}

	return response.Turn
}

func (d *Distributor) HandleFlipCells(request stubs.FlipRequest, response *stubs.Response) error {
	oldWorld := request.OldWorld
	newWorld := request.NewWorld
	turn := request.Turn

	for i := range oldWorld {
		for j := range oldWorld[i] {
			if oldWorld[i][j] != newWorld[i][j] {
				channels.events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: j, Y: i}}
			}
		}
	}

	channels.events <- TurnComplete{CompletedTurns: turn}
	return nil
}

func distributor(p Params, c distributorChannels) {
	channels = c

	if !distributorRegistered {
		if err := rpc.Register(&Distributor{}); err != nil {
			fmt.Println("Error registering distributor:", err)
			return
		}
		distributorRegistered = true
	}

	listenOnPortAndStartGame("127.0.0.1:8082", p, c)

}

func listenOnPortAndStartGame(addr string, p Params, c distributorChannels) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("Error listening on %s: %v\n", addr, err)
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Println("Distributor running on port:", addr)
	go rpc.Accept(listener)
	startGame(p, c)
}
