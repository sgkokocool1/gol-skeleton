package main

import (
	"fmt"
	"net/rpc"
	"time"

	"uk.ac.bris.cs/gameoflife/gol"
)

type AliveCount struct {
	Count  map[int]int
	Latest int
}

var countMap = make(map[int]int)
var aliveCount AliveCount = AliveCount{Count: countMap, Latest: 0}

// 光环数据的 RPC 请求和响应结构
type HaloRequest struct {
	HaloData  []uint8
	Iteration int
}
type HaloResponse struct {
	Success bool
}

type GolService struct {
	NeighborTop    string
	NeighborBottom string
	TopHaloChan    chan HaloRequest
	BottomHaloChan chan HaloRequest
}

func (s *GolService) GetAliveCells(req *gol.AliveRequest, res *gol.AliveResponse) error {
	// 模拟存活单元计数
	res.CountMap = aliveCount.Count
	res.Latest = aliveCount.Latest
	return nil
}

// sendHalo 发送光环数据，并在 HaloRequest 中附加当前的迭代步数。
func (s *GolService) sendHalo(haloData []uint8, iter int, neighborAddr string, isTop bool) error {
	fmt.Printf("time:%v; neighborAddr:%s; istop:%v; turn: %d\n", time.Now(), neighborAddr, isTop, iter)
	if neighborAddr == "" {
		return nil
	}

	client, err := rpc.Dial("tcp", neighborAddr)
	if err != nil {
		fmt.Println("Error connecting to neighbor:", err)
		return err
	}
	defer client.Close()

	request := HaloRequest{HaloData: haloData, Iteration: iter}
	var response HaloResponse
	if !isTop {
		err = client.Call("GolService.ReceiveTopHalo", request, &response)
	} else {
		err = client.Call("GolService.ReceiveBottomHalo", request, &response)
	}
	if err != nil || !response.Success {
		fmt.Printf("Error sending halo to %s for iter %d: %v\n", neighborAddr, iter, err)
	}
	return err
}

// 接收光环数据的 RPC 方法，放入对应通道
func (s *GolService) ReceiveTopHalo(req HaloRequest, res *HaloResponse) error {
	fmt.Printf("time:%v; recive top halo; turn:%d\n", time.Now(), req.Iteration)
	s.TopHaloChan <- req
	*res = HaloResponse{Success: true}
	return nil
}

func (s *GolService) ReceiveBottomHalo(req HaloRequest, res *HaloResponse) error {
	fmt.Printf("time:%v; recive bot halo; turn:%d\n", time.Now(), req.Iteration)
	s.BottomHaloChan <- req
	*res = HaloResponse{Success: true}
	return nil
}

// 接收特定迭代的光环数据
func (s *GolService) receiveHalo(iter int, isTop bool) []uint8 {
	fmt.Printf("time:%v; istop:%v; turn: %d\n", time.Now(), isTop, iter)

	if isTop {

		for req := range s.TopHaloChan {
			fmt.Printf("time:%v; recive top turn: %d\n", time.Now(), req.Iteration)
			if req.Iteration == iter {
				return req.HaloData
			}
		}
	} else {

		for req := range s.BottomHaloChan {
			fmt.Printf("time:%v; recive bot turn: %d\n", time.Now(), req.Iteration)
			if req.Iteration == iter {
				return req.HaloData
			}
		}
	}

	return []uint8{}
}

func (s *GolService) ComputeGrid(req gol.GridRequest, res *gol.GridResponse) error {
	fmt.Println("recive 1234")
	iter := 0
	grid := req.SubGrid
	topHalo := req.HaloTop
	bottomHalo := req.HaloBottom

	for iter < req.Iterations {
		iter++

		// 迭代并更新子网格
		newSubGrid := make([][]uint8, len(grid))
		for i := range newSubGrid {
			newSubGrid[i] = make([]uint8, len(grid[i]))
		}
		for i := 0; i < len(grid); i++ {
			for j := 0; j < len(grid[i]); j++ {
				aliveNeighbors := countAliveNeighbors(grid, i, j, topHalo, bottomHalo)
				if grid[i][j] == 255 {
					if aliveNeighbors < 2 || aliveNeighbors > 3 {
						newSubGrid[i][j] = 0
					} else {
						newSubGrid[i][j] = 255
					}
				} else if aliveNeighbors == 3 {
					newSubGrid[i][j] = 255
				}
			}
		}
		grid = newSubGrid
		aliveCount.Count[iter] = countAliveCells(newSubGrid)
		aliveCount.Latest = iter

		// 发送当前迭代的光环数据到邻居节点
		go s.sendHalo(grid[0], iter, s.NeighborTop, true)
		go s.sendHalo(grid[len(grid)-1], iter, s.NeighborBottom, false)

		// 阻塞，等待接收来自邻居的光环数据
		topHalo = s.receiveHalo(iter, true)
		bottomHalo = s.receiveHalo(iter, false)
		fmt.Printf("time:%v; top:%v; bot:%v; turn: %d\n", time.Now(), topHalo, bottomHalo, iter)
	}

	res.GridPart = grid
	return nil
}

func countAliveCells(world [][]uint8) int {
	alive := 0
	for i := 0; i < len(world); i++ {
		for j := 0; j < len(world[i]); j++ {
			if world[i][j] == 255 {
				alive++
			}
		}
	}
	return alive
}

// 计算活邻居数量的辅助函数
func countAliveNeighbors(grid [][]uint8, x, y int, haloTop, haloBottom []uint8) int {
	rows := len(grid)
	cols := len(grid[0])
	count := 0
	dirs := [][2]int{
		{-1, -1}, {-1, 0}, {-1, 1}, // 上行
		{0, -1}, {0, 1}, // 当前行
		{1, -1}, {1, 0}, {1, 1}, // 下行
	}

	for _, dir := range dirs {
		nx := x + dir[0]
		ny := y + dir[1]

		// 水平（左右）处理：循环列边界
		if ny < 0 {
			ny = cols - 1 // 左侧溢出，移动到右侧
		} else if ny >= cols {
			ny = 0 // 右侧溢出，移动到左侧
		}

		// 垂直（上下）处理
		if nx < 0 { // 上行，使用 haloTop
			if haloTop[ny] == 255 {
				count++
			}
		} else if nx >= rows { // 下行，使用 haloBottom
			if haloBottom[ny] == 255 {
				count++
			}
		} else if grid[nx][ny] == 255 { // 正常范围内的邻居
			count++
		}
	}

	return count
}
