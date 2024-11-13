package main

import (
	"uk.ac.bris.cs/gameoflife/stubs" // 导入 stubs 包，包含与远程过程调用（RPC）相关的结构体和方法
	"uk.ac.bris.cs/gameoflife/util"  // 导入 util 包，包含工具函数，如细胞数据结构
)

// createWorld 函数用于创建一个指定宽度和高度的二维切片，初始化为 0，表示一个空的世界。
// 它的输入参数是世界的宽度和额外的高度。
// 输入：
//   - width: 世界的宽度
//   - extraHeight: 世界的高度
// 输出：返回一个二维切片（矩阵），表示一个空的世界（细胞值为 0）
func createWorld(width, extraHeight int) [][]uint8 {
	world := make([][]uint8, extraHeight) // 创建一个具有 extraHeight 行的二维切片
	for i := range world {                // 遍历每一行
		world[i] = make([]uint8, width) // 为每一行分配 width 列
	}
	return world // 返回创建的二维切片（矩阵）
}

// GetImagePart 函数用于提取当前世界的一个子图，包含上下边界行，用于分布式计算。
// 它将世界的一个部分提取出来，并为该部分增加上下边界行（即环绕边界条件）。
// 输入：
//   - p: 包含图像宽度和高度的参数结构体
//   - startY: 提取部分的起始行
//   - endY: 提取部分的结束行
//   - currentWorld: 当前的世界（细胞矩阵）
// 输出：
//   返回一个包含额外上下边界行的子世界（二维切片）
func GetImagePart(
	p stubs.Params, // 参数结构体，包含图像的宽度和高度
	startY int, // 子图的起始行（即世界中该部分的起始行）
	endY int, // 子图的结束行（即世界中该部分的结束行）
	currentWorld [][]uint8, // 当前世界的二维矩阵（细胞矩阵）
) [][]uint8 {

	// 计算提取部分的高度
	height := endY - startY
	width := p.ImageWidth // 获取图像的宽度

	// 由于需要额外的边界行，子图的高度需要加 2（分别为上下边界）
	extraHeight := height + 2

	// 创建一个新的二维切片，用于存储带有额外边界行的子图
	nodeWorld := createWorld(width, extraHeight)

	// 从 currentWorld 中提取子图，并为其加入额外的边界行
	for i := 0; i < extraHeight; i++ { // 遍历所有的行
		for j := 0; j < width; j++ { // 遍历每一行中的列

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

	// 返回带有额外边界行的子图
	return nodeWorld
}

// CountAliveCells 函数用于统计当前世界中存活的细胞数。
// 它会遍历整个世界，找出所有值为 255（代表存活细胞）的细胞，并计算其数量。
// 输入：
//   - world: 当前的世界（细胞矩阵）
// 输出：
//   返回存活细胞的数量
func CountAliveCells(world [][]uint8) int {
	var cells []util.Cell  // 用来存储存活的细胞
	for i := range world { // 遍历世界中的每一行
		for j := range world[i] { // 遍历每一行中的每一列
			if world[i][j] == 255 { // 如果细胞存活（值为 255）
				// 将存活的细胞位置添加到 cells 数组中
				cells = append(cells, util.Cell{X: j, Y: i})
			}
		}
	}
	// 返回存活细胞的数量
	return len(cells)
}
