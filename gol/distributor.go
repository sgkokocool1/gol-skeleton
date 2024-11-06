package gol

import (
	"fmt"
	"math"
	"net/rpc"
	"os"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
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
	paused := false // 表示当前是否处于暂停状态
	c.events <- CellsFlipped{CompletedTurns: turn, Cells: getAliveCells(world, p.ImageWidth, p.ImageHeight)}
	c.events <- StateChange{turn, Executing}

	// TODO: Execute all turns of the Game of Life.
	var wg sync.WaitGroup
	rowPerWorker := p.ImageHeight / len(p.Workers)
	for i := 0; i < len(p.Workers); i++ {
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
			}
			if endRow < p.ImageHeight {
				haloBottom = world[endRow]
			}

			req := stubs.ComputeGridRequest{
				HaloTop:    haloTop,
				HaloBottom: haloBottom,
				Iterations: p.Turns,
				SubGrid:    subGrid,
				StartRow:   startRow,
				EndRow:     endRow,
			}
			response := new(stubs.ComputeGridResponse)
			client.Call(stubs.GoLWorkerComputeGridHandler, req, response)

			// 合并结果
			for row := range response.GridPart {
				world[startRow+row] = response.GridPart[row]
			}
		}(workerAddr, startRow, endRow)
	}

	done := make(chan struct{}) // 用于通知 goroutine 退出
	go func() {
		//time.Sleep(2 * time.Second)
		loopTimer := time.NewTicker(time.Second * 2)
		defer loopTimer.Stop()

		for {
			select {
			case <-done: // 当收到 done 信号时退出循环
				fmt.Println("Received exit signal, exiting loop...")
				return
			case <-loopTimer.C:
				alives, aturn := getAllAliveCells(p.Workers, p.Turns)
				c.events <- AliveCellsCount{CompletedTurns: aturn, CellsCount: alives}
			}
		}
	}()

	// Key presses
	go func() {
		for {
			key := <-keyPresses
			if key == 's' {
				fmt.Println("SSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSS")
				controllerKeyS(p.Workers, p, c)
			} else if key == 'q' {
				fmt.Println("QQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQ")
				controllerKeyQ(p.Workers, p, c)
			} else if key == 'k' {
				fmt.Println("KKKKKKKKKKKKKKKKKKKKKKKKKKKKKKK")
				controllerKeyK(p.Workers, p, c)
				close(done)
				close(c.events)
				os.Exit(0)
			} else if key == 'p' {
				fmt.Println("PPPPPPPPPPPPPPPPPPPPPPPPPPPPPP")
				paused = !paused
				controllerKeyP(p.Workers, p, c, paused)
			}
		}
	}()

	wg.Wait()
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

	close(done)
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

func controllerKeyS(workers []string, p Params, c distributorChannels) {
	minValue := math.MaxInt // 初始化为最大整数，确保第一个值被赋给 minValue
	worksWorld := make(map[int]map[int][][]uint8)
	newWorld := make([][]uint8, p.ImageHeight)
	for i := range newWorld {
		newWorld[i] = make([]uint8, p.ImageWidth)
	}
	startRow := 0

	for i := 0; i < len(workers); i++ {
		client, err := rpc.Dial("tcp", workers[i])
		if err != nil {
			panic(err)
		}
		defer client.Close()

		request := stubs.WorkerKeyRequest{Key: 's'}
		response := new(stubs.WorkerKeyResponse)
		client.Call(stubs.GoLWorkerKeyHandler, request, response)

		worksWorld[i] = response.World
		if response.Latest < minValue {
			minValue = response.Latest
		}
	}

	fmt.Printf("S Key Process trun: %d\n", minValue)

	for _, v := range worksWorld {
		subNewWorld := v[minValue]

		for v := range subNewWorld {
			newWorld[startRow+v] = subNewWorld[v]
		}
	}

	writeNewWorld(newWorld, minValue, p, c)
}

func controllerKeyQ(workers []string, p Params, c distributorChannels) {
	for i := 0; i < len(workers); i++ {
		client, err := rpc.Dial("tcp", workers[i])
		if err != nil {
			panic(err)
		}
		defer client.Close()
		request := stubs.WorkerKeyRequest{Key: 'q'}
		response := new(stubs.WorkerKeyResponse)
		client.Call(stubs.GoLWorkerKeyHandler, request, response)
	}
	fmt.Println("Q Key Process Finish")
}

func controllerKeyK(workers []string, p Params, c distributorChannels) {
	minValue := math.MaxInt // 初始化为最大整数，确保第一个值被赋给 minValue
	worksWorld := make(map[int]map[int][][]uint8)
	newWorld := make([][]uint8, p.ImageHeight)
	for i := range newWorld {
		newWorld[i] = make([]uint8, p.ImageWidth)
	}
	startRow := 0

	for i := 0; i < len(workers); i++ {
		client, err := rpc.Dial("tcp", workers[i])
		if err != nil {
			panic(err)
		}
		defer client.Close()

		request := stubs.WorkerKeyRequest{Key: 'k'}
		response := new(stubs.WorkerKeyResponse)
		client.Call(stubs.GoLWorkerKeyHandler, request, response)

		worksWorld[i] = response.World
		if response.Latest < minValue {
			minValue = response.Latest
		}
	}
	fmt.Printf("K Key Process trun: %d\n", minValue)
	for _, v := range worksWorld {
		subNewWorld := v[minValue]

		for v := range subNewWorld {
			newWorld[startRow+v] = subNewWorld[v]
		}
	}

	writeNewWorld(newWorld, minValue, p, c)
}

func controllerKeyP(workers []string, p Params, c distributorChannels, paused bool) {
	minValue := math.MaxInt // 初始化为最大整数，确保第一个值被赋给 minValue
	worksWorld := make(map[int]map[int][][]uint8)
	newWorld := make([][]uint8, p.ImageHeight)
	for i := range newWorld {
		newWorld[i] = make([]uint8, p.ImageWidth)
	}
	startRow := 0

	for i := 0; i < len(workers); i++ {
		client, err := rpc.Dial("tcp", workers[i])
		if err != nil {
			panic(err)
		}
		defer client.Close()

		request := stubs.WorkerKeyRequest{Key: 'p'}
		response := new(stubs.WorkerKeyResponse)
		client.Call(stubs.GoLWorkerKeyHandler, request, response)

		worksWorld[i] = response.World
		if response.Latest < minValue {
			minValue = response.Latest
		}
	}

	fmt.Printf("P Key Process trun: %d\n", minValue)
	for _, v := range worksWorld {
		subNewWorld := v[minValue]

		for v := range subNewWorld {
			newWorld[startRow+v] = subNewWorld[v]
		}
	}

	if paused {
		c.events <- CellsFlipped{CompletedTurns: minValue, Cells: getAliveCells(newWorld, p.ImageWidth, p.ImageHeight)}
		c.events <- StateChange{minValue, Paused}
	} else {
		c.events <- StateChange{minValue, Executing}
	}
}

func getAllAliveCells(workers []string, turn int) (int, int) {
	count := 0
	minValue := math.MaxInt // 初始化为最大整数，确保第一个值被赋给 minValue
	worksAlice := make(map[int]map[int]int)

	for i := 0; i < len(workers); i++ {
		client, err := rpc.Dial("tcp", workers[i])
		if err != nil {
			panic(err)
		}
		defer client.Close()

		req := stubs.AliveRequest{}
		response := new(stubs.AliveResponse)
		err = client.Call(stubs.GoLGetWorkerAliveCellsHandler, req, response)
		if err != nil {
			panic(err)
		}

		worksAlice[i] = response.CountMap
		if response.Latest < minValue {
			minValue = response.Latest
		}
	}

	for _, v := range worksAlice {
		count += v[minValue]
	}

	return count, minValue
}
