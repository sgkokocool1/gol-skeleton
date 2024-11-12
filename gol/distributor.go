package gol

import (
	"fmt"
	"sync"

	"uk.ac.bris.cs/gameoflife/util"
)

// 1.	初始化游戏世界。
// 2.	多线程并行计算世界的下一个状态。
// 3.	处理用户输入控制（暂停、保存、退出）。
// 4.	输出游戏状态到文件。

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

//从文件读取初始世界状态。
// •	根据图像的宽高生成文件名。
// •	通过 ioCommand 和 ioFilename 请求 I/O goroutine 从文件读取数据。
// •	从 ioInput 通道逐行接收数据，填充到二维切片 world 中。
// •	返回初始化后的世界数据。
func initializeWorld(p Params, c distributorChannels) [][]uint8 {
	filename := fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight)
	c.ioCommand <- ioInput
	c.ioFilename <- filename

	world := make([][]uint8, p.ImageHeight)
	for i := range world {
		world[i] = make([]uint8, p.ImageWidth)
		for j := range world[i] {
			world[i][j] = <-c.ioInput
		}
	}

	fmt.Println("initializeworld successful")
	return world
}

//并行计算局部区域的细胞状态更新。
// •	并行处理某个区块（startY 到 endY）的细胞状态。
// •	计算每个细胞周围的活邻居数。
// •	根据规则决定细胞的生死变化，并向 events 通道报告细胞状态变化。
func computeSection(startY, endY int, world [][]uint8, newWorld [][]uint8, width, height int, wg *sync.WaitGroup, c distributorChannels, turn int) {
	defer wg.Done()
	for y := startY; y < endY; y++ {
		for x := 0; x < width; x++ {
			aliveNeighbors := countAliveNeighbors(world, x, y, width, height)
			if world[y][x] == 255 {
				if aliveNeighbors < 2 || aliveNeighbors > 3 {
					newWorld[y][x] = 0 // 死亡
					c.events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: x, Y: y}}
				} else {
					newWorld[y][x] = 255 // 保持活
				}
			} else if aliveNeighbors == 3 {
				newWorld[y][x] = 255 // 复活
				c.events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: x, Y: y}}
			} else {
				newWorld[y][x] = 0 // 保持死
			}
		}
	}
}

//将当前世界状态保存到文件。
func writeNewWorld(world [][]uint8, turn int, p Params, c distributorChannels) {
	filename := fmt.Sprintf("%dx%dx%d", p.ImageWidth, p.ImageHeight, turn)
	c.ioCommand <- ioOutput
	c.ioFilename <- filename

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}

	c.ioCommand <- ioCheckIdle
	<-c.ioIdle // 等待接收 ioIdle 信号，确认 IO 空闲

	c.events <- ImageOutputComplete{CompletedTurns: turn, Filename: filename}
}

//核心调度器，控制游戏逻辑循环、线程调度和 I/O 操作。
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
	// 创建一个 2D 切片来存储当前状态的世界
	world := initializeWorld(p, c)
	turn := 0
	paused := false // 表示当前是否处于暂停状态

	//初始化游戏最开始的状态，发送刷新sdl页面事件，是得窗口显示存活和死亡细胞
	c.events <- CellsFlipped{CompletedTurns: turn, Cells: getAliveCells(world, p.ImageWidth, p.ImageHeight)}
	c.events <- StateChange{turn, Executing}
	// writeNewWorld(world, turn, p, c)

	// 执行所有的生命游戏的回合数
turnLoop:
	for turn < p.Turns {
		// 检查按键事件
		select {
		case key := <-keyPresses:
			if key == 's' {
				//按下s，收到s请求，保存当前的状态
				fmt.Printf("sssssssssssssssss,turn: %d\n", turn)
				writeNewWorld(world, turn, p, c)
			}
			if key == 'p' {
				//按下p，如果当前状态是正常状态则暂停，如果当前状态为暂停则恢复正常状态
				// 暂停或者恢复发送改变状态事件
				fmt.Printf("pppppppppppppppp,turn: %d\n", turn)
				paused = !paused
				if paused {
					c.events <- StateChange{turn, Paused}
				} else {
					c.events <- StateChange{turn, Executing}
				}
			}
			if key == 'q' {
				// 按下q，则退出循环，退出游戏
				fmt.Printf("qqqqqqqqqqqqqq,turn: %d\n", turn)
				break turnLoop
			}
		default:
			// 没有按键按下时继续游戏逻辑
			if paused {
				continue // 继续等待，直到不再暂停
			}

			turn++
			// 执行多线程并行计算
			// 使用 sync.WaitGroup 和 goroutines 并行处理不同的世界区域。
			var wg sync.WaitGroup
			newWorld := make([][]uint8, p.ImageHeight)
			for i := range newWorld {
				newWorld[i] = make([]uint8, p.ImageWidth)
			}

			// 分块并发处理
			// 每个线程处理一块高度
			rowsPerThread := p.ImageHeight / p.Threads
			for t := 0; t < p.Threads; t++ {
				startY := t * rowsPerThread
				endY := (t + 1) * rowsPerThread
				if t == p.Threads-1 {
					endY = p.ImageHeight
				}
				wg.Add(1)
				go computeSection(startY, endY, world, newWorld, p.ImageWidth, p.ImageHeight, &wg, c, turn)
			}

			// 等待所有线程完成
			wg.Wait()

			// newWorld := computeNewWorld(world, turn, p, c)
			// writeNewWorld(newWorld, turn, p, c)
			// 每个回合结束后发送 `AliveCellsCount` 事件
			// 发送回合结束事件 TurnComplete 和活细胞计数事件 AliveCellsCount。
			aliveCells := countAliveCells(newWorld, p.ImageWidth, p.ImageHeight)
			c.events <- AliveCellsCount{CompletedTurns: turn, CellsCount: aliveCells}
			c.events <- TurnComplete{turn}
			world = newWorld
		}
	}

	// 所有回合结束，写入最终状态
	writeNewWorld(world, turn, p, c)
	// 获取所有活细胞的坐标并转换为 []util.Cell
	aliveCells := getAliveCells(world, p.ImageWidth, p.ImageHeight)
	// 发送最终状态报告
	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: aliveCells}

	// 确保 IO 完成任何输出操作
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	// 通知状态变更为退出
	c.events <- StateChange{turn, Quitting}

	// 关闭事件通道以停止 SDL 协程
	close(c.events)
}

// getAliveCells 提取世界中活细胞的坐标，返回一个 []util.Cell 切片
func getAliveCells(world [][]uint8, width, height int) []util.Cell {
	var cells []util.Cell
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if world[y][x] == 255 {
				cells = append(cells, util.Cell{X: x, Y: y})
			}
		}
	}
	return cells
}

// countAliveNeighbors 计算指定单元格周围的活邻居数量
func countAliveNeighbors(world [][]uint8, x, y, width, height int) int {
	alive := 0
	// 所有neighbor数组，依次是
	// 左上，上，右上
	// 左， 右
	// 左下，下，右下
	neighbors := [][2]int{
		{-1, -1}, {-1, 0}, {-1, 1},
		{0, -1}, {0, 1},
		{1, -1}, {1, 0}, {1, 1},
	}

	for _, n := range neighbors {
		nx, ny := x+n[1], y+n[0]
		if nx < 0 {
			nx = width - 1
		} else if nx >= width {
			nx = 0
		}
		if ny < 0 {
			ny = height - 1
		} else if ny >= height {
			ny = 0
		}

		if world[ny][nx] == 255 {
			alive++
		}
	}
	return alive
}

// countAliveCells 计算当前世界中的活细胞数量
func countAliveCells(world [][]uint8, width, height int) int {
	alive := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if world[y][x] == 255 {
				alive++
			}
		}
	}
	return alive
}
