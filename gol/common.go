package gol

import (
	"strconv"

	"uk.ac.bris.cs/gameoflife/util"
)

// getAliveCells 提取世界中活细胞的坐标，返回一个 []util.Cell 切片
// •	函数作用：遍历整个二维世界（world），提取所有活细胞的坐标，并将其存储在 []util.Cell 切片中返回。
// •	world：二维数组，表示当前的细胞世界。
// •	width 和 height：分别表示世界的宽度和高度。
// •	cells：用于存储活细胞的切片。
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

// •	函数作用：创建一个空的二维世界，所有细胞初始状态为 0（死细胞）。
// •	height 和 width：表示世界的维度。
// •	world：二维数组，表示初始化后的世界。
func createWorld(height int, width int) [][]uint8 {
	world := make([][]uint8, height)
	for i := range world {
		world[i] = make([]uint8, width)
	}
	return world
}

// •	函数作用：将当前世界状态写入文件。
// •	p Params：包含世界的参数（如宽度、高度等）。
// •	c distributorChannels：用于与 IO 模块通信的通道集合。
// •	turn：当前回合数。
// •	world：当前世界的状态。
// •	c.ioCommand <- 0：发送写入图像的命令（0 通常表示写入）。
func writeImage(p Params, c distributorChannels, turn int, world [][]uint8) {
	c.ioCommand <- 0

	w := strconv.Itoa(p.ImageWidth)
	h := strconv.Itoa(p.ImageHeight)
	t := strconv.Itoa(turn)
	filename := w + "x" + h + "x" + t
	c.ioFilename <- filename

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}

	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- ImageOutputComplete{CompletedTurns: turn, Filename: filename}
}

// •	函数作用：从文件中读取图像数据并加载到 world。
// •	c.ioCommand <- 1：发送读取图像的命令（1 通常表示读取）。
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
