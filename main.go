package main

import (
	"cloud-scanner/cmd"
	"cloud-scanner/logging"
	"os"
)

func main() {

	if err := cmd.RunApp(); err != nil {
		logger := logging.GetSugar()
		logger.Errorf("Error when run app. Error: %+v", err)
		os.Exit(1)
	}

}
