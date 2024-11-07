package main

import (
	"distributed/gol"
	"distributed/util"
	"os"
	"runtime/trace"
	"testing"
)

// TestTrace is a special test to be used to generate traces - not a real test
func TestTrace(t *testing.T) {
	traceParams := gol.Params{
		Turns:       10,
		Threads:     4,
		ImageWidth:  64,
		ImageHeight: 64,
	}
	f, _ := os.Create("trace.out")
	events := make(chan gol.Event)
	err := trace.Start(f)
	util.Check(err)
	go gol.Run(traceParams, events, nil)
	for range events {
	}
	trace.Stop()
	err = f.Close()
	util.Check(err)
}
