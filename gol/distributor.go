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

// distributorChannels 结构体定义了与分发器相关的通道，用于与其他模块之间的通信。
// •	events：用于发送游戏事件的通道。
// •	ioCommand：用于发送I/O命令的通道。
// •	ioIdle：用于接收I/O闲置状态的通道。
// •	ioFilename：用于发送文件名的通道。
// •	ioOutput：用于发送I/O输出数据的通道。
// •	ioInput：用于接收I/O输入数据的通道。
// •	ioKeyPress：用于接收按键事件的通道。
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
	distributorRegistered bool                // 用于标记分发器是否已注册
	channels              distributorChannels // 存储分发器通道
	pauseFlag             bool                // 用于标记游戏是否暂停
)

type Distributor struct{}

// startGame 函数用于初始化游戏世界并启动游戏控制器。
// •	流程：
// 1.	初始化游戏世界 initialWorld。
// 2.	调用 gameOfLifeController 控制游戏逻辑。
// 3.	游戏结束后，保存最终状态并通知 ioCommand 通道结束 I/O 操作。
func startGame(p Params, c distributorChannels) {
	// 初始化游戏世界，读取初始世界图像
	worldSlice := createWorld(p.ImageHeight, p.ImageWidth)
	initialWorld := getImage(p, c, worldSlice)

	// 发送细胞翻转事件，刷新图像显示
	c.events <- CellsFlipped{CompletedTurns: 0, Cells: getAliveCells(initialWorld, p.ImageWidth, p.ImageHeight)}
	// 发送游戏状态改变事件，状态改为开始
	c.events <- StateChange{0, Executing}

	// 处理游戏的计算逻辑
	finalWorld, turn := gameOfLifeController(p, c, initialWorld)

	// 游戏结束，发送最终的轮次事件，统计存活细胞，写输出文件
	// 如果是正常结束轮次，保存游戏状态
	if turn == p.Turns {
		aliveCells := getAliveCells(finalWorld, p.ImageWidth, p.ImageHeight)
		c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: aliveCells}
		writeImage(p, c, p.Turns, finalWorld)

		// 确保I/O操作完成后退出
		c.ioCommand <- ioCheckIdle
		<-c.ioIdle

		c.events <- StateChange{p.Turns, Quitting}
		close(c.events)
	}
}

// gameOfLifeController 函数用于控制游戏生命周期，处理远程节点的计算结果、用户输入、暂停/恢复等操作。
// •	流程：
// 1.	创建 RPC 客户端连接。
// 2.	定时获取活细胞数量。
// 3.	响应用户按键操作（保存、暂停、退出等）。
// 4.	游戏完成或用户退出时停止计时器并返回当前世界状态。
func gameOfLifeController(p Params, c distributorChannels, initialWorld [][]uint8) ([][]uint8, int) {
	defer func() {
		// 退出时，将暂停标志置为 false
		pauseFlag = false
	}()

	// 启动一个2秒钟的定时器
	ticker := time.NewTicker(2 * time.Second)

	// 与计算节点建立RPC连接
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

	// 向计算节点发送请求，获取下一个状态
	response := new(stubs.Response)
	done := client.Go(stubs.BrokerHandler, request, response, nil)

	// 游戏主循环
	for {
		select {
		case <-done.Done:
			// 收到计算节点的返回结果，游戏执行完成，停止定时器，返回最终状态
			ticker.Stop()
			return response.World, response.Turn
		case <-ticker.C:
			// 每2秒请求一次活细胞数量
			sendAliveCellsCount(client, c)
		case key := <-c.ioKeyPress:
			// 处理按键事件
			res := handleKeyPress(p, c, client, key)
			if key == 'q' || key == 'k' {
				// 按 'q' 或 'k' 退出游戏
				ticker.Stop()
				return res.CurrentWorld, res.Turn
			}
		}
	}
}

// sendAliveCellsCount 函数请求远程计算节点获取当前的活细胞数量，并将其发送到事件通道。
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

// handleKeyPress 函数处理用户输入的按键事件。
// •	's'：保存当前世界状态。
// •	'q' 或 'k'：退出或关闭整个系统。
// •	'p'：暂停/恢复游戏。
func handleKeyPress(p Params, c distributorChannels, client *rpc.Client, key rune) *stubs.CurrentStateResponse {
	switch key {
	case 's':
		// 按 's' 保存当前世界状态
		saveCurrentState(client, p, c)
	case 'q', 'k':
		// 按 'q' 或 'k' 退出游戏
		return quitOrShutdownGame(p, c, client, key)
	case 'p':
		// 按 'p' 暂停/恢复游戏
		pauseGame(p, c, client)
	default:
		fmt.Println("无效的按键")
	}

	return nil
}

// saveCurrentState 函数保存当前世界状态。
// •	通过 RPC 获取当前活细胞的数量，并将该信息传递给事件通道。
func saveCurrentState(client *rpc.Client, p Params, c distributorChannels) {
	// 创建一个空请求对象
	request := stubs.BlankRequest{}
	response := new(stubs.CurrentStateResponse)
	// 发起 RPC 调用，获取当前游戏状态
	err := client.Call(stubs.GetCurrentState, request, response)
	if err != nil {
		fmt.Printf("Error GetCurrentState -> %s\n", err.Error())
		os.Exit(1)
	}
	fmt.Println("保存当前状态")
	if response.Turn == 0 {
		writeImage(p, c, 0, createWorld(p.ImageHeight, p.ImageWidth))
	} else {
		writeImage(p, c, response.Turn, response.CurrentWorld)
	}
}

// quitOrShutdownGame 函数处理退出或关闭游戏的操作。
// •	根据按键 ('q' 或 'k') 发送退出命令并保存当前世界状态。
func quitOrShutdownGame(p Params, c distributorChannels, client *rpc.Client, key rune) *stubs.CurrentStateResponse {
	keyRequest := stubs.KeyRequest{Key: "q"}
	keyResponse := new(stubs.CurrentStateResponse)

	// 发起 RPC 调用，退出游戏
	err := client.Call(stubs.HandleKey, keyRequest, keyResponse)
	if err != nil {
		fmt.Printf("Error HandleKey -> %s\n", err.Error())
		os.Exit(1)
	}
	fmt.Println("退出游戏并保存状态")
	if keyResponse.Turn == 0 {
		writeImage(p, c, 0, createWorld(p.ImageHeight, p.ImageWidth))
	} else {
		writeImage(p, c, keyResponse.Turn, keyResponse.CurrentWorld)
	}

	// 通知状态更改
	c.events <- StateChange{CompletedTurns: keyResponse.Turn, NewState: Quitting}

	if key == 'k' {
		// 按 'k' 关闭所有服务
		shutdownBrokerAndNodes(client)
	}

	return keyResponse
}

// shutdownBrokerAndNodes 函数关闭 broker 和节点服务。
func shutdownBrokerAndNodes(client *rpc.Client) {
	shutDownRequest := stubs.KeyRequest{Key: "k"}
	shutDownResponse := new(stubs.CurrentStateResponse)
	done := client.Go(stubs.HandleKey, shutDownRequest, shutDownResponse, nil)
	<-done.Done
	time.Sleep(500 * time.Millisecond)
}

// pauseGame 函数处理游戏的暂停和恢复。
func pauseGame(p Params, c distributorChannels, client *rpc.Client) {
	pauseFlag = !pauseFlag
	if pauseFlag {
		// 如果游戏处于暂停状态，则通知远程节点暂停
		trun := togglePause(client)
		c.events <- StateChange{CompletedTurns: trun, NewState: Paused}
		fmt.Println("Game paused")
	} else {
		// 如果游戏恢复，则通知远程节点继续
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
// •	d *Distributor：接收者，表示该方法属于 Distributor 类型。
// •	request stubs.FlipRequest：请求参数，包含旧世界（OldWorld）、新世界（NewWorld）以及当前回合数（Turn）。
// •	response *stubs.Response：响应参数，用于传递处理结果。
// •	返回值：返回一个 error，表示方法执行过程中是否出现错误。
func (d *Distributor) HandleFlipCells(request stubs.FlipRequest, response *stubs.Response) error {
	// •	oldWorld：旧的世界状态（即上一次迭代的状态）。
	// •	newWorld：新的世界状态（即当前迭代后的状态）。
	// •	turn：当前回合数，用于标识此次迭代。
	oldWorld := request.OldWorld
	newWorld := request.NewWorld
	turn := request.Turn

	for i := range oldWorld {
		for j := range oldWorld[i] {
			// •	条件判断：检查 oldWorld[i][j] 是否与 newWorld[i][j] 不同。
			// •	如果不同，表示该细胞在新旧世界之间发生了翻转（例如，从活到死，或从死到活）。
			// •	发送事件通知：
			// •	构造一个 CellFlipped 类型的事件，包含以下信息：
			// •	CompletedTurns：当前回合数。
			// •	Cell：细胞的坐标（X: j, Y: i）。
			// •	通过 channels.events 通道发送事件通知，以便其他模块（如 UI 界面或日志记录器）可以接收到并处理该事件。
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
