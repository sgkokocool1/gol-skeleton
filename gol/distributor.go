package gol

import (
	"fmt"
	"net/rpc"
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

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {

	// TODO: Create a 2D slice to store the world.
	world := initializeWorld(p, c)
	turn := 0

	c.events <- CellsFlipped{CompletedTurns: turn, Cells: getAliveCells(world, p.ImageWidth, p.ImageHeight)}
	c.events <- StateChange{turn, Executing}

	// TODO: Execute all turns of the Game of Life.
	var wg sync.WaitGroup
	rowPerWorker := p.ImageHeight / len(p.Workers)
	for i := 0; i < len(p.Workers); i++ {
		istop, isbottom := true, true
		startRow := i * rowPerWorker
		endRow := startRow + rowPerWorker
		subGrid := world[startRow:endRow]
		workerAddr := p.Workers[i]
		fmt.Printf("start: %d; end: %d; addr: %s\n", startRow, endRow, workerAddr)
		wg.Add(1)

		go func(workerAddr string, startRow, endRow int) {
			defer wg.Done()

			client, err := rpc.Dial("tcp", workerAddr)
			if err != nil {
				panic(err)
			}
			defer client.Close()

			haloTop := world[p.ImageHeight-1]
			haloBottom := world[0]
			if startRow > 0 {
				haloTop = world[startRow-1]
				istop = false
			}
			if endRow < p.ImageHeight {
				haloBottom = world[endRow]
				isbottom = false
			}
			req := GridRequest{
				Istop:      istop,
				Isbottom:   isbottom,
				HaloTop:    haloTop,
				HaloBottom: haloBottom,
				Iterations: p.Turns,
				SubGrid:    subGrid,
			}

			var res GridResponse
			err = client.Call("GolService.ComputeGrid", req, &res)
			if err != nil {
				panic(err)
			}

			// 合并结果
			for row := range res.GridPart {
				world[startRow+row] = res.GridPart[row]
			}
		}(workerAddr, startRow, endRow)
	}
	wg.Wait()

	for _, row := range world {
		fmt.Println(row)
	}

	// TODO: Report the final state using FinalTurnCompleteEvent.
	writeNewWorld(world, p.Turns, p, c)
	// 获取所有活细胞的坐标并转换为 []util.Cell
	aliveCells := getAliveCells(world, p.ImageWidth, p.ImageHeight)
	// 发送最终状态报告
	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: aliveCells}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
