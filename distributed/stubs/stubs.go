package stubs

var BrokerHandler = "Broker.HandleBroker"
var GetCurrentState = "Broker.GetCurrentState"
var HandleKey = "Broker.HandleKey"

var HandleWorker = "Worker.HandleNextState"
var CloseNode = "Worker.CloseNode"

var HandleFlipCells = "Distributor.HandleFlipCells"

type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type Request struct {
	Message string
	World   [][]uint8
	Params  Params
}

type Response struct {
	Message string
	Status  string
	World   [][]uint8
}

type BlankRequest struct{}

type CurrentStateResponse struct {
	AliveCellsCount int
	CurrentWorld    [][]uint8
	Turn            int
}

type KeyRequest struct {
	Key string
}

type FlipRequest struct {
	OldWorld [][]uint8
	NewWorld [][]uint8
	Turn     int
}
