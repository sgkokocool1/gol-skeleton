package main

// worker/worker.go

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"sync"
)

type Params struct {
	NeighborTop    string
	NeighborBottom string
	TopHaloChan    chan HaloRequest
	BottomHaloChan chan HaloRequest
}

func main() {
	var params Params

	flag.StringVar(
		&params.NeighborTop,
		"t",
		"",
		"Specify the NeighborTop of worker . Defaults to null.")

	flag.StringVar(
		&params.NeighborBottom,
		"b",
		"127.0.0.1:1235",
		"Specify the NeighborBottom of worker . Defaults to 1235.")

	flag.Parse()

	var topMap, bottomMap sync.Map
	golService := &GolService{
		NeighborTop:    params.NeighborTop,
		NeighborBottom: params.NeighborBottom,
		// TopHaloChan:    make(chan HaloRequest),
		// BottomHaloChan: make(chan HaloRequest),
		TopMap:    topMap,
		BottomMap: bottomMap,
	}

	rpc.Register(golService)
	listener, err := net.Listen("tcp", ":1234")
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	fmt.Println("Listen tcp 1234")
	rpc.Accept(listener)
}
