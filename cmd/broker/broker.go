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
	pauseFlag     bool
	quiteFlag     bool
	shutdownFlag  bool
}

var world [][]uint8
var mutex sync.Mutex
var totalTurns int
var turn int = 0

// var shutdownFlag bool

// var pauseFlag bool

// •	初始化：Broker 会监听 127.0.0.1:8083 端口，并注册 RPC 服务。
// •	节点列表：通过 NewBroker 初始化节点地址列表。
// •	RPC 注册：将 Broker 注册为 RPC 服务对象。
// •	监听端口：使用 net.Listen 启动 TCP 监听。
// •	Broker 关闭监控：shutdownWatcher 协程会在 shutdownFlag 置为 true 时关闭 Broker。
func main() {
	pAddr := flag.String("port", "127.0.0.1:8083", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	broker := NewBroker([]string{
		// "34.227.14.229:8085",
		// "44.202.164.89:808",
		"8.130.81.92:8085",
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
		pauseFlag:     false,
		quiteFlag:     false,
		shutdownFlag:  false,
	}
}

func (b *Broker) shutdownWatcher() {
	for {
		time.Sleep(100 * time.Millisecond)
		mutex.Lock()
		if b.shutdownFlag {
			mutex.Unlock()
			fmt.Println("Shutting down the broker...")
			os.Exit(0)
		}
		mutex.Unlock()
	}
}

// HandleBroker distributes the world update workload among nodes and manages their responses.
// •	初始化：接收来自 Distributor 的请求，初始化世界和游戏参数。
// •	分发任务：通过 distributeWork 方法，将计算任务分发给多个节点。
// •	同步结果：收集节点的计算结果，并将更新的世界状态发送回 Distributor。
// •	暂停逻辑：如果 pauseFlag 为 true，则游戏暂停。
func (b *Broker) HandleBroker(request stubs.Request, response *stubs.Response) error {
	defer func() {
		b.quiteFlag = false
		b.pauseFlag = false
		turn = 0
		world = request.World
	}()

	// 请求初始化世界
	world = request.World
	totalTurns = request.Params.Turns

	// 根据节点数量把初始世界分成n个片段，每个worker处理一个片段
	numNodes := len(b.nodeAddresses)
	workerHeight := len(world) / numNodes
	remaining := len(world) % numNodes

	channels := make([]chan [][]uint8, numNodes)
	for turn = 0; turn < totalTurns; {

		// 每个轮次，多个节点计算每个节点的状态
		updatedWorld := b.distributeWork(numNodes, workerHeight, remaining, request, channels)

		// Update the world state and notify distributor
		mutex.Lock()
		// 想controller节点发送目前世界的状态，刷新sdl页面
		b.callDistributor(updatedWorld)
		world = updatedWorld
		turn++
		mutex.Unlock()

		// Pause if needed
		// 按下p，暂停轮次
		for b.pauseFlag {
			time.Sleep(100 * time.Millisecond)
		}
	}

	// 最后返回最终状态
	response.Turn = turn
	response.Status = "OK"
	response.World = world
	return nil
}

// distributeWork distributes the workload to worker nodes.
// 根据worker的数量，把世界分成多个状态
// •	b *Broker：这是一个方法，属于 Broker 类型的结构体。
// •	numNodes int：worker 节点的数量。
// •	workerHeight int：每个 worker 节点需要处理的世界的高度（行数）。
// •	remaining int：多出来的行数，用于将无法整除的行分配给最后一个节点。
// •	request stubs.Request：包含处理请求的参数（包括世界的状态和尺寸）。
// •	channels []chan [][]uint8：用于与各个 worker 节点通信的通道数组。
// •	返回值为 [][]uint8，表示经过所有 worker 节点处理后的完整世界状态。
func (b *Broker) distributeWork(numNodes, workerHeight, remaining int, request stubs.Request, channels []chan [][]uint8) [][]uint8 {
	updatedWorld := make([][]uint8, 0)
	for i := 0; i < numNodes; i++ {
		channels[i] = make(chan [][]uint8)
		startY := i * workerHeight
		endY := ((i + 1) * workerHeight) + remaining
		nodeWorld := GetImagePart(request.Params, startY, endY, world)

		// 调度给worker节点计算每个节点状态
		go b.callNode(b.nodeAddresses[i], endY-startY, nodeWorld, channels[i])
	}

	// Gather results from channels
	// 最后整和每个节点状态为整个世界状态
	for i := 0; i < numNodes; i++ {
		receivedData := <-channels[i]
		updatedWorld = append(updatedWorld, receivedData...)
	}
	return updatedWorld
}

// callNode handles the RPC call to a node and collects the result.
// •	b *Broker：方法属于 Broker 结构体。
// •	address string：worker 节点的 IP 地址和端口。
// •	height int：该 worker 节点需要处理的世界的高度（行数）。
// •	nodeWorld [][]uint8：发送给 worker 节点的部分世界数据（二维数组）。
// •	out chan [][]uint8：用于发送处理后的结果的通道。
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
//  向controller节点发送请求刷新当前世界状态
// •	b *Broker：方法属于 Broker 类型。
// •	updatedWorld [][]uint8：更新后的 Game of Life 世界的二维数组（即包含所有细胞的状态）。
func (b *Broker) callDistributor(updatedWorld [][]uint8) {
	// •	rpc.Dial("tcp", "127.0.0.1:8082")：
	// •	通过 TCP 协议连接到本地的 distributor 节点，地址为 127.0.0.1:8082。
	// •	distributor 通常监听在这个端口，用于接收 Broker 的 RPC 请求。
	// •	err != nil：
	// •	如果连接失败，打印错误信息（如连接被拒绝、端口不可用等），然后立即返回。
	// •	defer client.Close()：
	// •	确保在函数结束时关闭 RPC 客户端连接，无论函数是正常结束还是遇到错误。
	client, err := rpc.Dial("tcp", "127.0.0.1:8082")
	if err != nil {
		fmt.Println("Error connecting to distributor:", err)
		return
	}
	defer client.Close()

	// •	client.Call()：通过 RPC 调用 distributor 的 HandleFlipCells 方法，传递世界状态的更新信息。
	// •	stubs.HandleFlipCells：distributor 节点的 RPC 方法名称，用于处理细胞状态的翻转。
	// •	请求参数：
	// •	stubs.FlipRequest：请求结构体，包含以下字段：
	// •	OldWorld：表示旧的细胞状态世界（world）。
	// •	NewWorld：表示更新后的细胞状态世界（updatedWorld）。
	// •	Turn：当前的回合数（turn）。
	// •	响应参数：
	// •	&stubs.Response{}：空的响应结构体，因为这里不关心 distributor 返回的数据，只需要通知 distributor。
	client.Call(stubs.HandleFlipCells, stubs.FlipRequest{OldWorld: world, NewWorld: updatedWorld, Turn: turn}, &stubs.Response{})
}

// GetCurrentState provides the current world state and count of alive cells.
// •	b *Broker：方法属于 Broker 类型的接收者方法。
// •	request stubs.Request：请求参数，虽然没有在该函数内部使用，通常作为 RPC 请求的占位符。
// •	response *stubs.CurrentStateResponse：指向 stubs.CurrentStateResponse 结构体的指针，用于存储返回的响应数据。
// •	error：返回类型是 error，表示如果函数执行过程中出现错误，可以通过返回非空的 error 值来通知调用方。
func (b *Broker) GetCurrentState(request stubs.Request, response *stubs.CurrentStateResponse) error {
	mutex.Lock()
	defer mutex.Unlock()
	response.CurrentWorld = world
	response.AliveCellsCount = CountAliveCells(world)
	response.Turn = turn
	return nil
}

// HandleKey processes keyboard commands for broker control.
// •	‘q’ 键：终止当前游戏，并返回世界的最新状态。
// •	‘k’ 键：关闭所有节点。
// •	‘p’ 键：暂停或恢复游戏。
func (b *Broker) HandleKey(request stubs.KeyRequest, response *stubs.CurrentStateResponse) error {
	switch request.Key {
	case "q":
		mutex.Lock()
		defer mutex.Unlock()
		//	b.quiteFlag = true
		responseChan := make(chan struct{})
		go func() {
			err := b.HandleBroker(stubs.Request{}, &stubs.Response{})
			if err != nil {
				fmt.Println("Error calling distributor: ", err)
			}
			responseChan <- struct{}{}
		}()
		<-responseChan
		*response = stubs.CurrentStateResponse{
			CurrentWorld: world,
			Turn:         turn,
		}
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
	mutex.Lock()
	defer mutex.Unlock()
	b.pauseFlag = !b.pauseFlag
}
