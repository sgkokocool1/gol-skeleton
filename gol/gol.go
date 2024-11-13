package gol

// Params provides the details of how to run the Game of Life and which image to load.
// Params 结构体定义了游戏的运行参数，包括：
// Turns：游戏要执行的回合数。
// Threads：并发线程数。
// ImageWidth：图像的宽度（列数）。
// ImageHeight：图像的高度（行数）。
type Params struct {
	Turns       int // 游戏要执行的回合数
	Threads     int // 并发线程数
	ImageWidth  int // 图像的宽度（列数）
	ImageHeight int // 图像的高度（行数）
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
// Run() 函数是生命游戏的核心启动函数，用于初始化通道和启动各个 goroutine。
// 接收三个参数：
// p：游戏的参数（类型为 Params）
// events：事件通道，用于发送各种事件（例如细胞翻转、游戏结束等）
// keyPresses：键盘输入通道，用于接收用户按键（例如暂停、退出、保存等操作）
func Run(p Params, events chan<- Event, keyPresses <-chan rune) {

	// TODO: Put the missing channels in here.
	// 创建缺失的通道
	// ioFilename：用于发送和接收要读取或保存的文件名
	// ioOutput：用于将图像数据发送到 I/O 模块以进行保存
	// ioInput：用于从 I/O 模块接收图像数据
	// ioCommand：用于发送 I/O 操作命令（如保存、加载图像）
	// ioIdle：用于检测 I/O 模块是否空闲
	ioFilename := make(chan string)                          // 创建文件名通道，用于发送文件名
	ioOutput := make(chan uint8, p.ImageHeight*p.ImageWidth) // 创建输出通道，缓冲区大小为图像像素数
	ioInput := make(chan uint8, p.ImageHeight*p.ImageWidth)  // 创建输入通道，缓冲区大小为图像像素数
	ioCommand := make(chan ioCommand)                        // 创建 I/O 命令通道
	ioIdle := make(chan bool)                                // 创建 I/O 空闲状态通道

	// ioChannels 是一个 ioChannels 类型的结构体，封装了所有与 I/O 相关的通道。
	// 通过 ioChannels 结构体，可以简化 I/O 操作的调用和管理。
	ioChannels := ioChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: ioFilename,
		output:   ioOutput,
		input:    ioInput,
	}

	// 启动一个 goroutine，执行 startIo() 函数，用于处理 I/O 操作。
	// startIo() 函数负责加载和保存图像数据，同时响应来自 ioCommand 通道的命令。
	go startIo(p, ioChannels)

	// distributorChannels 是一个 distributorChannels 类型的结构体，封装了所有用于分发器的通道：
	// events：用于发送游戏事件（如细胞状态更新、游戏完成等）
	// ioCommand、ioIdle、ioFilename、ioOutput、ioInput：与 I/O 操作相关的通道
	// ioKeyPress：用于接收用户按键输入
	distributorChannels := distributorChannels{
		events:     events,     // 游戏事件通道
		ioCommand:  ioCommand,  // I/O 命令通道
		ioIdle:     ioIdle,     // I/O 空闲状态通道
		ioFilename: ioFilename, // 文件名通道
		ioOutput:   ioOutput,   // 图像输出通道
		ioInput:    ioInput,    // 图像输入通道
		ioKeyPress: keyPresses, // 键盘输入通道
	}

	// 调用 distributor() 函数，传入 Params 和 distributorChannels，启动生命游戏的分发处理。
	// distributor() 函数负责：
	// 管理各个 worker 的并发执行。
	// 根据用户输入（通过 keyPresses）调整游戏状态（暂停、保存、退出等）。
	// 处理游戏逻辑和细胞状态更新，并将结果通过 events 通道发送。
	distributor(p, distributorChannels)
}
