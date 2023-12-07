package constant

type EngineStatus int8

const (
	EngineInit    EngineStatus = 0
	EngineRunning EngineStatus = 1
	EngineStop    EngineStatus = 2
)

const TempDir string = "cloud_scanner_tmp"
