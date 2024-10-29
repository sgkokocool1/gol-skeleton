package gol

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"time"
)

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

// 初始化一个随机的细胞矩阵
func initializeRandomMatrix(width, height int) [][]uint8 {
	rand.Seed(time.Now().UnixNano())
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
		for j := range matrix[i] {
			if rand.Intn(2) == 1 {
				matrix[i][j] = 255 // 255 表示活细胞
			} else {
				matrix[i][j] = 0 // 0 表示死细胞
			}
		}
	}
	return matrix
}

// 将细胞矩阵保存为 PNG 图像
func saveMatrixAsPNG(matrix [][]uint8, filename string) error {
	height := len(matrix)
	width := len(matrix[0])
	img := image.NewGray(image.Rect(0, 0, width, height))
	for y, row := range matrix {
		for x, cell := range row {
			// 将每个细胞的值设置为图像的灰度值
			img.SetGray(x, y, color.Gray{Y: cell})
		}
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events chan<- Event, keyPresses <-chan rune) {
	filename := fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight)
	// matrix := initializeRandomMatrix(p.ImageWidth, p.ImageHeight)
	// // 保存初始状态为 PGM 图像
	// err := saveMatrixAsPNG(matrix, "images/"+filename+".pgm")
	// if err != nil {
	// 	panic(err)
	// }

	//	TODO: Put the missing channels in here.
	// 创建缺失的通道
	ioFilename := make(chan string) // 文件名通道
	ioFilename <- filename

	output := make(chan uint8) // 文件写入的输出通道
	input := make(chan uint8)  // 文件读取的输入通道

	ioCommand := make(chan ioCommand)
	ioCommand <- ioInput

	ioIdle := make(chan bool)

	ioChannels := ioChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: ioFilename,
		output:   output,
		input:    input,
	}
	go startIo(p, ioChannels)

	distributorChannels := distributorChannels{
		events:     events,
		ioCommand:  ioCommand,
		ioIdle:     ioIdle,
		ioFilename: ioFilename,
		ioOutput:   output,
		ioInput:    input,
	}
	distributor(p, distributorChannels)
}
