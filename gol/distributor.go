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

// Optimized function to initialize the world
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

	fmt.Println("World initialized successfully")
	return world
}

// Efficiently processes a section of the world using goroutines and optimizes neighbor checks
func computeSection(startY, endY int, world, newWorld [][]uint8, width, height int, wg *sync.WaitGroup, c distributorChannels, turn int) {
	defer wg.Done()
	for y := startY; y < endY; y++ {
		for x := 0; x < width; x++ {
			aliveNeighbors := countAliveNeighbors(world, x, y, width, height)
			currentCell := world[y][x]
			if currentCell == 255 && (aliveNeighbors < 2 || aliveNeighbors > 3) {
				newWorld[y][x] = 0
				c.events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: x, Y: y}}
			} else if currentCell == 0 && aliveNeighbors == 3 {
				newWorld[y][x] = 255
				c.events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: x, Y: y}}
			} else {
				newWorld[y][x] = currentCell
			}
		}
	}
}

// Writes the current state of the world to output
func writeNewWorld(world [][]uint8, turn int, p Params, c distributorChannels) {
	filename := fmt.Sprintf("%dx%dx%d", p.ImageWidth, p.ImageHeight, turn)
	c.ioCommand <- ioOutput
	c.ioFilename <- filename

	for _, row := range world {
		for _, cell := range row {
			c.ioOutput <- cell
		}
	}

	c.ioCommand <- ioCheckIdle
	<-c.ioIdle // Wait for IO to finish

	c.events <- ImageOutputComplete{CompletedTurns: turn, Filename: filename}
}

// Main distributor function to manage turns, key presses, and state changes
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
	world := initializeWorld(p, c)
	turn := 0
	paused := false

	c.events <- CellsFlipped{CompletedTurns: turn, Cells: getAliveCells(world, p.ImageWidth, p.ImageHeight)}
	c.events <- StateChange{turn, Executing}

turnLoop:
	for turn < p.Turns {
		select {
		case key := <-keyPresses:
			switch key {
			case 's':
				writeNewWorld(world, turn, p, c)
			case 'p':
				paused = !paused
				newState := Paused
				if !paused {
					newState = Executing
				}
				c.events <- StateChange{turn, newState}
			case 'q':
				break turnLoop
			}
		default:
			if paused {
				continue
			}

			turn++
			newWorld := make([][]uint8, p.ImageHeight)
			for i := range newWorld {
				newWorld[i] = make([]uint8, p.ImageWidth)
			}

			var wg sync.WaitGroup
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

			wg.Wait()

			aliveCells := countAliveCells(newWorld, p.ImageWidth, p.ImageHeight)
			c.events <- AliveCellsCount{CompletedTurns: turn, CellsCount: aliveCells}
			c.events <- TurnComplete{turn}
			world = newWorld
		}
	}

	writeNewWorld(world, turn, p, c)
	aliveCells := getAliveCells(world, p.ImageWidth, p.ImageHeight)
	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: aliveCells}

	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}
	close(c.events)
}

// Helper function to get alive cell positions
func getAliveCells(world [][]uint8, width, height int) []util.Cell {
	cells := make([]util.Cell, 0, width*height)
	for y, row := range world {
		for x, cell := range row {
			if cell == 255 {
				cells = append(cells, util.Cell{X: x, Y: y})
			}
		}
	}
	return cells
}

// Optimized function to count alive neighbors around a cell
func countAliveNeighbors(world [][]uint8, x, y, width, height int) int {
	alive := 0
	neighbors := [][2]int{
		{-1, -1}, {-1, 0}, {-1, 1},
		{0, -1}, {0, 1},
		{1, -1}, {1, 0}, {1, 1},
	}

	for _, n := range neighbors {
		nx, ny := (x+n[1]+width)%width, (y+n[0]+height)%height
		if world[ny][nx] == 255 {
			alive++
		}
	}
	return alive
}

// Helper function to count alive cells in the world
func countAliveCells(world [][]uint8, width, height int) int {
	alive := 0
	for _, row := range world {
		for _, cell := range row {
			if cell == 255 {
				alive++
			}
		}
	}
	return alive
}
