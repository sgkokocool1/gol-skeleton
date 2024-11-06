package stubs

var GoLGetWorkerAliveCellsHandler = "GolWorkerService.GetWorkerAliveCells"
var GoLReceiveTopHaloHandler = "GolWorkerService.ReceiveTopHalo"
var GoLReceiveBottomHaloHandler = "GolWorkerService.ReceiveBottomHalo"
var GoLWorkerComputeGridHandler = "GolWorkerService.WorkerComputeGrid"
var GoLWorkerKeyHandler = "GolWorkerService.WorkerKeyWorld"

type ComputeGridRequest struct {
	HaloTop, HaloBottom []uint8
	Iterations          int
	SubGrid             [][]uint8
	StartRow, EndRow    int
}

type ComputeGridResponse struct {
	GridPart [][]uint8
}

type AliveRequest struct{}
type AliveResponse struct {
	CountMap map[int]int
	Latest   int
}

// 光环数据的 RPC 请求和响应结构
type HaloRequest struct {
	HaloData  []uint8
	Iteration int
}
type HaloResponse struct {
	Success bool
}

type WorkerKeyRequest struct {
	Key rune
}

type WorkerKeyResponse struct {
	World  map[int][][]uint8
	Latest int
}
