package gol

import (
	"fmt"
	"sync"

	"uk.ac.bris.cs/gameoflife/util"
)

// 1.	初始化游戏世界。
// 2.	多线程并行计算世界的下一个状态。
// 3.	处理用户输入控制（暂停、保存、退出）。
// 4.	输出游戏状态到文件。

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

//从文件读取初始世界状态。
// •	根据图像的宽高生成文件名。
// •	通过 ioCommand 和 ioFilename 请求 I/O goroutine 从文件读取数据。
// •	从 ioInput 通道逐行接收数据，填充到二维切片 world 中。
// •	返回初始化后的世界数据。
// initializeWorld 初始化世界（二维数组），加载图像数据。
// 输入：Params 类型的 p (包含图像的宽度和高度)，
//       distributorChannels 类型的 c (用于与分发器和其他通道通信)
// 输出：返回一个[][]uint8 类型的二维切片，表示世界的状态。
func initializeWorld(p Params, c distributorChannels) [][]uint8 {
	// 通过图像的宽度和高度生成一个文件名，格式为 "宽度x高度"
	filename := fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight)

	// 向 ioCommand 通道发送一个命令，表示要从图像源读取数据
	c.ioCommand <- ioInput

	// 向 ioFilename 通道发送文件名，告诉接收方要加载的图像文件
	c.ioFilename <- filename

	// 创建一个二维切片，用于存储世界的状态，行数为图像的高度
	world := make([][]uint8, p.ImageHeight)

	// 遍历每一行，初始化每一行的切片
	for i := range world {
		// 为每一行分配列的空间，列数为图像的宽度
		world[i] = make([]uint8, p.ImageWidth)

		// 遍历每一列，从 ioInput 通道获取像素值并赋值到当前单元格
		for j := range world[i] {
			// 从 ioInput 通道读取图像的每个像素值（0-255）
			world[i][j] = <-c.ioInput
		}
	}

	// 打印初始化成功的提示信息
	fmt.Println("initializeworld successful")

	// 返回加载的世界（二维切片）
	return world
}

//并行计算局部区域的细胞状态更新。
// •	并行处理某个区块（startY 到 endY）的细胞状态。
// •	计算每个细胞周围的活邻居数。
// •	根据规则决定细胞的生死变化，并向 events 通道报告细胞状态变化。
// computeSection 计算世界状态的一个部分并更新新世界（newWorld）。
// 输入：startY (起始行)，endY (结束行)，world (当前世界状态)，
//       newWorld (新的世界状态)，width (宽度)，height (高度)，
//       wg (同步等待组，用于等待所有 goroutine 完成)，
//       c (分发通道，用于发送状态变化事件)，turn (当前回合)。
// 输出：更新 newWorld，函数没有返回值。
//       会通过 c.events 发送状态变化的事件（CellFlipped）。
func computeSection(startY, endY int, world [][]uint8, newWorld [][]uint8, width, height int, wg *sync.WaitGroup, c distributorChannels, turn int) {
	// 确保函数结束时通知等待组减少计数
	defer wg.Done()

	// 遍历从 startY 到 endY 的行（部分区域的计算）
	for y := startY; y < endY; y++ {
		// 遍历每一列
		for x := 0; x < width; x++ {
			// 计算当前细胞周围的活邻居数量
			aliveNeighbors := countAliveNeighbors(world, x, y, width, height)

			// 如果当前细胞是活的（值为255）
			if world[y][x] == 255 {
				// 如果活邻居数小于2或大于3，该细胞死亡
				if aliveNeighbors < 2 || aliveNeighbors > 3 {
					newWorld[y][x] = 0 // 细胞死亡
					// 发送死亡事件
					c.events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: x, Y: y}}
				} else {
					// 否则，细胞保持活着
					newWorld[y][x] = 255 // 细胞保持活
				}
			} else if aliveNeighbors == 3 {
				// 如果当前细胞是死的并且有恰好3个活邻居，细胞复活
				newWorld[y][x] = 255 // 细胞复活
				// 发送复活事件
				c.events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: x, Y: y}}
			} else {
				// 否则，细胞保持死亡
				newWorld[y][x] = 0 // 细胞保持死
			}
		}
	}
}

//将当前世界状态保存到文件。
// writeNewWorld 将世界的状态（world）输出到文件，并通过事件通道发送输出完成的通知。
// 输入：world ([][]uint8) 当前世界的状态，一个二维数组表示每个细胞的状态；
//       turn (int) 当前的回合数，用于生成文件名；
//       p (Params) 包含世界的宽度和高度，用于文件命名和循环控制；
//       c (distributorChannels) 包含通道，用于与其他组件通信。
// 输出：无，函数会将 world 的内容写入指定的输出文件，并通过事件通道发送 `ImageOutputComplete` 事件。
//       该事件标识图像输出已完成。
func writeNewWorld(world [][]uint8, turn int, p Params, c distributorChannels) {
	// 生成文件名，格式为 "宽度x高度x回合数"（例如 "100x100x5"）
	filename := fmt.Sprintf("%dx%dx%d", p.ImageWidth, p.ImageHeight, turn)

	// 向 IO 通道发送输出命令，指示 IO 进行输出操作
	c.ioCommand <- ioOutput

	// 通过文件名通道传送生成的文件名
	c.ioFilename <- filename

	// 遍历 world 数组，将每个细胞的状态（0 或 255）发送到 IO 输出通道
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			// 将当前细胞的状态发送到输出通道
			c.ioOutput <- world[y][x]
		}
	}

	// 向 IO 通道发送检查是否空闲的命令
	c.ioCommand <- ioCheckIdle

	// 等待接收 IO 空闲信号，确保所有输出操作已完成
	<-c.ioIdle

	// 通过事件通道发送图像输出完成的事件，包含回合数和文件名
	c.events <- ImageOutputComplete{CompletedTurns: turn, Filename: filename}
}

//核心调度器，控制游戏逻辑循环、线程调度和 I/O 操作。
// •	输入：
// •	p Params：包含游戏的配置信息，如图像的宽度和高度、回合数、线程数等。
// •	c distributorChannels：包含多个通道用于与其他部分（如 IO 操作、事件发送等）通信。
// •	keyPresses <-chan rune：一个通道，接收按键事件（例如 ‘s’、‘p’、‘q’）来控制游戏暂停、保存状态或退出。
// •	输出：
// •	无直接返回值。该函数负责执行游戏的主要逻辑，包括计算新的世界状态、响应按键输入、写入文件、发送事件等。
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
	// 创建一个 2D 切片来存储当前状态的世界
	// 初始化世界状态并开始游戏循环
	world := initializeWorld(p, c) // 调用初始化函数来创建世界
	turn := 0                      // 初始回合数为 0
	paused := false                // 游戏初始为未暂停状态

	// 发送初始状态的活细胞事件，并发送状态更改事件（开始执行）
	c.events <- CellsFlipped{CompletedTurns: turn, Cells: getAliveCells(world, p.ImageWidth, p.ImageHeight)}
	c.events <- StateChange{turn, Executing}

	// 主游戏循环（turnLoop）
turnLoop:
	for turn < p.Turns {
		// 监听按键事件
		select {
		case key := <-keyPresses:
			if key == 's' {
				// 按下 's' 键，保存当前世界状态
				fmt.Printf("sssssssssssssssss,turn: %d\n", turn)
				writeNewWorld(world, turn, p, c) // 保存当前世界状态到文件
			}
			if key == 'p' {
				// 按下 'p' 键，切换暂停状态
				fmt.Printf("pppppppppppppppp,turn: %d\n", turn)
				paused = !paused // 切换暂停状态
				if paused {
					// 如果暂停，发送暂停事件
					c.events <- StateChange{turn, Paused}
				} else {
					// 如果恢复执行，发送恢复事件
					c.events <- StateChange{turn, Executing}
				}
			}
			if key == 'q' {
				// 按下 'q' 键，退出循环
				fmt.Printf("qqqqqqqqqqqqqq,turn: %d\n", turn)
				break turnLoop // 跳出循环，结束游戏
			}
		default:
			// 没有按键按下时继续游戏逻辑
			if paused {
				continue // 如果游戏处于暂停状态，继续等待按键输入
			}

			turn++ // 增加回合数

			// 创建一个新的 world 用于存储下一回合的状态
			var wg sync.WaitGroup
			newWorld := make([][]uint8, p.ImageHeight)
			for i := range newWorld {
				newWorld[i] = make([]uint8, p.ImageWidth)
			}

			// 分块并行处理：将世界分割成多个区域并使用 goroutines 并行计算每个区域的状态
			rowsPerThread := p.ImageHeight / p.Threads // 每个线程负责的行数
			for t := 0; t < p.Threads; t++ {
				startY := t * rowsPerThread
				endY := (t + 1) * rowsPerThread
				if t == p.Threads-1 {
					endY = p.ImageHeight // 最后一个线程处理剩余的行
				}
				wg.Add(1)
				go computeSection(startY, endY, world, newWorld, p.ImageWidth, p.ImageHeight, &wg, c, turn) // 启动并行计算
			}

			// 等待所有线程完成
			wg.Wait()

			// 计算新世界并执行下一步操作（例如：更新细胞状态）

			// 计算活细胞数量并发送 `AliveCellsCount` 和 `TurnComplete` 事件
			aliveCells := countAliveCells(newWorld, p.ImageWidth, p.ImageHeight)      // 统计活细胞数
			c.events <- AliveCellsCount{CompletedTurns: turn, CellsCount: aliveCells} // 发送活细胞数事件
			c.events <- TurnComplete{turn}                                            // 发送回合完成事件

			world = newWorld // 更新世界状态
		}
	}

	// 游戏结束后，写入最终的世界状态
	writeNewWorld(world, turn, p, c) // 保存最后一个回合的世界状态

	// 获取所有活细胞的坐标，并发送最终的回合完成事件
	aliveCells := getAliveCells(world, p.ImageWidth, p.ImageHeight)        // 获取活细胞的坐标
	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: aliveCells} // 发送最终状态报告

	// 确保 IO 完成任何输出操作
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle // 等待 IO 完成

	// 通知系统游戏退出，发送退出事件
	c.events <- StateChange{turn, Quitting}

	// 关闭事件通道，通知 SDL 协程退出
	close(c.events)
}

// getAliveCells 提取世界中活细胞的坐标，返回一个 []util.Cell 切片
// •	输入：
// •	world [][]uint8：表示游戏世界的二维切片，其中每个位置的值是 uint8 类型，通常 255 表示活细胞，0 表示死细胞。
// •	width int：世界的宽度（列数）。
// •	height int：世界的高度（行数）。
// •	输出：
// •	返回一个切片 []util.Cell，其中每个元素表示一个活细胞的坐标，util.Cell 是一个结构体，通常包含 X 和 Y 坐标。
func getAliveCells(world [][]uint8, width, height int) []util.Cell {
	var cells []util.Cell // 创建一个空切片，用于存储活细胞的坐标

	// 遍历世界中的每一个位置
	for y := 0; y < height; y++ { // 遍历行
		for x := 0; x < width; x++ { // 遍历列
			if world[y][x] == 255 { // 如果该位置的细胞是活的（255）
				// 将该活细胞的坐标 (x, y) 添加到 cells 切片中
				cells = append(cells, util.Cell{X: x, Y: y})
			}
		}
	}

	return cells // 返回所有活细胞的坐标切片
}

// countAliveNeighbors 计算指定单元格周围的活邻居数量
// •	输入：
// •	world [][]uint8：表示游戏世界的二维切片，每个位置的值为 uint8 类型，255 表示活细胞，0 表示死细胞。
// •	x int：当前检查位置的列索引。
// •	y int：当前检查位置的行索引。
// •	width int：世界的宽度（列数）。
// •	height int：世界的高度（行数）。
// •	输出：
// •	返回一个整数，表示给定位置 (x, y) 的细胞周围的活细胞数量。
func countAliveNeighbors(world [][]uint8, x, y, width, height int) int {
	alive := 0 // 初始化变量，用于统计活细胞数量

	// 定义所有8个邻居的位置，相对于中心细胞的位置偏移
	neighbors := [][2]int{
		{-1, -1}, {-1, 0}, {-1, 1}, // 左上，上，右上
		{0, -1}, {0, 1}, // 左， 右
		{1, -1}, {1, 0}, {1, 1}, // 左下，下，右下
	}

	// 遍历所有邻居的位置
	for _, n := range neighbors {
		nx, ny := x+n[1], y+n[0] // 计算邻居的位置 (nx, ny)

		// 检查是否越界，如果越界则使用环绕的方式
		if nx < 0 { // 如果邻居的 x 坐标小于 0，则环绕到最右边
			nx = width - 1
		} else if nx >= width { // 如果邻居的 x 坐标超出最大宽度，则环绕到最左边
			nx = 0
		}

		if ny < 0 { // 如果邻居的 y 坐标小于 0，则环绕到最底部
			ny = height - 1
		} else if ny >= height { // 如果邻居的 y 坐标超出最大高度，则环绕到最顶部
			ny = 0
		}

		// 检查该邻居位置是否是活细胞
		if world[ny][nx] == 255 {
			alive++ // 如果是活细胞，增加计数
		}
	}

	return alive // 返回周围活细胞的数量
}

// countAliveCells 计算当前世界中的活细胞数量
// •	输入：
// •	world [][]uint8：表示世界的二维切片，其中每个元素为 uint8 类型，255 表示活细胞，0 表示死细胞。
// •	width int：世界的宽度（列数）。
// •	height int：世界的高度（行数）。
// •	输出：
// •	返回一个整数，表示在给定的世界中活细胞的总数。
// 定义函数，用于计算世界中活细胞的数量
func countAliveCells(world [][]uint8, width, height int) int {
	alive := 0 // 初始化变量，用于统计活细胞数量

	// 遍历世界中的每一个位置
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// 检查当前细胞是否是活细胞
			if world[y][x] == 255 {
				alive++ // 如果是活细胞，增加计数
			}
		}
	}

	return alive // 返回活细胞的数量
}
