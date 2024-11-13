package main

import (
	"flag"    // 导入 flag 包，解析命令行参数
	"fmt"     // 导入 fmt 包，用于格式化输出
	"net"     // 导入 net 包，用于网络通信
	"net/rpc" // 导入 rpc 包，用于实现远程过程调用
	"os"      // 导入 os 包，用于操作系统级别的功能（如退出程序）

	"uk.ac.bris.cs/gameoflife/stubs" // 导入 stubs 包，包含 RPC 请求和响应结构体
)

// Worker 类型用于表示工作节点，每个 Worker 会处理计算任务。
type Worker struct{}

// main 函数是程序的入口，执行以下步骤：
// 1. 使用 flag 解析命令行参数，指定 Worker 节点的监听地址和端口。
// 2. 创建一个 Worker 对象，并注册到 RPC 服务器。
// 3. 使用 net.Listen 创建 TCP 监听器，监听传入的连接。
// 4. 使用 listener.Accept() 接收连接并启动新的 goroutine 来处理每个连接，
//    使用 rpc.ServeConn(conn) 处理 RPC 调用。
func main() {
	// 解析命令行参数，指定默认监听地址为 "0.0.0.0:8085"（即本机所有网卡的 8085 端口）
	pAddr := flag.String("port", "0.0.0.0:8085", "IP and port to listen on")
	flag.Parse() // 解析命令行参数

	worker := &Worker{}         // 创建一个 Worker 实例
	err := rpc.Register(worker) // 注册 Worker 到 RPC 服务器
	if err != nil {
		fmt.Println("Error registering RPC server:", err)
		return
	}

	// 使用 net.Listen 创建一个 TCP 监听器，监听指定的地址和端口
	listener, err := net.Listen("tcp", *pAddr)
	if err != nil {
		fmt.Println("Error starting listener:", err)
		return
	}
	defer listener.Close() // 程序退出时关闭监听器

	fmt.Println("Worker listening on " + *pAddr) // 输出监听地址

	// 无限循环，等待并处理传入的连接
	for {
		conn, err := listener.Accept() // 接受传入的连接
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue // 出现错误时，继续接受下一个连接
		}
		go rpc.ServeConn(conn) // 启动新的 goroutine 来处理该连接的 RPC 请求
	}
}

// HandleNextState 处理世界状态的计算，计算下一个状态。
// 输入：
//   - request stubs.Request：请求对象，包含当前世界状态。
//   - response *stubs.Response：响应对象，用于返回计算结果。
// 输出：
//   - 返回 error，如果没有错误，返回 nil；如果有错误，返回错误信息。
// 处理的步骤：
// 1. 根据当前世界状态计算下一个状态。
// 2. 将计算结果（下一个世界状态）设置到 response 中。
func (w *Worker) HandleNextState(request stubs.Request, response *stubs.Response) error {
	// 调用 calculateNextWorld 函数来计算下一个世界状态
	nextWorld := calculateNextWorld(request.World)

	// 设置响应状态为 "OK"
	response.Status = "OK"
	// 设置响应的世界状态为计算得到的下一个世界状态
	response.World = nextWorld
	return nil // 返回 nil，表示没有错误
}

// calculateNextWorld 生成当前世界的下一个状态，基于康威的生命游戏规则。
// 输入：
//   - currentWorld [][]uint8：当前世界的二维切片，表示细胞的状态。
// 输出：
//   - [][]uint8：表示下一个世界状态的二维切片。
// 处理步骤：
// 1. 遍历当前世界中的每个细胞，计算每个细胞的邻居数量。
// 2. 根据邻居数量应用生命游戏的规则，计算每个细胞的下一状态。
func calculateNextWorld(currentWorld [][]uint8) [][]uint8 {
	height := len(currentWorld)          // 获取世界的行数
	width := len(currentWorld[0])        // 获取世界的列数
	nextWorld := make([][]uint8, height) // 创建一个新的二维切片，用于存储下一个世界的状态

	for i := range currentWorld { // 遍历当前世界的每一行
		nextWorld[i] = make([]uint8, width) // 为每一行分配列数
		for j := range currentWorld[i] {    // 遍历每一行中的每个细胞
			// 调用 countLiveNeighbours 函数来计算当前细胞周围活细胞的数量
			liveNeighbours := countLiveNeighbours(i, j, currentWorld)
			// 根据规则应用 applyRules 函数，计算当前细胞的下一个状态
			nextWorld[i][j] = applyRules(currentWorld[i][j], liveNeighbours)
		}
	}
	return nextWorld // 返回计算后的下一个世界状态
}

// applyRules 根据生命游戏的规则，决定细胞的下一状态。
// 输入：
//   - cell uint8：当前细胞的状态（0 表示死亡，255 表示存活）
//   - liveNeighbours int：当前细胞周围活细胞的数量。
// 输出：
//   - uint8：细胞的下一状态。
// 处理步骤：
// 1. 如果细胞当前是活的（255），并且周围活细胞少于 2 或多于 3，则该细胞死亡。
// 2. 如果细胞当前是死的（0），并且周围有 3 个活细胞，则该细胞复活。
// 3. 否则，细胞保持当前状态。
func applyRules(cell uint8, liveNeighbours int) uint8 {
	if cell == 255 && (liveNeighbours < 2 || liveNeighbours > 3) {
		return 0 // 细胞死亡
	}
	if cell == 0 && liveNeighbours == 3 {
		return 255 // 细胞复活
	}
	return cell // 细胞保持当前状态
}

// countLiveNeighbours 计算指定细胞周围活细胞的数量。
// 输入：
//   - i int：细胞所在行的索引。
//   - j int：细胞所在列的索引。
//   - world [][]uint8：当前世界的二维切片，表示细胞的状态。
// 输出：
//   - int：周围活细胞的数量。
// 处理步骤：
// 1. 使用 8 个偏移量表示周围 8 个邻居的相对位置。
// 2. 遍历这些邻居位置，检查每个邻居是否是活细胞（值为 255），并计算活细胞的数量。
// 3. 使用模运算确保邻居索引不越界，保持环绕边界。
func countLiveNeighbours(i, j int, world [][]uint8) int {
	// 定义 8 个邻居的相对坐标偏移量
	neighborOffsets := [8][2]int{
		{-1, -1}, {-1, 0}, {-1, 1},
		{0, -1}, {0, 1},
		{1, -1}, {1, 0}, {1, 1},
	}
	height := len(world)   // 获取世界的行数
	width := len(world[0]) // 获取世界的列数
	liveNeighbours := 0    // 计数器，统计活细胞的数量

	// 遍历 8 个邻居的位置
	for _, offset := range neighborOffsets {
		// 计算邻居的实际坐标，并使用模运算确保索引不会越界
		ni := (i + offset[0] + height) % height
		nj := (j + offset[1] + width) % width
		if world[ni][nj] == 255 { // 如果邻居是活细胞
			liveNeighbours++ // 活细胞计数加 1
		}
	}
	return liveNeighbours // 返回活细胞数量
}

// CloseNode 关闭当前工作节点，退出程序。
// 输入：
//   - request stubs.BlankRequest：空请求对象。
//   - response *stubs.Response：响应对象。
// 输出：
//   - 返回 nil。
// 处理步骤：
// 1. 输出 "Closing node..."，表示节点正在关闭。
// 2. 调用 os.Exit(0) 退出程序。
func (w *Worker) CloseNode(request stubs.BlankRequest, response *stubs.Response) error {
	fmt.Println("Closing node...") // 输出关闭信息
	os.Exit(0)                     // 退出程序
	return nil                     // 返回 nil，表示操作成功
}
