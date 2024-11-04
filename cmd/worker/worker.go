package main

import (
	"fmt"
	"net/rpc"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/gol"
)

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
	TopMap         sync.Map
	BottomMap      sync.Map
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

	s.TopMap.Store(req.Iteration, req.HaloData)
	//s.TopHaloChan <- req
	*res = HaloResponse{Success: true}
	return nil
}

func (s *GolService) ReceiveBottomHalo(req HaloRequest, res *HaloResponse) error {
	fmt.Printf("time:%v; recive bot halo; turn:%d\n", time.Now(), req.Iteration)
	//s.BottomHaloChan <- req
	s.BottomMap.Store(req.Iteration, req.HaloData)
	*res = HaloResponse{Success: true}
	return nil
}

// 接收特定迭代的光环数据
func (s *GolService) receiveHalo(iter int, isTop bool) ([]uint8, bool) {
	fmt.Printf("time:%v; istop:%v; turn: %d\n", time.Now(), isTop, iter)

	if isTop {

		val, ok := s.TopMap.Load(iter)
		if ok {
			return val.([]uint8), true
		}

		return []uint8{}, false
		// for req := range s.TopHaloChan {
		// 	fmt.Printf("time:%v; recive top turn: %d\n", time.Now(), req.Iteration)
		// 	if req.Iteration == iter {
		// 		return req.HaloData
		// 	}
		// }
	} else {

		val, ok := s.BottomMap.Load(iter)
		if ok {
			return val.([]uint8), true
		}
		return []uint8{}, false
		// for req := range s.BottomHaloChan {
		// 	fmt.Printf("time:%v; recive bot turn: %d\n", time.Now(), req.Iteration)
		// 	if req.Iteration == iter {
		// 		return req.HaloData
		// 	}
		// }
	}
}

func (s *GolService) ComputeGrid(req gol.GridRequest, res *gol.GridResponse) error {
	fmt.Println("recive 1234")
	subGrid := req.SubGrid
	topHalo := req.HaloTop
	bottomHalo := req.HaloBottom

	for iter := 0; iter < req.Iterations; iter++ {
		// 迭代并更新子网格
		newSubGrid := make([][]uint8, len(subGrid))
		for i := range newSubGrid {
			newSubGrid[i] = make([]uint8, len(subGrid[i]))
		}
		for i := 0; i < len(subGrid); i++ {
			for j := 0; j < len(subGrid[i]); j++ {
				aliveNeighbors := countAliveNeighbors(subGrid, i, j, topHalo, bottomHalo)
				if subGrid[i][j] == 255 {
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
		subGrid = newSubGrid

		// 发送当前迭代的光环数据到邻居节点
		go s.sendHalo(subGrid[0], iter, s.NeighborTop, true)
		go s.sendHalo(subGrid[len(subGrid)-1], iter, s.NeighborBottom, false)

		// 阻塞，等待接收来自邻居的光环数据
		if !req.Istop {
			for i := 0; i < 5; i++ {
				v, ok := s.receiveHalo(iter, true)
				if ok {
					topHalo = v
				} else {
					time.Sleep(1 * time.Second)
				}
			}
		}

		if !req.Isbottom {
			for i := 0; i < 5; i++ {
				v, ok := s.receiveHalo(iter, false)
				if ok {
					bottomHalo = v
				} else {
					time.Sleep(1 * time.Second)
				}
			}
		}

		fmt.Printf("time:%v; top:%v; bot:%v; turn: %d\n", time.Now(), topHalo, bottomHalo, iter)
	}

	res.GridPart = subGrid
	return nil
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
