package sdl

import (
	"fmt"
	"time"

	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/util"

	"github.com/veandco/go-sdl2/sdl"
)

// 定义常量 FPS 为 60，表示每秒钟刷新 60 次（即 60 帧每秒）。
const FPS = 60

// Run 是图形界面模式的主要函数，处理游戏的事件循环和图形更新
func Run(p gol.Params, events <-chan gol.Event, keyPresses chan<- rune) {
	// 创建一个新的 SDL 窗口，大小为 p.ImageWidth × p.ImageHeight
	// 通过 NewWindow 函数返回一个窗口对象 w
	// 使用 defer 关键字确保函数结束时销毁窗口并释放资源
	w := NewWindow(int32(p.ImageWidth), int32(p.ImageHeight))
	defer w.Destroy()

	// dirty 标志，表示是否需要重新渲染帧（每次游戏状态变化时都会设置为 true）
	dirty := false
	// refreshTicker 定时器，用来控制每秒钟刷新 60 次（FPS）
	// 每秒会触发一次事件，用于刷新图形界面
	refreshTicker := time.NewTicker(time.Second / time.Duration(FPS))
	// avgTurns 用于计算每秒钟完成的游戏轮数（平均每秒的游戏轮数）
	avgTurns := util.NewAvgTurns()

	// sdl 标签用于 break 跳出外层循环
	// 使用 select 监听多个通道的输入：事件通道和定时器事件
sdl:
	for {
		// 每当 refreshTicker 定时器触发时，处理图形刷新逻辑
		select {
		case <-refreshTicker.C:
			// 获取当前的事件
			event := w.PollEvent()
			if event != nil {
				// 处理 SDL 事件
				switch e := event.(type) {
				// 如果是退出事件（例如关闭窗口），发送 'q' 到 keyPresses 通道
				case *sdl.QuitEvent:
					keyPresses <- 'q'
				// 如果是键盘事件，检查按键并将对应字符发送到 keyPresses 通道
				case *sdl.KeyboardEvent:
					switch e.Keysym.Sym {
					case sdl.K_ESCAPE: // 按下 ESC 键
						keyPresses <- 'q'
					case sdl.K_p: // 按下 P 键
						keyPresses <- 'p'
					case sdl.K_s: // 按下 S 键
						keyPresses <- 's'
					case sdl.K_q: // 按下 Q 键
						keyPresses <- 'q'
					case sdl.K_k: // 按下 K 键
						keyPresses <- 'k'
					}
				}
			}
			// 如果需要刷新图像（dirty 为 true），则调用 RenderFrame 函数渲染新的一帧
			if dirty {
				w.RenderFrame()
				// 渲染完成后，重置 dirty 为 false
				dirty = false
			}

		// 处理从事件通道 events 收到的事件
		case event, ok := <-events:
			if !ok {
				// 如果事件通道关闭，跳出循环
				break sdl
			}
			// 根据不同的事件类型进行处理
			switch e := event.(type) {
			case gol.CellFlipped:
				// 如果是 CellFlipped 事件，翻转对应单元格的像素
				w.FlipPixel(e.Cell.X, e.Cell.Y)
			case gol.CellsFlipped:
				// 如果是 CellsFlipped 事件，遍历所有单元格并翻转对应像素
				for _, cell := range e.Cells {
					w.FlipPixel(cell.X, cell.Y)
				}
			case gol.TurnComplete:
				// 如果是 TurnComplete 事件，标记 dirty 为 true，表示需要重新渲染
				dirty = true
			case gol.AliveCellsCount:
				// 如果是 AliveCellsCount 事件，输出当前轮数和活细胞数
				fmt.Printf("Completed Turns %-8v %-20v Avg%+5v turns/sec\n", event.GetCompletedTurns(), event, avgTurns.Get(event.GetCompletedTurns()))
			case gol.FinalTurnComplete:
				// 如果是 FinalTurnComplete 事件，输出游戏结束的轮数信息
				fmt.Printf("Completed Turns %-8v %v\n", event.GetCompletedTurns(), event)
			case gol.ImageOutputComplete:
				// 如果是 ImageOutputComplete 事件，输出图像生成的相关信息
				fmt.Printf("Completed Turns %-8v %v\n", event.GetCompletedTurns(), event)
			case gol.StateChange:
				// 如果是 StateChange 事件，输出当前的状态信息
				fmt.Printf("Completed Turns %-8v %v\n", event.GetCompletedTurns(), event)
				// 如果新的状态是 Quitting，则跳出循环，结束程序
				if e.NewState == gol.Quitting {
					break sdl
				}
			}
		}
	}
}

// RunHeadless 是无图形界面的版本，处理游戏逻辑和事件输出
func RunHeadless(events <-chan gol.Event) {
	// avgTurns 用于计算每秒完成的游戏轮数
	avgTurns := util.NewAvgTurns()

	// 遍历事件通道，处理各种事件
	for event := range events {
		// 根据不同的事件类型进行处理
		switch e := event.(type) {
		case gol.AliveCellsCount:
			// 如果是 AliveCellsCount 事件，输出当前轮数和活细胞数
			fmt.Printf("Completed Turns %-8v %-20v Avg%+5v turns/sec\n", event.GetCompletedTurns(), event, avgTurns.Get(event.GetCompletedTurns()))
		case gol.FinalTurnComplete:
			// 如果是 FinalTurnComplete 事件，输出游戏结束的轮数信息
			fmt.Printf("Completed Turns %-8v %v\n", event.GetCompletedTurns(), "Final Turn Complete")
		case gol.ImageOutputComplete:
			// 如果是 ImageOutputComplete 事件，输出图像生成的相关信息
			fmt.Printf("Completed Turns %-8v %v\n", event.GetCompletedTurns(), event)
		case gol.StateChange:
			// 如果是 StateChange 事件，输出当前的状态信息
			fmt.Printf("Completed Turns %-8v %v\n", event.GetCompletedTurns(), event)
			// 如果新的状态是 Quitting，则结束程序
			if e.NewState == gol.Quitting {
				break
			}
		}
	}
}
