package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"

	"distributed/stubs"
)

type Worker struct{}

func main() {
	// Set up the worker to listen on a specified port
	pAddr := flag.String("port", "127.0.0.1:8085", "IP and port to listen on")
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
func (w *Worker) HandleNextState(request stubs.Request, response *stubs.Response) error {
	nextWorld := calculateNextWorld(request.World)
	response.Status = "OK"
	response.World = nextWorld
	return nil
}

// calculateNextWorld generates the next state of the world based on Conway's Game of Life rules.
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
	neighborOffsets := [8][2]int{
		{-1, -1}, {-1, 0}, {-1, 1},
		{0, -1}, {0, 1},
		{1, -1}, {1, 0}, {1, 1},
	}

	height := len(world)
	width := len(world[0])
	liveNeighbours := 0

	for _, offset := range neighborOffsets {
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
