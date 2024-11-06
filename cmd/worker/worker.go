package main

import (
	"fmt"
	"net/rpc"
	"os"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
)

type AliveCount struct {
	mu     sync.Mutex
	Count  map[int]int
	Latest int
}

type LatestWorld struct {
	mu     sync.Mutex
	World  map[int][][]uint8
	Latest int
}

var countMap = make(map[int]int)
var aliveCount AliveCount = AliveCount{Count: countMap, Latest: 0}
var worldMap = make(map[int][][]uint8)
var latestWorld LatestWorld = LatestWorld{World: worldMap, Latest: 0}
var QuitFlag, PauseFlag bool = false, false

type GolWorkerService struct {
	NeighborTop    string
	NeighborBottom string
	TopHaloChan    chan stubs.HaloRequest
	BottomHaloChan chan stubs.HaloRequest
}

func (s *GolWorkerService) WorkerKeyWorld(req stubs.WorkerKeyRequest, res *stubs.WorkerKeyResponse) error {
	if req.Key == 's' {
		fmt.Println("SSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSSS")
		latestWorld.mu.Lock()
		res.World = latestWorld.World
		res.Latest = latestWorld.Latest
		latestWorld.mu.Unlock()
		return nil
	}
	if req.Key == 'q' { // Resetting state of server
		fmt.Println("QQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQ")
		QuitFlag = true
		latestWorld.mu.Lock()
		res.World = latestWorld.World
		res.Latest = latestWorld.Latest
		latestWorld.mu.Unlock()
		return nil
	}
	if req.Key == 'k' {
		fmt.Println("KKKKKKKKKKKKKKKKKKKKKKKKKKKKKKK")
		QuitFlag = true
		latestWorld.mu.Lock()
		res.World = latestWorld.World
		res.Latest = latestWorld.Latest
		latestWorld.mu.Unlock()
		os.Exit(0)
	}
	if req.Key == 'p' {
		fmt.Println("PPPPPPPPPPPPPPPPPPPPPPPPPPPPPP")
		PauseFlag = !PauseFlag
		latestWorld.mu.Lock()
		res.World = latestWorld.World
		res.Latest = latestWorld.Latest
		latestWorld.mu.Unlock()
		return nil
	}

	return nil
}

func (s *GolWorkerService) GetWorkerAliveCells(req stubs.AliveRequest, res *stubs.AliveResponse) error {
	// 模拟存活单元计数
	aliveCount.mu.Lock()
	res.CountMap = aliveCount.Count
	res.Latest = aliveCount.Latest
	aliveCount.mu.Unlock()
	return nil
}

// 接收光环数据的 RPC 方法，放入对应通道
func (s *GolWorkerService) ReceiveTopHalo(req stubs.HaloRequest, res *stubs.HaloResponse) error {
	fmt.Printf("time:%v; recive top halo; turn:%d\n", time.Now(), req.Iteration)
	s.TopHaloChan <- req
	*res = stubs.HaloResponse{Success: true}
	return nil
}

func (s *GolWorkerService) ReceiveBottomHalo(req stubs.HaloRequest, res *stubs.HaloResponse) error {
	fmt.Printf("time:%v; recive bot halo; turn:%d\n", time.Now(), req.Iteration)
	s.BottomHaloChan <- req
	*res = stubs.HaloResponse{Success: true}
	return nil
}

func (s *GolWorkerService) WorkerComputeGrid(req stubs.ComputeGridRequest, res *stubs.ComputeGridResponse) error {
	fmt.Println("recive 1234")
	QuitFlag, PauseFlag = false, false
	iter := 0
	grid := req.SubGrid
	topHalo := req.HaloTop
	bottomHalo := req.HaloBottom

	for iter < req.Iterations {
		if QuitFlag {
			return nil
		}

		if PauseFlag {
			continue
		}

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

		aliveCount.mu.Lock()
		aliveCount.Count[iter] = countAliveCells(newSubGrid)
		aliveCount.Latest = iter
		aliveCount.mu.Unlock()

		latestWorld.mu.Lock()
		latestWorld.World[iter] = grid
		latestWorld.Latest = iter
		latestWorld.mu.Unlock()

		// 发送当前迭代的光环数据到邻居节点
		go s.sendHalo(grid[0], iter, s.NeighborTop, true)
		go s.sendHalo(grid[len(grid)-1], iter, s.NeighborBottom, false)

		// 阻塞，等待接收来自邻居的光环数据
		topHalo = s.receiveHalo(iter, true)
		bottomHalo = s.receiveHalo(iter, false)
		fmt.Printf("time:%v; turn: %d\n", time.Now(), iter)
	}

	res.GridPart = grid
	return nil
}

// 接收特定迭代的光环数据
func (s *GolWorkerService) receiveHalo(iter int, isTop bool) []uint8 {
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

// sendHalo 发送光环数据，并在 HaloRequest 中附加当前的迭代步数。
func (s *GolWorkerService) sendHalo(haloData []uint8, iter int, neighborAddr string, isTop bool) error {
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

	request := stubs.HaloRequest{HaloData: haloData, Iteration: iter}
	response := new(stubs.HaloResponse)
	if !isTop {
		err = client.Call(stubs.GoLReceiveTopHaloHandler, request, response)
	} else {
		err = client.Call(stubs.GoLReceiveBottomHaloHandler, request, response)
	}
	if err != nil || !response.Success {
		fmt.Printf("Error sending halo to %s for iter %d: %v\n", neighborAddr, iter, err)
	}
	return err
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
