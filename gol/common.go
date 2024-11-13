package gol

import (
	"strconv" // 导入strconv包，用于将整数转化为字符串

	"uk.ac.bris.cs/gameoflife/util" // 导入自定义的util包，包含Cell结构等定义
)

// getAliveCells 提取世界中活细胞的坐标，返回一个 []util.Cell 切片。
// •	函数作用：遍历整个二维世界（world），提取所有活细胞的坐标，并将其存储在 []util.Cell 切片中返回。
// •	world：二维数组，表示当前的细胞世界。每个元素表示一个细胞的状态，0表示死细胞，255表示活细胞。
// •	width 和 height：分别表示世界的宽度和高度。
// •	返回值：返回一个存储活细胞坐标的切片，每个元素是一个util.Cell结构体，包含X和Y坐标。
// •	世界世界中的每个活细胞都会被遍历，并将其坐标添加到切片中。
func getAliveCells(world [][]uint8, width, height int) []util.Cell {
	var cells []util.Cell // 创建一个空切片，存储活细胞的坐标
	// 遍历整个世界的二维数组
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// 如果当前细胞是活细胞（值为255），则将其坐标加入切片
			if world[y][x] == 255 {
				cells = append(cells, util.Cell{X: x, Y: y}) // 将活细胞的坐标（x, y）添加到切片中
			}
		}
	}
	// 返回包含所有活细胞坐标的切片
	return cells
}

// createWorld 创建一个空的二维世界，所有细胞初始状态为0（死细胞）
// •	函数作用：初始化一个二维数组作为世界，所有细胞初始状态为死细胞（0）
// •	height 和 width：表示世界的维度，height是世界的高度（行数），width是世界的宽度（列数）。
// •	返回值：返回一个二维数组，表示初始化后的世界，所有元素值为0，代表所有细胞都处于死亡状态。
// •	在二维数组中，索引值表示位置，值为0表示死细胞。
func createWorld(height int, width int) [][]uint8 {
	// 创建一个长度为height的二维切片
	world := make([][]uint8, height)
	// 为二维切片的每一行分配内存，宽度为width
	for i := range world {
		world[i] = make([]uint8, width) // 每行的宽度为width，元素初始为0（死细胞）
	}
	// 返回初始化后的世界
	return world
}

// writeImage 将当前世界状态写入文件。
// •	函数作用：将当前世界的状态以图像的形式写入文件。
// •	p Params：包含世界的参数（如宽度、高度等），通过这些参数生成文件名。
// •	c distributorChannels：用于与 I/O 模块通信的通道集合。
// •	turn：当前回合数。
// •	world：当前世界的状态，二维数组表示世界中的每个细胞状态。
// •	c.ioCommand <- 0：向IO模块发送写入命令（0通常表示写入）。
// •	该函数会将world中的每个细胞的状态依次发送给IO模块，最后通知文件写入完成。
func writeImage(p Params, c distributorChannels, turn int, world [][]uint8) {
	c.ioCommand <- 0 // 向IO模块发送写入命令

	// 将宽度、长度、回合数转换为字符串
	w := strconv.Itoa(p.ImageWidth)
	h := strconv.Itoa(p.ImageHeight)
	t := strconv.Itoa(turn)
	// 拼接成文件名，格式为 "宽度x高度x回合数"
	filename := w + "x" + h + "x" + t
	c.ioFilename <- filename // 将文件名发送到IO模块

	// 遍历world中的每个细胞，发送其状态
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x] // 发送细胞状态（0或255）到IO模块
		}
	}

	// 告诉IO模块文件写入完毕，检查IO模块是否处于空闲状态
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle // 等待IO模块空闲

	// 发送图像输出完成的事件
	c.events <- ImageOutputComplete{CompletedTurns: turn, Filename: filename}
}

// getImage 从文件中读取图像数据并加载到 world。
// •	函数作用：从IO模块读取图像数据，将其加载到世界状态（world）中。
// •	p Params：包含世界的参数（如宽度、高度等）。
// •	c distributorChannels：用于与 I/O 模块通信的通道集合。
// •	world：二维数组，表示当前世界，函数会将读取的数据填充到这个数组。
// •	c.ioCommand <- 1：向IO模块发送读取命令（1通常表示读取）。
// •	该函数会将读取的world数据返回，并更新world的状态。
func getImage(p Params, c distributorChannels, world [][]uint8) [][]uint8 {

	c.ioCommand <- 1 // 向IO模块发送读取命令

	// 将宽度和高度转换为字符串并发送到IO模块作为文件名
	w := strconv.Itoa(p.ImageWidth)
	h := strconv.Itoa(p.ImageHeight)
	c.ioFilename <- w + "x" + h

	// 遍历world的每个位置，从IO模块读取数据并填充到world中
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			world[y][x] = <-c.ioInput // 从IO模块读取细胞状态并存储到world中
		}
	}

	// 返回更新后的world
	return world
}
