package gol

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"uk.ac.bris.cs/gameoflife/util"
)

// 该模块主要负责：

// 	1.	将游戏状态写入 PGM 图像文件（writePgmImage()）。
// 	2.	从 PGM 图像文件读取数据并初始化游戏状态（readPgmImage()）。
// 	3.	响应控制命令，如读取、写入图像，检查是否处于空闲状态。
// •	类型定义：通道结构、命令类型。
// •	写入 PGM 图像的函数。
// •	读取 PGM 图像的函数。
// •	启动 I/O goroutine 的入口函数。

// ioChannels 是 I/O 操作的通道结构，用于在 I/O 模块与主程序之间通信：

// 	•	command：接收要执行的 I/O 操作命令。
// 	•	idle：通知主程序 I/O goroutine 是否空闲。
// 	•	filename：传递文件名用于读取或写入。
// 	•	output：传递图像数据用于写入文件。
// 	•	input：传递从文件读取的图像数据。
type ioChannels struct {
	command <-chan ioCommand
	idle    chan<- bool

	filename <-chan string
	output   <-chan uint8
	input    chan<- uint8
}

// ioState is the internal ioState of the io goroutine.
type ioState struct {
	params   Params
	channels ioChannels
}

// ioCommand allows requesting behaviour from the io (pgm) goroutine.
type ioCommand uint8

// This is a way of creating enums in Go.
// It will evaluate to:
//		ioOutput 	= 0
//		ioInput 	= 1
//		ioCheckIdle = 2
const (
	ioOutput ioCommand = iota
	ioInput
	ioCheckIdle
)

// writePgmImage receives an array of bytes and writes it to a pgm file.
// •	创建 out 目录以存储输出文件。
// •	接收文件名并创建 .pgm 文件。
// •	写入 PGM 文件的头部（格式 P5，宽度、高度、灰度最大值）。
// •	通过从 output 通道接收像素数据，逐行写入文件。
func (io *ioState) writePgmImage() {
	_ = os.Mkdir("out", os.ModePerm)

	// Request a filename from the distributor.
	filename := <-io.channels.filename

	file, ioError := os.Create("out/" + filename + ".pgm")
	util.Check(ioError)
	defer file.Close()

	_, _ = file.WriteString("P5\n")
	//_, _ = file.WriteString("# PGM file writer by pnmmodules (https://github.com/owainkenwayucl/pnmmodules).\n")
	_, _ = file.WriteString(strconv.Itoa(io.params.ImageWidth))
	_, _ = file.WriteString(" ")
	_, _ = file.WriteString(strconv.Itoa(io.params.ImageHeight))
	_, _ = file.WriteString("\n")
	_, _ = file.WriteString(strconv.Itoa(255))
	_, _ = file.WriteString("\n")

	world := make([][]byte, io.params.ImageHeight)
	for i := range world {
		world[i] = make([]byte, io.params.ImageWidth)
	}

	for y := 0; y < io.params.ImageHeight; y++ {
		for x := 0; x < io.params.ImageWidth; x++ {
			val := <-io.channels.output
			//if val != 0 {
			//	fmt.Println(x, y)
			//}
			world[y][x] = val
		}
	}

	for y := 0; y < io.params.ImageHeight; y++ {
		for x := 0; x < io.params.ImageWidth; x++ {
			_, ioError = file.Write([]byte{world[y][x]})
			util.Check(ioError)
		}
	}

	ioError = file.Sync()
	util.Check(ioError)

	fmt.Println("File", filename, "output done!")
}

// readPgmImage opens a pgm file and sends its data as an array of bytes.
// •	从 filename 通道获取文件名，读取 PGM 文件内容。
// •	检查 PGM 文件头的合法性。
// •	将文件中的像素数据通过 input 通道发送出去。
func (io *ioState) readPgmImage() {
	fmt.Println("start read pgm file")
	// Request a filename from the distributor.
	filename := <-io.channels.filename

	data, ioError := os.ReadFile("images/" + filename + ".pgm")
	util.Check(ioError)

	fields := strings.Fields(string(data))

	if fields[0] != "P5" {
		panic("Not a pgm file")
	}

	width, _ := strconv.Atoi(fields[1])
	if width != io.params.ImageWidth {
		panic("Incorrect width")
	}

	height, _ := strconv.Atoi(fields[2])
	if height != io.params.ImageHeight {
		panic("Incorrect height")
	}

	maxval, _ := strconv.Atoi(fields[3])
	if maxval != 255 {
		panic("Incorrect maxval/bit depth")
	}

	image := []byte(fields[4])
	for _, b := range image {
		io.channels.input <- b
	}

	fmt.Println("File", filename, "input done!")
}

// startIo should be the entrypoint of the io goroutine.
// •	初始化 ioState 实例。
// •	监听 command 通道，执行相应的操作：
// •	ioInput：读取 PGM 文件。
// •	ioOutput：写入 PGM 文件。
// •	ioCheckIdle：向 idle 通道发送 true，表示当前 goroutine 处于空闲状态。
func startIo(p Params, c ioChannels) {
	io := ioState{
		params:   p,
		channels: c,
	}

	for command := range io.channels.command {
		// Block and wait for requests from the distributor
		switch command {
		case ioInput:
			io.readPgmImage()
		case ioOutput:
			io.writePgmImage()
		case ioCheckIdle:
			io.channels.idle <- true
		}
	}
}
