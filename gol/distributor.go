package gol

import "uk.ac.bris.cs/gameoflife/util"

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
func distributor(p Params, c distributorChannels) {
	// 创建一个 2D 切片来存储当前状态的世界
	world := make([][]uint8, p.ImageHeight)
	for i := range world {
		world[i] = make([]uint8, p.ImageWidth)
	}

	turn := 0
	c.events <- StateChange{turn, Executing}

	// 执行所有的生命游戏的回合数
	for turn < p.Turns {
		newWorld := make([][]uint8, p.ImageHeight)
		for i := range newWorld {
			newWorld[i] = make([]uint8, p.ImageWidth)
		}

		// 更新当前回合的状态
		for y := 0; y < p.ImageHeight; y++ {
			for x := 0; x < p.ImageWidth; x++ {
				aliveNeighbors := countAliveNeighbors(world, x, y, p.ImageWidth, p.ImageHeight)
				if world[y][x] == 255 {
					// 活细胞规则
					if aliveNeighbors < 2 || aliveNeighbors > 3 {
						newWorld[y][x] = 0 // 死亡
					} else {
						newWorld[y][x] = 255 // 保持活着
					}
				} else {
					// 死细胞规则
					if aliveNeighbors == 3 {
						newWorld[y][x] = 255 // 复活
					} else {
						newWorld[y][x] = 0
					}
				}
			}
		}

		world = newWorld
		turn++
		c.events <- TurnComplete{turn}

		// 每个回合结束后发送 `AliveCellsCount` 事件
		aliveCells := countAliveCells(world, p.ImageWidth, p.ImageHeight)
		c.events <- AliveCellsCount{CompletedTurns: turn, CellsCount: aliveCells}
	}

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
