package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"

	"uk.ac.bris.cs/gameoflife/stubs"
)

type Worker struct{}

// •	使用 flag 解析命令行参数，指定 Worker 节点的监听地址和端口。
// 	•	创建一个 Worker 对象，并注册到 RPC 服务器。
// 	•	使用 net.Listen 创建 TCP 监听器，监听传入的连接。
// 	•	通过 listener.Accept() 接收连接并启动一个新的 goroutine 来处理每个连接，使用 rpc.ServeConn(conn) 处理 RPC 调用。
func main() {
	// Set up the worker to listen on a specified port
	pAddr := flag.String("port", "0.0.0.0:8085", "IP and port to listen on")
	flag.Parse()

	worker := &Worker{}
	err := rpc.Register(worker)
	if err != nil {
		fmt.Println("Error registering RPC server:", err)
		return
	}

	listener, err := net.Listen("tcp", *pAddr)
	if err != nil {
		fmt.Println("Error starting listener:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Worker listening on " + *pAddr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		go rpc.ServeConn(conn)
	}
}

// HandleNextState processes the world state and calculates the next state.
// 处理下一个状态计算
func (w *Worker) HandleNextState(request stubs.Request, response *stubs.Response) error {
	nextWorld := calculateNextWorld(request.World)
	response.Status = "OK"
	response.World = nextWorld
	return nil
}

// calculateNextWorld generates the next state of the world based on Conway's Game of Life rules.
// •	计算当前世界状态的下一状态。
// 	•	对每个细胞，统计其周围活细胞的数量，并根据 Game of Life 的规则决定该细胞在下一状态是存活还是死亡。
func calculateNextWorld(currentWorld [][]uint8) [][]uint8 {
	height := len(currentWorld)
	width := len(currentWorld[0])
	nextWorld := make([][]uint8, height)

	for i := range currentWorld {
		nextWorld[i] = make([]uint8, width)
		for j := range currentWorld[i] {
			liveNeighbours := countLiveNeighbours(i, j, currentWorld)
			nextWorld[i][j] = applyRules(currentWorld[i][j], liveNeighbours)
		}
	}
	return nextWorld
}

// applyRules applies the Game of Life rules to determine the next state of a cell.
func applyRules(cell uint8, liveNeighbours int) uint8 {
	if cell == 255 && (liveNeighbours < 2 || liveNeighbours > 3) {
		return 0 // Cell dies
	}
	if cell == 0 && liveNeighbours == 3 {
		return 255 // Cell becomes alive
	}
	return cell // No change
}

// countLiveNeighbours counts the number of live neighbors around a specific cell.
func countLiveNeighbours(i, j int, world [][]uint8) int {
	// •	定义了一个长度为 8 的二维数组 neighborOffsets，每个元素表示相对于当前细胞的邻居的坐标偏移量。
	// •	这 8 个偏移量分别代表当前细胞周围 8 个方向（上左、上、上右、左、右、下左、下、下右）的邻居。
	neighborOffsets := [8][2]int{
		{-1, -1}, {-1, 0}, {-1, 1},
		{0, -1}, {0, 1},
		{1, -1}, {1, 0}, {1, 1},
	}
	// •	计算世界的高度（行数）和宽度（列数）。
	// •	height 是 world 的行数，即二维切片的长度。
	// •	width 是 world 的列数，即第一行的长度。
	height := len(world)
	width := len(world[0])
	liveNeighbours := 0

	// •	遍历 neighborOffsets 中的每一个偏移量 offset。
	// •	这里使用了 range 循环，offset 是一个长度为 2 的数组，表示某个邻居的相对位置。
	for _, offset := range neighborOffsets {
		// •	计算邻居在 world 中的实际坐标：
		// •	ni 是邻居的行索引，nj 是列索引。
		// •	(i + offset[0] + height) % height：这里使用了模运算确保索引在 0 到 height-1 的范围内。如果超出边界，会环绕到另一侧。
		// •	(j + offset[1] + width) % width：同理，确保列索引在 0 到 width-1 的范围内，实现环绕。
		ni := (i + offset[0] + height) % height
		nj := (j + offset[1] + width) % width
		if world[ni][nj] == 255 {
			liveNeighbours++
		}
	}
	return liveNeighbours
}

// CloseNode shuts down the worker node.
func (w *Worker) CloseNode(request stubs.BlankRequest, response *stubs.Response) error {
	fmt.Println("Closing node...")
	os.Exit(0)
	return nil
}
