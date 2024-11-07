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

	"distributed/stubs"
)

type Broker struct {
	nodeAddresses []string
	world         [][]uint8
	mutex         sync.Mutex
	totalTurns    int
	turn          int
	shutdownFlag  bool
	pauseFlag     bool
}

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
		b.mutex.Lock()
		if b.shutdownFlag {
			b.mutex.Unlock()
			fmt.Println("Shutting down the broker...")
			os.Exit(0)
		}
		b.mutex.Unlock()
	}
}

// HandleBroker distributes the world update workload among nodes and manages their responses.
func (b *Broker) HandleBroker(request stubs.Request, response *stubs.Response) error {
	b.world = request.World
	b.totalTurns = request.Params.Turns
	numNodes := len(b.nodeAddresses)
	workerHeight := len(b.world) / numNodes
	remaining := len(b.world) % numNodes

	channels := make([]chan [][]uint8, numNodes)
	for b.turn = 0; b.turn < b.totalTurns; {
		updatedWorld := b.distributeWork(numNodes, workerHeight, remaining, request, channels)

		// Update the world state and notify distributor
		b.mutex.Lock()
		b.callDistributor(updatedWorld)
		b.world = updatedWorld
		b.turn++
		b.mutex.Unlock()

		// Pause if needed
		b.waitIfPaused()
	}

	response.Status = "OK"
	response.World = b.world
	return nil
}

// distributeWork distributes the workload to worker nodes.
func (b *Broker) distributeWork(numNodes, workerHeight, remaining int, request stubs.Request, channels []chan [][]uint8) [][]uint8 {
	updatedWorld := make([][]uint8, 0)
	for i := 0; i < numNodes; i++ {
		channels[i] = make(chan [][]uint8)
		startY := i * workerHeight
		endY := ((i + 1) * workerHeight) + remaining
		nodeWorld := GetImagePart(request.Params, startY, endY, b.world)

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

	client.Call(stubs.HandleFlipCells, stubs.FlipRequest{OldWorld: b.world, NewWorld: updatedWorld, Turn: b.turn}, &stubs.Response{})
}

// GetCurrentState provides the current world state and count of alive cells.
func (b *Broker) GetCurrentState(request stubs.Request, response *stubs.CurrentStateResponse) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	response.CurrentWorld = b.world
	response.AliveCellsCount = CountAliveCells(b.world)
	response.Turn = b.turn
	return nil
}

// HandleKey processes keyboard commands for broker control.
func (b *Broker) HandleKey(request stubs.KeyRequest, response *stubs.CurrentStateResponse) error {
	switch request.Key {
	case "q":
		b.shutdown()
	case "k":
		b.shutdownNodes()
	case "p":
		b.togglePause()
		response.CurrentWorld = b.world
		response.Turn = b.turn
	}
	return nil
}

// shutdown sets the shutdown flag to true, triggering broker shutdown.
func (b *Broker) shutdown() {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.shutdownFlag = true
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
	b.pauseFlag = !b.pauseFlag
}

// waitIfPaused waits if the broker is in paused state.
func (b *Broker) waitIfPaused() {
	for b.pauseFlag {
		time.Sleep(100 * time.Millisecond)
	}
}
