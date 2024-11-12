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

// •	startGame: 初始化游戏世界，并控制游戏的启动和终止。
// •	gameOfLifeController: 游戏的核心控制逻辑，包括轮询远程计算节点的结果、处理用户输入、以及暂停和恢复游戏。
// •	sendAliveCellsCount: 定期请求节点报告当前活细胞的数量。
// •	handleKeyPress: 处理用户输入，包括保存、暂停、退出等操作。
// •	saveCurrentState: 保存当前的世界状态到文件。
// •	pauseGame 和 togglePause: 控制游戏的暂停和恢复。
// •	HandleFlipCells: 用于处理来自远程节点的细胞翻转事件。
// •	distributor: 分发器的入口，注册 RPC 服务并监听端口。
// •	listenOnPortAndStartGame: 启动 RPC 服务并监听来自计算节点的请求。

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

// •	功能：初始化游戏世界，启动游戏控制器，并在游戏结束时保存世界状态。
// •	流程：
// 1.	初始化世界 initialWorld。
// 2.	调用 gameOfLifeController 控制游戏逻辑。
// 3.	游戏完成后，保存最终状态并通知 ioCommand 通道结束 I/O 操作。
func startGame(p Params, c distributorChannels) {
	worldSlice := createWorld(p.ImageHeight, p.ImageWidth)
	initialWorld := getImage(p, c, worldSlice)

	c.events <- CellsFlipped{CompletedTurns: 0, Cells: getAliveCells(initialWorld, p.ImageWidth, p.ImageHeight)}
	c.events <- StateChange{0, Executing}

	finalWorld, turn := gameOfLifeController(p, c, initialWorld)

	// 游戏结束，如果正常结束则发送最后轮次的事件，统计存活细胞，写输出文件
	// 如果轮次不是正常结束的轮次，测试按键k或者q退出，controller里面处理的当时退出的状态
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

// •	功能：控制游戏的生命周期，包括定时获取活细胞数量、处理用户输入、暂停/恢复等操作。
// •	流程：
// 1.	创建 RPC 客户端连接。
// 2.	启动远程调用以计算 Game of Life 的下一步状态。
// 3.	定时（每2秒）请求活细胞数量。
// 4.	响应用户的按键操作（保存、暂停、退出等）。
// 5.	游戏完成或用户退出时停止 ticker 并返回当前世界状态。
func gameOfLifeController(p Params, c distributorChannels, initialWorld [][]uint8) ([][]uint8, int) {
	defer func() {
		// 退出时候，置默认暂停为false
		pauseFlag = false
	}()

	// 启动一个2s的定时器
	ticker := time.NewTicker(2 * time.Second)

	// 与broker建立连接
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

	// 向borker发送请求，处理游戏细胞状态
	response := new(stubs.Response)
	done := client.Go(stubs.BrokerHandler, request, response, nil)

	for {
		select {
		case <-done.Done:
			// 收到broker返回，游戏执行完成，停止定时器，返回最终状态
			ticker.Stop()
			return response.World, response.Turn
		case <-ticker.C:
			// 定时统计存活细胞数量
			sendAliveCellsCount(client, c)
		case key := <-c.ioKeyPress:
			// 处理按键事件
			res := handleKeyPress(p, c, client, key)
			if key == 'q' || key == 'k' {
				ticker.Stop()
				return res.CurrentWorld, res.Turn
			}
		}
	}
}

// 	•	功能：请求远程节点获取当前的活细胞数量，并将其发送到事件通道。
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

// •	功能：处理用户输入的按键。
// •	操作：
// •	's'：保存当前世界状态。
// •	'q' 或 'k'：退出或关闭整个系统。
// •	'p'：暂停/恢复游戏。
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

// •	's'：保存当前世界状态。
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

// 	•	'q' 或 'k'：退出或关闭整个系统。
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

//	•	'p'：暂停/恢复游戏。
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

// 处理远程计算节点返回的细胞翻转信息，并通知事件通道。
// 收到细胞翻转事件，想sdl发送细胞翻转，刷新sdl页面
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

// 注册分发器服务，并启动监听以处理远程请求。
func distributor(p Params, c distributorChannels) {
	channels = c

	// 主要注册Distributor对象，rpc监听HandleFlipCells事件
	if !distributorRegistered {
		if err := rpc.Register(&Distributor{}); err != nil {
			fmt.Println("Error registering distributor:", err)
			return
		}
		distributorRegistered = true
	}

	// 监听rpc端口并且开始游戏
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
	// 开始游戏
	startGame(p, c)
}
