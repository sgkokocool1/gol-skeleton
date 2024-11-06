package main

// worker/worker.go

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/stubs"
)

type Params struct {
	NeighborTop    string
	NeighborBottom string
	TopHaloChan    chan stubs.HaloRequest
	BottomHaloChan chan stubs.HaloRequest
}

func main() {
	var params Params

	flag.StringVar(
		&params.NeighborTop,
		"t",
		"127.0.0.1:1234",
		"Specify the NeighborTop of worker . Defaults to null.")

	flag.StringVar(
		&params.NeighborBottom,
		"b",
		"127.0.0.1:1234",
		// "",
		"Specify the NeighborBottom of worker . Defaults to 1235.")

	flag.Parse()

	// var topMap, bottomMap sync.Map
	golService := &GolWorkerService{
		NeighborTop:    params.NeighborTop,
		NeighborBottom: params.NeighborBottom,
		TopHaloChan:    make(chan stubs.HaloRequest),
		BottomHaloChan: make(chan stubs.HaloRequest),
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
