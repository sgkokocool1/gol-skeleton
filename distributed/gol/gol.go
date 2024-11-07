package gol

import "strings"

// 定义一个新类型 StringSlice，用来实现对 []string 的处理
type StringSlice []string

// 实现 flag.Value 接口的 Set 方法
func (s *StringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// 实现 flag.Value 接口的 String 方法，用于打印标志值
func (s *StringSlice) String() string {
	return strings.Join(*s, ", ")
}

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events chan<- Event, keyPresses <-chan rune) {

	//	TODO: Put the missing channels in here.
	// 创建缺失的通道
	ioFilename := make(chan string) // 文件名通道
	ioOutput := make(chan uint8, p.ImageHeight*p.ImageWidth)
	ioInput := make(chan uint8, p.ImageHeight*p.ImageWidth)
	ioCommand := make(chan ioCommand)
	ioIdle := make(chan bool)

	ioChannels := ioChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: ioFilename,
		output:   ioOutput,
		input:    ioInput,
	}

	go startIo(p, ioChannels)

	distributorChannels := distributorChannels{
		events:     events,
		ioCommand:  ioCommand,
		ioIdle:     ioIdle,
		ioFilename: ioFilename,
		ioOutput:   ioOutput,
		ioInput:    ioInput,
		ioKeyPress: keyPresses,
	}
	distributor(p, distributorChannels)
}
