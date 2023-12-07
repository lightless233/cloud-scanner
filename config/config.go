package config

type AppConfig struct {
	Target    string
	InputFile string

	MasscanWorkerCount uint
	NmapWorkerCount    uint
	MasscanRate        uint

	OutputFile string

	Debug bool
}

var appConfig AppConfig

func GetAppConfig() *AppConfig {
	return &appConfig
}
