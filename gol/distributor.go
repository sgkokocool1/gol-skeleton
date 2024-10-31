package gol

import (
	"fmt"
	"sync"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

// distributor divides the work between workers and interacts with other goroutines.
// func distributor(p Params, c distributorChannels) {

// 	// TODO: Create a 2D slice to store the world.

// 	turn := 0
// 	c.events <- StateChange{turn, Executing}

// 	// TODO: Execute all turns of the Game of Life.

// 	// TODO: Report the final state using FinalTurnCompleteEvent.

// 	// Make sure that the Io has finished any output before exiting.
// 	c.ioCommand <- ioCheckIdle
// 	<-c.ioIdle

// 	c.events <- StateChange{turn, Quitting}

// 	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
// 	close(c.events)
// }
// distributor divides the work between workers and interacts with other goroutines.
// distributor divides the work between workers and interacts with other goroutines.

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

func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
	// 创建一个 2D 切片来存储当前状态的世界
	world := initializeWorld(p, c)
	turn := 0
	paused := false // 表示当前是否处于暂停状态

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
				fmt.Printf("sssssssssssssssss,turn: %d\n", turn)

				writeNewWorld(world, turn, p, c)
			}
			if key == 'p' {
				fmt.Printf("pppppppppppppppp,turn: %d\n", turn)
				paused = !paused
				if paused {
					c.events <- StateChange{turn, Paused}
				} else {
					c.events <- StateChange{turn, Executing}
				}
			}
			if key == 'q' {
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
			var wg sync.WaitGroup
			newWorld := make([][]uint8, p.ImageHeight)
			for i := range newWorld {
				newWorld[i] = make([]uint8, p.ImageWidth)
			}

			// 分块并发处理
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
			aliveCells := countAliveCells(newWorld, p.ImageWidth, p.ImageHeight)
			c.events <- AliveCellsCount{CompletedTurns: turn, CellsCount: aliveCells}
			c.events <- TurnComplete{turn}
			world = newWorld
		}
	}

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
