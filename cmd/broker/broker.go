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

// Broker 结构体，负责管理节点列表和控制游戏的暂停、退出和关闭等标志。
type Broker struct {
	nodeAddresses []string // 节点地址列表
	pauseFlag     bool     // 暂停标志
	quiteFlag     bool     // 静默模式标志（未完全实现）
	shutdownFlag  bool     // 关闭标志
}

// 全局变量，存储世界状态、互斥锁以及游戏回合数。
var world [][]uint8  // 世界状态，表示生命游戏的当前状态
var mutex sync.Mutex // 互斥锁，确保并发访问共享资源时的安全
var totalTurns int   // 总回合数
var turn int = 0     // 当前回合数

// main 函数，启动 Broker，监听传入的 RPC 请求，并处理关闭逻辑。
// •	初始化：Broker 会监听 127.0.0.1:8083 端口，并注册 RPC 服务。
// •	节点列表：通过 NewBroker 初始化节点地址列表。
// •	RPC 注册：将 Broker 注册为 RPC 服务对象。
// •	监听端口：使用 net.Listen 启动 TCP 监听。
// •	Broker 关闭监控：shutdownWatcher 协程会在 shutdownFlag 置为 true 时关闭 Broker。
func main() {
	pAddr := flag.String("port", "127.0.0.1:8083", "Port to listen on") // 设置监听的端口，默认为 127.0.0.1:8083
	flag.Parse()
	rand.Seed(time.Now().UnixNano()) // 用当前时间戳初始化随机数生成器

	// 初始化 Broker，传入节点地址列表
	broker := NewBroker([]string{
		"8.130.81.92:8085", // 示例节点地址
	})

	// 注册 Broker 为 RPC 服务对象
	err := rpc.Register(broker)
	if err != nil {
		fmt.Println("注册 Broker 出错:", err)
		return
	}

	// 开始监听端口，等待传入的 RPC 请求
	listener, err := net.Listen("tcp", *pAddr)
	if err != nil {
		fmt.Println("启动监听出错:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Broker 在端口", *pAddr, "运行")

	// 启动一个 goroutine 来监听 shutdown 标志，判断是否需要关闭 Broker
	go broker.shutdownWatcher()

	// 接受 RPC 请求
	rpc.Accept(listener)
}

// NewBroker 初始化并返回一个新的 Broker 实例，接收节点地址列表作为参数。
func NewBroker(nodeAddresses []string) *Broker {
	return &Broker{
		nodeAddresses: nodeAddresses,
		pauseFlag:     false,
		quiteFlag:     false,
		shutdownFlag:  false,
	}
}

// shutdownWatcher 持续检查 shutdownFlag 标志，若为 true，则退出程序并关闭 Broker。
func (b *Broker) shutdownWatcher() {
	for {
		time.Sleep(100 * time.Millisecond) // 每 100 毫秒检查一次
		mutex.Lock()                       // 锁定互斥锁，安全访问共享变量
		if b.shutdownFlag {
			mutex.Unlock()
			fmt.Println("Broker 正在关闭...")
			os.Exit(0) // 如果标志为 true，则退出程序
		}
		mutex.Unlock()
	}
}

// HandleBroker 是主 RPC 方法，用于处理世界状态更新和任务分发给节点。
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
		world = request.World // 从请求中初始化世界状态
	}()

	// 初始化世界状态和总回合数
	world = request.World
	totalTurns = request.Params.Turns

	// 将世界状态分割成多个部分，每个节点处理一个部分
	numNodes := len(b.nodeAddresses)
	workerHeight := len(world) / numNodes
	remaining := len(world) % numNodes

	// 创建通道用于与各个节点的通信
	channels := make([]chan [][]uint8, numNodes)
	for turn = 0; turn < totalTurns; {

		// 将工作分发给各个节点进行计算
		updatedWorld := b.distributeWork(numNodes, workerHeight, remaining, request, channels)

		// 更新世界状态，并通知 distributor
		mutex.Lock()
		b.callDistributor(updatedWorld) // 发送更新后的世界状态给 distributor
		world = updatedWorld
		turn++
		mutex.Unlock()

		// 如果暂停标志为 true，暂停游戏
		for b.pauseFlag {
			time.Sleep(100 * time.Millisecond) // 暂停时，每 100 毫秒检查一次
		}
	}

	// 返回游戏结束后的最终世界状态
	response.Turn = turn
	response.Status = "OK"
	response.World = world
	return nil
}

// distributeWork 将世界状态分成多个部分，分发给不同的节点进行处理。
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
	updatedWorld := make([][]uint8, 0) // 存储更新后的世界状态
	for i := 0; i < numNodes; i++ {
		channels[i] = make(chan [][]uint8) // 为每个节点创建一个通道
		startY := i * workerHeight
		endY := ((i + 1) * workerHeight) + remaining
		nodeWorld := GetImagePart(request.Params, startY, endY, world) // 获取每个节点需要处理的世界部分

		// 将任务分发给工作节点
		go b.callNode(b.nodeAddresses[i], endY-startY, nodeWorld, channels[i])
	}

	// 收集所有节点的结果，并将其合并成完整的世界状态
	for i := 0; i < numNodes; i++ {
		receivedData := <-channels[i]                        // 从每个节点接收处理结果
		updatedWorld = append(updatedWorld, receivedData...) // 合并结果
	}
	return updatedWorld
}

// callNode 向工作节点发送请求，并收集其返回的更新后的世界状态。
// callNode handles the RPC call to a node and collects the result.
// •	b *Broker：方法属于 Broker 结构体。
// •	address string：worker 节点的 IP 地址和端口。
// •	height int：该 worker 节点需要处理的世界的高度（行数）。
// •	nodeWorld [][]uint8：发送给 worker 节点的部分世界数据（二维数组）。
// •	out chan [][]uint8：用于发送处理后的结果的通道。
func (b *Broker) callNode(address string, height int, nodeWorld [][]uint8, out chan [][]uint8) {
	client, err := rpc.Dial("tcp", address) // 与工作节点建立 RPC 连接
	if err != nil {
		fmt.Println("连接工作节点出错:", address, "详细信息:", err)
		return
	}
	defer client.Close()

	request := stubs.Request{World: nodeWorld}               // 创建请求，包含该节点需要处理的世界部分
	response := new(stubs.Response)                          // 响应结构体
	err = client.Call(stubs.HandleWorker, request, response) // 调用节点的 RPC 方法进行计算
	if err != nil {
		fmt.Println("调用工作节点出错:", address, "详细信息:", err)
		return
	}

	// 将处理后的世界部分发送回 Broker
	out <- response.World[1 : height+1]
}

// callDistributor 向 distributor 发送更新后的世界状态，用于刷新显示。
// callDistributor sends the updated world state to the distributor.
//  向controller节点发送请求刷新当前世界状态
// •	b *Broker：方法属于 Broker 类型。
// •	updatedWorld [][]uint8：更新后的 Game of Life 世界的二维数组（即包含所有细胞的状态）。
func (b *Broker) callDistributor(updatedWorld [][]uint8) {
	client, err := rpc.Dial("tcp", "127.0.0.1:8082") // 与 distributor 建立 RPC 连接
	if err != nil {
		fmt.Println("连接到 distributor 出错:", err)
		return
	}
	defer client.Close()

	// 向 distributor 发送更新后的世界状态
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

// GetCurrentState 获取当前世界状态和存活细胞数量。
func (b *Broker) GetCurrentState(request stubs.Request, response *stubs.CurrentStateResponse) error {
	mutex.Lock()
	defer mutex.Unlock()
	response.CurrentWorld = world
	response.AliveCellsCount = CountAliveCells(world) // 计算存活的细胞数量
	response.Turn = turn
	return nil
}

// HandleKey 处理用户输入的控制命令（例如暂停、退出、关闭节点）。
func (b *Broker) HandleKey(request stubs.KeyRequest, response *stubs.CurrentStateResponse) error {
	switch request.Key {
	case "q": // 退出游戏
		mutex.Lock()
		defer mutex.Unlock()
		responseChan := make(chan struct{})
		go func() {
			err := b.HandleBroker(stubs.Request{}, &stubs.Response{})
			if err != nil {
				fmt.Println("调用 distributor 出错: ", err)
			}
			responseChan <- struct{}{}
		}()
		<-responseChan
		*response = stubs.CurrentStateResponse{
			CurrentWorld: world,
			Turn:         turn,
		}
	case "k": // 关闭所有节点
		b.shutdownNodes()
	case "p": // 暂停或恢复游戏
		b.togglePause()
		response.CurrentWorld = world
		response.Turn = turn
	}
	return nil
}

// shutdown 设置 shutdownFlag 为 true，触发 Broker 关闭。
func (b *Broker) shutdown() {
	mutex.Lock()
	defer mutex.Unlock()
	b.shutdownFlag = true
}

// shutdownNodes 向所有节点发送关闭请求。
func (b *Broker) shutdownNodes() {
	for _, address := range b.nodeAddresses {
		client, err := rpc.Dial("tcp", address)
		if err != nil {
			fmt.Println("连接到节点出错:", address, "详细信息:", err)
			continue
		}
		done := client.Go(stubs.CloseNode, stubs.BlankRequest{}, &stubs.Response{}, nil)
		<-done.Done
		client.Close()
	}
	b.shutdown()
}

// togglePause 切换暂停标志，控制游戏暂停或恢复。
func (b *Broker) togglePause() {
	mutex.Lock()
	defer mutex.Unlock()
	b.pauseFlag = !b.pauseFlag
}
