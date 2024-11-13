package sdl

import (
	"fmt"
	"time"

	"github.com/veandco/go-sdl2/sdl"
	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/util"
)

const FPS = 60

// •	输入：
// •	p gol.Params：包含图像宽度、高度、回合数等参数。
// •	events <-chan gol.Event：一个通道，接收游戏事件。
// •	keyPresses chan<- rune：一个通道，用于接收按键信息。
// •	输出：
// •	无显式返回值。该函数用于管理游戏的主要循环，处理事件、更新界面、响应用户输入等。

// Run 启动并运行生命游戏的主要事件循环
func Run(p gol.Params, events <-chan gol.Event, keyPresses chan<- rune) {
	// 创建一个新的窗口，指定窗口大小为图像的宽度和高度
	w := NewWindow(int32(p.ImageWidth), int32(p.ImageHeight))
	defer w.Destroy() // 在函数结束时销毁窗口

	dirty := false                                                    // 用于指示是否需要刷新窗口
	refreshTicker := time.NewTicker(time.Second / time.Duration(FPS)) // 设置一个定时器，用于控制每秒刷新帧数
	avgTurns := util.NewAvgTurns()                                    // 用于计算每回合的平均速度

sdl: // 定义标签sdl，后续用于跳出事件循环
	for {
		// 事件选择器，用于等待事件或定时器的触发
		select {
		// 如果定时器超时，意味着需要刷新窗口
		case <-refreshTicker.C:
			event := w.PollEvent() // 获取窗口事件
			if event != nil {
				switch e := event.(type) {
				// 处理窗口退出事件
				case *sdl.QuitEvent:
					keyPresses <- 'q' // 向keyPresses通道发送退出信号
				// 处理按键事件
				case *sdl.KeyboardEvent:
					switch e.Keysym.Sym {
					case sdl.K_ESCAPE:
						keyPresses <- 'q' // 按下Esc键退出
					case sdl.K_p:
						keyPresses <- 'p' // 按下P键暂停或恢复游戏
					case sdl.K_s:
						keyPresses <- 's' // 按下S键保存当前状态
					case sdl.K_q:
						keyPresses <- 'q' // 按下Q键退出游戏
					case sdl.K_k:
						keyPresses <- 'k' // 按下K键，可能是其他特定操作
					}
				}
			}
			if dirty {
				w.RenderFrame() // 如果需要刷新，渲染当前帧
				dirty = false   // 重置dirty标志
			}

		// 处理接收到的游戏事件
		case event, ok := <-events:
			if !ok { // 如果事件通道关闭，退出事件循环
				break sdl
			}
			switch e := event.(type) {
			// 处理单个细胞状态翻转事件
			case gol.CellFlipped:
				w.FlipPixel(e.Cell.X, e.Cell.Y) // 翻转指定位置的像素
			// 处理多个细胞状态翻转事件
			case gol.CellsFlipped:
				fmt.Printf("CellsFlipped Turns %-8v %v\n", event.GetCompletedTurns(), event)
				// 遍历所有翻转的细胞，更新显示
				for _, cell := range e.Cells {
					w.FlipPixel(cell.X, cell.Y)
				}
			// 处理回合完成事件
			case gol.TurnComplete:
				dirty = true // 标记需要刷新窗口
			// 处理活细胞数量事件
			case gol.AliveCellsCount:
				fmt.Printf("Completed Turns %-8v %-20v Avg%+5v turns/sec\n", event.GetCompletedTurns(), event, avgTurns.Get(event.GetCompletedTurns()))
			// 处理最终回合完成事件
			case gol.FinalTurnComplete:
				fmt.Printf("Completed Turns %-8v %v\n", event.GetCompletedTurns(), event)
			// 处理图像输出完成事件
			case gol.ImageOutputComplete:
				fmt.Printf("Completed Turns %-8v %v\n", event.GetCompletedTurns(), event)
			// 处理状态变更事件
			case gol.StateChange:
				fmt.Printf("Completed Turns %-8v %v\n", event.GetCompletedTurns(), event)
				// 如果游戏状态为退出，则退出循环
				if e.NewState == gol.Quitting {
					break sdl
				}
			}
		}
	}
}

// •	输入：
// •	events <-chan gol.Event：这是一个接收 gol.Event 类型事件的通道，用于接收游戏中的不同事件。
// •	输出：
// •	无显式的返回值。这个函数的目的是以“无头模式”（headless mode）运行，处理接收到的事件，并将相关信息打印到控制台。
// RunHeadless 用于在无头模式下运行生命游戏，处理事件并打印相关信息
func RunHeadless(events <-chan gol.Event) {
	avgTurns := util.NewAvgTurns() // 创建一个新的AvgTurns实例，用于计算每回合的平均速度

	// 监听事件通道，处理每个接收到的事件
	for event := range events {
		switch e := event.(type) {
		// 处理活细胞计数事件，打印每回合的统计信息
		case gol.AliveCellsCount:
			fmt.Printf("Completed Turns %-8v %-20v Avg%+5v turns/sec\n", event.GetCompletedTurns(), event, avgTurns.Get(event.GetCompletedTurns()))

		// 处理最终回合完成事件，打印最终回合的完成信息
		case gol.FinalTurnComplete:
			fmt.Printf("Completed Turns %-8v %v\n", event.GetCompletedTurns(), "Final Turn Complete")

		// 处理图像输出完成事件，打印输出完成的信息
		case gol.ImageOutputComplete:
			fmt.Printf("Completed Turns %-8v %v\n", event.GetCompletedTurns(), event)

		// 处理状态变更事件，打印状态变更信息
		case gol.StateChange:
			fmt.Printf("Completed Turns %-8v %v\n", event.GetCompletedTurns(), event)
			if e.NewState == gol.Quitting { // 如果新的状态是退出，跳出循环
				break
			}
		}
	}
}
