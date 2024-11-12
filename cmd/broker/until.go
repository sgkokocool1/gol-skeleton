package main

import (
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

func createWorld(width, extraHeight int) [][]uint8 {
	world := make([][]uint8, extraHeight)
	for i := range world {
		world[i] = make([]uint8, width)
	}
	return world
}

// •	该函数的主要目的是在进行分布式计算时，将世界（细胞矩阵）分割为多个部分发送给不同的节点（Nodes）。
// •	为了处理边界细胞的计算，提取的子图会多包含一行上下的边界行，以模拟环绕边界条件。
// •	这种处理方式确保在进行细胞状态更新时，节点能够正确地考虑相邻行的细胞状态，即使这些相邻行跨越了世界的上下边界。
func GetImagePart(
	p stubs.Params, // 参数结构体，包含图像宽度和高度
	startY int, // 提取部分的起始行
	endY int, // 提取部分的结束行
	currentWorld [][]uint8, // 当前世界（细胞矩阵）
) [][]uint8 {

	// 计算提取部分的高度
	height := endY - startY
	width := p.ImageWidth

	// 由于需要包含上下额外的边界行，因此高度要加 2
	extraHeight := height + 2

	// 创建一个新的二维切片，用于存储带有额外边界的子图
	nodeWorld := createWorld(width, extraHeight)

	// 从 currentWorld 中提取子图，并加入额外的边界行
	for i := 0; i < extraHeight; i++ {
		for j := 0; j < width; j++ {

			// 如果是子图的第一行，且起始行在整个世界的顶部，则添加原世界最后一行的数据
			if i == 0 && startY == 0 {
				nodeWorld[i][j] = currentWorld[p.ImageHeight-1][j]

				// 如果是子图的最后一行，且结束行在整个世界的底部，则添加原世界的第一行的数据
			} else if i == height+1 && startY+height == p.ImageHeight {
				nodeWorld[i][j] = currentWorld[0][j]

				// 否则，直接从 currentWorld 中提取对应行的数据
			} else {
				a := startY + i - 1 // 计算在 currentWorld 中对应的行
				nodeWorld[i][j] = currentWorld[a][j]
			}
		}
	}

	// 返回提取并扩展后的子图
	return nodeWorld
}

func CountAliveCells(world [][]uint8) int {
	var cells []util.Cell
	for i := range world {
		for j := range world[i] {
			if world[i][j] == 255 {
				cells = append(cells, util.Cell{X: j, Y: i})
			}
		}
	}
	return len(cells)
}
