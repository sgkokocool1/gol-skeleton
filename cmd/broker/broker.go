package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
)

type Broker struct {
	nodeAddresses []string
}

var world [][]uint8
var mutex sync.Mutex
var totalTurns int
var turn int
var shutdownFlag bool
var pauseFlag bool

func main() {
	pAddr := flag.String("port", "127.0.0.1:8083", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	broker := NewBroker([]string{
		"127.0.0.1:8085",
		//"127.0.0.1:8086",
	})

	// Register the broker
	err := rpc.Register(broker)
	if err != nil {
		fmt.Println("Error registering broker:", err)
		return
	}

	listener, err := net.Listen("tcp", *pAddr)
	if err != nil {
		fmt.Println("Error starting listener:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Broker running on port:", *pAddr)

	// Handle broker shutdown in a separate goroutine
	go broker.shutdownWatcher()

	// Start accepting RPC calls
	rpc.Accept(listener)
}

// NewBroker creates and initializes a new Broker instance.
func NewBroker(nodeAddresses []string) *Broker {
	return &Broker{
		nodeAddresses: nodeAddresses,
	}
}

func (b *Broker) shutdownWatcher() {
	for {
		time.Sleep(100 * time.Millisecond)
		mutex.Lock()
		if shutdownFlag {
			mutex.Unlock()
			fmt.Println("Shutting down the broker...")
			os.Exit(0)
		}
		mutex.Unlock()
	}
}

// HandleBroker distributes the world update workload among nodes and manages their responses.
func (b *Broker) HandleBroker(request stubs.Request, response *stubs.Response) error {
	world = request.World
	totalTurns = request.Params.Turns
	numNodes := len(b.nodeAddresses)
	workerHeight := len(world) / numNodes
	remaining := len(world) % numNodes

	channels := make([]chan [][]uint8, numNodes)
	for turn = 0; turn < totalTurns; {
		updatedWorld := b.distributeWork(numNodes, workerHeight, remaining, request, channels)

		// Update the world state and notify distributor
		mutex.Lock()
		b.callDistributor(updatedWorld)
		world = updatedWorld
		turn++
		mutex.Unlock()

		// Pause if needed
		b.waitIfPaused()
	}

	response.Status = "OK"
	response.World = world
	return nil
}

// distributeWork distributes the workload to worker nodes.
func (b *Broker) distributeWork(numNodes, workerHeight, remaining int, request stubs.Request, channels []chan [][]uint8) [][]uint8 {
	updatedWorld := make([][]uint8, 0)
	for i := 0; i < numNodes; i++ {
		channels[i] = make(chan [][]uint8)
		startY := i * workerHeight
		endY := ((i + 1) * workerHeight) + remaining
		nodeWorld := GetImagePart(request.Params, startY, endY, world)

		go b.callNode(b.nodeAddresses[i], endY-startY, nodeWorld, channels[i])
	}

	// Gather results from channels
	for i := 0; i < numNodes; i++ {
		receivedData := <-channels[i]
		updatedWorld = append(updatedWorld, receivedData...)
	}
	return updatedWorld
}

// callNode handles the RPC call to a node and collects the result.
func (b *Broker) callNode(address string, height int, nodeWorld [][]uint8, out chan [][]uint8) {
	client, err := rpc.Dial("tcp", address)
	if err != nil {
		fmt.Println("Error connecting to node:", address, "Details:", err)
		return
	}
	defer client.Close()

	request := stubs.Request{World: nodeWorld}
	response := new(stubs.Response)
	err = client.Call(stubs.HandleWorker, request, response)
	if err != nil {
		fmt.Println("Error calling node:", address, "Details:", err)
		return
	}

	out <- response.World[1 : height+1]
}

// callDistributor sends the updated world state to the distributor.
func (b *Broker) callDistributor(updatedWorld [][]uint8) {
	client, err := rpc.Dial("tcp", "127.0.0.1:8020")
	if err != nil {
		fmt.Println("Error connecting to distributor:", err)
		return
	}
	defer client.Close()

	client.Call(stubs.HandleFlipCells, stubs.FlipRequest{OldWorld: world, NewWorld: updatedWorld, Turn: turn}, &stubs.Response{})
}

// GetCurrentState provides the current world state and count of alive cells.
func (b *Broker) GetCurrentState(request stubs.Request, response *stubs.CurrentStateResponse) error {
	mutex.Lock()
	defer mutex.Unlock()
	response.CurrentWorld = world
	response.AliveCellsCount = CountAliveCells(world)
	response.Turn = turn
	return nil
}

// HandleKey processes keyboard commands for broker control.
func (b *Broker) HandleKey(request stubs.KeyRequest, response *stubs.CurrentStateResponse) error {
	switch request.Key {
	case "q":
		*response = stubs.CurrentStateResponse{
			CurrentWorld: world,
			Turn:         turn,
		}
		responseChan := make(chan struct{})
		go func() {
			err := b.HandleBroker(stubs.Request{}, &stubs.Response{})
			if err != nil {
				fmt.Println("Error calling distributor: ", err)
			}
			responseChan <- struct{}{}
		}()
		<-responseChan
	case "k":
		b.shutdownNodes()
	case "p":
		b.togglePause()
		response.CurrentWorld = world
		response.Turn = turn
	}
	return nil
}

// shutdown sets the shutdown flag to true, triggering broker shutdown.
func (b *Broker) shutdown() {
	mutex.Lock()
	defer mutex.Unlock()
	shutdownFlag = true
}

// shutdownNodes sends shutdown requests to all nodes.
func (b *Broker) shutdownNodes() {
	for _, address := range b.nodeAddresses {
		client, err := rpc.Dial("tcp", address)
		if err != nil {
			fmt.Println("Error connecting to node:", address, "Details:", err)
			continue
		}
		done := client.Go(stubs.CloseNode, stubs.BlankRequest{}, &stubs.Response{}, nil)
		<-done.Done
		client.Close()
	}
	b.shutdown()
}

// togglePause toggles the pause state.
func (b *Broker) togglePause() {
	pauseFlag = !pauseFlag
}

// waitIfPaused waits if the broker is in paused state.
func (b *Broker) waitIfPaused() {
	for pauseFlag {
		time.Sleep(100 * time.Millisecond)
	}
}
