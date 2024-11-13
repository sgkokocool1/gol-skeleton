package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"uk.ac.bris.cs/gameoflife/gol" // 包含 Game of Life 的核心逻辑
	"uk.ac.bris.cs/gameoflife/sdl" // 用于处理 SDL 图形界面
)

// main is the function called when starting Game of Life with 'go run .'
func main() {
	runtime.LockOSThread()
	// 定义参数结构体
	var params gol.Params

	flag.IntVar(
		//	定义参数 -t，用于设置线程数（默认值为 8）。多线程用于加速模拟计算。
		&params.Threads,
		"t",
		8,
		"Specify the number of worker threads to use. Defaults to 8.")

	flag.IntVar(
		//定义参数 -w，用于设置图像宽度（默认值为 512）。
		&params.ImageWidth,
		"w",
		512,
		"Specify the width of the image. Defaults to 512.")

	flag.IntVar(
		//定义参数 -h，用于设置图像高度（默认值为 512）。
		&params.ImageHeight,
		"h",
		512,
		"Specify the height of the image. Defaults to 512.")

	flag.IntVar(
		//定义参数 -turns，用于设置要模拟的轮数（默认值为 1000）。
		&params.Turns,
		"turns",
		100000000,
		"Specify the number of turns to process. Defaults to 10000000000.")

	headless := flag.Bool(
		//定义布尔参数 -headless，用于决定是否禁用 SDL 窗口（默认为 false）。在无头模式下，模拟结果不会显示在图形界面中。
		"headless",
		false,
		"Disable the SDL window for running in a headless environment.")

	//解析所有命令行参数。
	flag.Parse()

	//打印解析到的参数值，方便用户确认参数设置是否正确。
	fmt.Printf("%-10v %v\n", "Threads", params.Threads)
	fmt.Printf("%-10v %v\n", "Width", params.ImageWidth)
	fmt.Printf("%-10v %v\n", "Height", params.ImageHeight)
	fmt.Printf("%-10v %v\n", "Turns", params.Turns)

	// •	keyPresses：用于接收键盘输入的字符（如退出程序的 q）。
	// •	events：用于传递事件，如模拟的状态更新、完成通知等。
	keyPresses := make(chan rune, 10)
	events := make(chan gol.Event, 1000)

	//	启动一个 goroutine 监听 SIGTERM 和 SIGINT 信号（如用户按下 Ctrl+C），收到信号时向 keyPresses 通道发送 'q'，以安全地退出程序。
	go sigterm(keyPresses)

	//启动一个 goroutine 执行 gol.Run() 函数，进行 Game of Life 的模拟。此函数会根据 params 的设置来运行模拟，并将事件发送到 events 通道。
	go gol.Run(params, events, keyPresses)

	// •	如果 headless 参数为 false（即非无头模式），则调用 sdl.Run() 显示图形界面。
	// •	否则，调用 sdl.RunHeadless() 以无头模式运行，通常用于服务器环境下的命令行运行。
	if !(*headless) {
		sdl.Run(params, events, keyPresses)
	} else {
		sdl.RunHeadless(events)
	}
}

// •	该函数用于捕获操作系统信号：
// •	创建一个 sigterm 通道，用于接收系统信号。
// •	使用 signal.Notify 注册 SIGTERM 和 SIGINT 信号（通常是通过 kill 命令或 Ctrl+C 触发）。
// •	当接收到信号时，将字符 'q' 发送到 keyPresses 通道，通知程序退出。
func sigterm(keyPresses chan<- rune) {
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM, syscall.SIGINT)
	<-sigterm
	keyPresses <- 'q'
}
