package gol

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"uk.ac.bris.cs/gameoflife/util"
)

// ioChannels 定义了与 I/O 操作相关的通道。
// 这些通道用于与 I/O goroutine 进行通信，传递文件名、图像数据以及 I/O 命令。
type ioChannels struct {
	command  <-chan ioCommand // 用于接收 I/O 操作命令（如读取、写入等）
	idle     chan<- bool      // 用于通知 I/O goroutine 是否空闲
	filename <-chan string    // 用于接收要读取或保存的文件名
	output   <-chan uint8     // 用于接收图像的像素数据
	input    chan<- uint8     // 用于向 I/O goroutine 发送图像数据
}

// ioState 是一个表示 I/O goroutine 内部状态的结构体。
// 它包含了与 I/O 操作相关的参数和通道。
type ioState struct {
	params   Params     // 传入的游戏参数（如图像的宽度、高度等）
	channels ioChannels // 用于 I/O 操作的通道
}

// ioCommand 是用于请求 I/O goroutine 行为的命令类型。
// 使用枚举（通过常量）表示不同的命令，如读取图像、保存图像等。
type ioCommand uint8

// 通过 iota 创建枚举值，表示不同的 I/O 操作类型。
// ioOutput 用于表示保存图像操作
// ioInput 用于表示读取图像操作
// ioCheckIdle 用于检查 I/O goroutine 是否空闲
const (
	ioOutput    ioCommand = iota // 0，表示写入图像
	ioInput                      // 1，表示读取图像
	ioCheckIdle                  // 2，表示检查 I/O 空闲状态
)

// writePgmImage 将一个字节数组保存为 PGM 图像文件。
// 输入：无（从通道 io.channels.output 获取像素数据）
// 输出：无（文件保存成功后打印输出）
func (io *ioState) writePgmImage() {
	_ = os.Mkdir("out", os.ModePerm) // 创建输出目录，若已存在则忽略

	// 从通道中获取文件名
	filename := <-io.channels.filename

	// 创建一个新的 PGM 文件
	file, ioError := os.Create("out/" + filename + ".pgm")
	util.Check(ioError) // 检查文件创建是否成功
	defer file.Close()  // 函数结束时关闭文件

	// 写入 PGM 文件的头部信息
	_, _ = file.WriteString("P5\n")                             // PGM 文件格式标识
	_, _ = file.WriteString(strconv.Itoa(io.params.ImageWidth)) // 写入图像宽度
	_, _ = file.WriteString(" ")
	_, _ = file.WriteString(strconv.Itoa(io.params.ImageHeight)) // 写入图像高度
	_, _ = file.WriteString("\n")
	_, _ = file.WriteString(strconv.Itoa(255)) // 写入最大像素值（通常是 255）
	_, _ = file.WriteString("\n")

	// 创建一个二维字节数组来存储图像数据
	world := make([][]byte, io.params.ImageHeight)
	for i := range world {
		world[i] = make([]byte, io.params.ImageWidth)
	}

	// 从输出通道获取每个像素的值，并填充到 world 数组中
	for y := 0; y < io.params.ImageHeight; y++ {
		for x := 0; x < io.params.ImageWidth; x++ {
			val := <-io.channels.output // 获取像素值
			world[y][x] = val           // 填充到对应位置
		}
	}

	// 将像素数据写入文件
	for y := 0; y < io.params.ImageHeight; y++ {
		for x := 0; x < io.params.ImageWidth; x++ {
			_, ioError = file.Write([]byte{world[y][x]}) // 写入每个像素值
			util.Check(ioError)                          // 检查写入是否成功
		}
	}

	// 刷新文件内容，确保所有数据写入磁盘
	ioError = file.Sync()
	util.Check(ioError)

	// 打印输出保存完成的消息
	fmt.Println("File", filename, "output done!")
}

// readPgmImage 读取 PGM 文件并将数据传递到输入通道中。
// 输入：无（从文件中读取图像数据）
// 输出：无（数据通过 io.channels.input 通道发送）
func (io *ioState) readPgmImage() {

	// 从通道中获取文件名
	filename := <-io.channels.filename

	// 读取指定的 PGM 文件
	data, ioError := os.ReadFile("images/" + filename + ".pgm")
	util.Check(ioError) // 检查文件读取是否成功

	// 将文件内容转换为字符串并分割成字段
	fields := strings.Fields(string(data))

	// 检查 PGM 文件格式是否正确
	if fields[0] != "P5" {
		panic("Not a pgm file") // 如果不是 P5 格式的文件，抛出错误
	}

	// 检查图像的宽度是否与预期匹配
	width, _ := strconv.Atoi(fields[1])
	if width != io.params.ImageWidth {
		panic("Incorrect width") // 宽度不匹配，抛出错误
	}

	// 检查图像的高度是否与预期匹配
	height, _ := strconv.Atoi(fields[2])
	if height != io.params.ImageHeight {
		panic("Incorrect height") // 高度不匹配，抛出错误
	}

	// 检查最大值是否为 255（标准 PGM 文件）
	maxval, _ := strconv.Atoi(fields[3])
	if maxval != 255 {
		panic("Incorrect maxval/bit depth") // 最大值不匹配，抛出错误
	}

	// 获取图像的像素数据
	image := []byte(fields[4])

	// 将像素数据发送到输入通道
	for _, b := range image {
		io.channels.input <- b
	}

	// 打印输出读取完成的消息
	fmt.Println("File", filename, "input done!")
}

// startIo 是 I/O goroutine 的入口函数。
// 它等待来自 distributor 的命令并执行相应的 I/O 操作。
// 输入：p（Params）和 c（ioChannels）
// 输出：无（异步执行 I/O 操作）
func startIo(p Params, c ioChannels) {
	io := ioState{
		params:   p,
		channels: c,
	}

	// 循环等待 I/O 命令并处理
	for command := range io.channels.command {
		// 根据收到的命令执行相应的操作
		switch command {
		case ioInput:
			io.readPgmImage() // 读取图像
		case ioOutput:
			io.writePgmImage() // 保存图像
		case ioCheckIdle:
			io.channels.idle <- true // 检查是否空闲
		}
	}
}
