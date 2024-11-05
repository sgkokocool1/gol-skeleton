package gol

type GridRequest struct {
	HaloTop, HaloBottom []uint8
	Iterations          int
	SubGrid             [][]uint8
	StartRow, EndRow    int
}

type AliveRequest struct{}
type AliveResponse struct {
	CountMap map[int]int
	Latest   int
}

type GridResponse struct {
	GridPart [][]uint8
}

type GolService interface {
	ComputeGrid(req GridRequest, res *GridResponse) error
}

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
	Workers     []string
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events chan<- Event, keyPresses <-chan rune) {

	//	TODO: Put the missing channels in here.
	// 创建缺失的通道
	ioFilename := make(chan string) // 文件名通道
	ioOutput := make(chan uint8)    // 文件写入的输出通道
	ioInput := make(chan uint8)     // 文件读取的输入通道
	ioCommand := make(chan ioCommand)
	ioIdle := make(chan bool)

	ioChannels := ioChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: ioFilename,
		output:   ioOutput,
		input:    ioInput,
	}

	p.Workers = make([]string, 1)
	p.Workers[0] = "127.0.0.1:1234"
	//p.Workers[1] = "127.0.0.1:1235"

	go startIo(p, ioChannels)

	distributorChannels := distributorChannels{
		events:     events,
		ioCommand:  ioCommand,
		ioIdle:     ioIdle,
		ioFilename: ioFilename,
		ioOutput:   ioOutput,
		ioInput:    ioInput,
	}
	distributor(p, distributorChannels, keyPresses)
}
