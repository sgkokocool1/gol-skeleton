package gol

import (
	"strconv"

	"distributed/util"
)

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

func createWorld(height int, width int) [][]uint8 {
	world := make([][]uint8, height)
	for i := range world {
		world[i] = make([]uint8, width)
	}
	return world
}

func writeImage(p Params, c distributorChannels, turn int, world [][]uint8) {
	c.ioCommand <- 0

	w := strconv.Itoa(p.ImageWidth)
	h := strconv.Itoa(p.ImageHeight)
	t := strconv.Itoa(p.Turns)
	filename := w + "x" + h + "x" + t
	c.ioFilename <- filename

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}

	c.events <- ImageOutputComplete{CompletedTurns: turn, Filename: filename + ".png"}
}

func getImage(p Params, c distributorChannels, world [][]uint8) [][]uint8 {

	c.ioCommand <- 1

	w := strconv.Itoa(p.ImageWidth)
	h := strconv.Itoa(p.ImageHeight)
	c.ioFilename <- w + "x" + h

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			world[y][x] = <-c.ioInput
		}
	}

	return world
}
