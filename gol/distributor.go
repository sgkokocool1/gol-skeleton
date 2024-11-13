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
	// 初始化游戏世界，吃泡面和check/image下面读取初始文件
	worldSlice := createWorld(p.ImageHeight, p.ImageWidth)
	initialWorld := getImage(p, c, worldSlice)

	// 发送细胞翻转事件，刷新sdl页面
	c.events <- CellsFlipped{CompletedTurns: 0, Cells: getAliveCells(initialWorld, p.ImageWidth, p.ImageHeight)}
	// 发送改变游戏状态时间，状态改为开始
	c.events <- StateChange{0, Executing}

	// 处理游戏计算逻辑
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
// •	client：类型为 *rpc.Client，表示与远程 RPC 服务端的连接客户端。
// •	c：类型为 distributorChannels，封装了多个通道，用于不同模块之间的通信。
// •	这个函数的作用是通过 RPC 获取当前活细胞的数量，并将该信息传递给事件通道，以便进一步处理。
func saveCurrentState(client *rpc.Client, p Params, c distributorChannels) {
	// •	request：创建一个 BlankRequest 类型的请求对象。BlankRequest 通常是一个空的请求，用于调用不需要传递参数的 RPC 方法。
	// •	response：创建一个指向 stubs.CurrentStateResponse 的指针作为响应对象，用于存储 RPC 调用返回的结果。
	// •	CurrentStateResponse 可能包含当前回合数和活细胞数量等状态信息。
	request := stubs.BlankRequest{}
	response := new(stubs.CurrentStateResponse)
	// •	使用 client.Call() 方法进行 RPC 调用：
	// •	stubs.GetCurrentState：这是远程方法的名称，用于获取当前游戏状态（包括活细胞数量和已完成的回合数）。
	// •	request：传递的请求参数（此处为 BlankRequest，表示无需参数）。
	// •	response：用于接收远程方法的返回结果。
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
// •	p：类型为 Params，包含游戏的相关参数（如图像宽高、回合数等）。
// •	c：类型为 distributorChannels，封装了多个通道，用于模块之间的通信。
// •	client：类型为 *rpc.Client，表示与远程 RPC 服务端的连接客户端。
// •	key：类型为 rune，表示键盘输入的字符（如 q 或 k）。
// •	函数返回一个指向 stubs.CurrentStateResponse 的指针，包含当前游戏状态的信息。

func quitOrShutdownGame(p Params, c distributorChannels, client *rpc.Client, key rune) *stubs.CurrentStateResponse {
	// •	keyRequest：创建一个 KeyRequest 类型的请求对象，其中 Key 被设置为 "q"。这表示将通过 RPC 调用发送一个退出命令。
	// •	keyResponse：创建一个指向 stubs.CurrentStateResponse 的指针作为响应对象，用于存储 RPC 调用返回的结果（例如当前回合数和世界状态）。
	keyRequest := stubs.KeyRequest{Key: "q"}
	keyResponse := new(stubs.CurrentStateResponse)

	// •	使用 client.Call() 方法进行 RPC 调用：
	// •	stubs.HandleKey：远程方法的名称，用于处理键输入（例如退出）。
	// •	keyRequest：请求参数（包含按键 "q"）。
	// •	keyResponse：用于接收远程方法的返回结果。
	// •	err：如果 RPC 调用出错，则会返回一个 error 对象。
	// •	如果发生错误，打印错误信息并调用 os.Exit(1) 终止程序。
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
