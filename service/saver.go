package service

import (
	"bufio"
	"cloud-scanner/config/constant"
	"fmt"
	"os"
	"sync"
)

type SaverEngine struct {
	// 引擎状态
	Status constant.EngineStatus

	// 存放主线程的 wait group
	mainWaitGroup *sync.WaitGroup

	// 存放自己的 wait group
	waitGroup *sync.WaitGroup

	// 接受任务的队列
	saverJobChan *chan PortResult
}

// NewSaverEngine 创建一个新的 SaverEngine
func NewSaverEngine(mainWaitGroup *sync.WaitGroup, saverJobChan *chan PortResult) *SaverEngine {
	var waitGroup sync.WaitGroup
	return &SaverEngine{
		Status:        constant.EngineInit,
		mainWaitGroup: mainWaitGroup,
		waitGroup:     &waitGroup,
		saverJobChan:  saverJobChan,
	}
}

// Run 启动 SaverEngine
func (engine *SaverEngine) Run() {
	defer func() {
		engine.mainWaitGroup.Done()
		// close(*engine.saverJobChan)
	}()

	engine.waitGroup.Add(1)
	go engine.worker()
	engine.waitGroup.Wait()

	logger.Debugf("SaverEngine exit.")
}

// worker 真正的工作函数
func (engine *SaverEngine) worker() {
	defer engine.waitGroup.Done()

	tag := "[SaverEngine]"
	logger.Debugf("%s worker start.", tag)

	// output filename
	fp, err := os.OpenFile(appConfig.OutputFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		logger.Fatalf("Cannot open output file to write: %s， error: %+v", appConfig.OutputFile, err)
		os.Exit(1)
	}
	writer := bufio.NewWriter(fp)
	defer func() {
		_ = fp.Close()
	}()

	for {
		task, opened := <-*engine.saverJobChan
		if !opened {
			break
		}
		logger.Debugf("%s Get task %+v", tag, task)

		//
		line := fmt.Sprintf("%s, %s, %d, %s, %s\n", task.Host, task.Protocol, task.Port, task.Service, task.Banner)
		_, _ = writer.WriteString(line)
		_ = writer.Flush()
	}
	logger.Debugf("%s worker stop.", tag)
}
