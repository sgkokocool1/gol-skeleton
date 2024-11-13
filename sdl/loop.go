package sdl

import (
	"fmt"
	"time"

	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/util"

	"github.com/veandco/go-sdl2/sdl"
)

//定义常量 FPS 为 60，表示每秒钟刷新 60 次（即 60 帧每秒）。
const FPS = 60

func Run(p gol.Params, events <-chan gol.Event, keyPresses chan<- rune) {
	// •	定义 Run() 函数，接收以下参数：
	// •	p：gol.Params 类型的参数，包括图像宽度、高度等配置。
	// •	events：事件通道，用于接收生命游戏中的各种事件。
	// •	keyPresses：键盘输入通道，用于接收用户按键。
	// •	创建一个新的 SDL 窗口 w，大小为 p.ImageWidth × p.ImageHeight。
	// •	defer w.Destroy()：确保在程序退出时销毁窗口，释放资源。
	w := NewWindow(int32(p.ImageWidth), int32(p.ImageHeight))
	defer w.Destroy()

	// •	dirty：标记是否需要重新渲染帧。
	// •	refreshTicker：创建一个定时器，用于控制画面刷新频率（每秒 60 次）。
	// •	avgTurns：用于计算平均每秒完成的游戏轮数。
	dirty := false
	refreshTicker := time.NewTicker(time.Second / time.Duration(FPS))
	avgTurns := util.NewAvgTurns()

	// •	sdl: 标签用于 break 跳出循环。
	// •	使用 select 监听多个通道的输入，包括 refreshTicker 和 events。
sdl:
	for {
		select {
		case <-refreshTicker.C:
			event := w.PollEvent()
			if event != nil {
				switch e := event.(type) {
				case *sdl.QuitEvent:
					keyPresses <- 'q'
				case *sdl.KeyboardEvent:
					switch e.Keysym.Sym {
					case sdl.K_ESCAPE:
						keyPresses <- 'q'
					case sdl.K_p:
						keyPresses <- 'p'
					case sdl.K_s:
						keyPresses <- 's'
					case sdl.K_q:
						keyPresses <- 'q'
					case sdl.K_k:
						keyPresses <- 'k'
					}
				}
			}
			if dirty {
				w.RenderFrame()
				dirty = false
			}

		case event, ok := <-events:
			if !ok {
				break sdl
			}
			switch e := event.(type) {
			case gol.CellFlipped:
				w.FlipPixel(e.Cell.X, e.Cell.Y)
			case gol.CellsFlipped:
				for _, cell := range e.Cells {
					w.FlipPixel(cell.X, cell.Y)
				}
			case gol.TurnComplete:
				dirty = true
			case gol.AliveCellsCount:
				fmt.Printf("Completed Turns %-8v %-20v Avg%+5v turns/sec\n", event.GetCompletedTurns(), event, avgTurns.Get(event.GetCompletedTurns()))
			case gol.FinalTurnComplete:
				fmt.Printf("Completed Turns %-8v %v\n", event.GetCompletedTurns(), event)
			case gol.ImageOutputComplete:
				fmt.Printf("Completed Turns %-8v %v\n", event.GetCompletedTurns(), event)
			case gol.StateChange:
				fmt.Printf("Completed Turns %-8v %v\n", event.GetCompletedTurns(), event)
				if e.NewState == gol.Quitting {
					break sdl
				}
			}
		}
	}
}

func RunHeadless(events <-chan gol.Event) {
	avgTurns := util.NewAvgTurns()
	for event := range events {
		switch e := event.(type) {
		case gol.AliveCellsCount:
			fmt.Printf("Completed Turns %-8v %-20v Avg%+5v turns/sec\n", event.GetCompletedTurns(), event, avgTurns.Get(event.GetCompletedTurns()))
		case gol.FinalTurnComplete:
			fmt.Printf("Completed Turns %-8v %v\n", event.GetCompletedTurns(), "Final Turn Complete")
		case gol.ImageOutputComplete:
			fmt.Printf("Completed Turns %-8v %v\n", event.GetCompletedTurns(), event)
		case gol.StateChange:
			fmt.Printf("Completed Turns %-8v %v\n", event.GetCompletedTurns(), event)
			if e.NewState == gol.Quitting {
				break
			}
		}
	}
}
