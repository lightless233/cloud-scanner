package service

import (
	"bufio"
	"cloud-scanner/config/constant"
	"io"
	"net"
	"os"
	"strings"
	"sync"
)

// TaskBuilder 生产任务的引擎
type TaskBuilder struct {

	// 引擎状态
	Status constant.EngineStatus

	// 存放主线程的 wg
	mainWaitGroup *sync.WaitGroup

	// 生成好的 IP 任务放到这个 channel 中
	masscanJobChan *chan string
}

// NewTaskBuilder 构造一个新的 TaskBuilder
func NewTaskBuilder(mainWg *sync.WaitGroup, masscanJobChan *chan string) *TaskBuilder {
	return &TaskBuilder{
		mainWaitGroup:  mainWg,
		Status:         constant.EngineInit,
		masscanJobChan: masscanJobChan,
	}
}

// Run 启动 TaskBuilder 引擎
func (b *TaskBuilder) Run() {
	defer b.mainWaitGroup.Done()
	b.worker()
}

func (b *TaskBuilder) worker() {
	defer func() {
		logger.Debugf("TaskBuilder defer() called.")
		close(*b.masscanJobChan)
		b.Status = constant.EngineStop
	}()

	b.Status = constant.EngineRunning
	var successfulCount uint = 0

	if appConfig.Target != "" {
		// 把任务塞到队列里
		targets := strings.Split(appConfig.Target, ",")
		for _, target := range targets {
			target = strings.TrimSpace(target)
			*b.masscanJobChan <- target
			successfulCount += 1
		}
		logger.Infof("%d jobs were successfully added.", successfulCount)
	} else if appConfig.InputFile != "" {
		// 读文件
		fp, err := os.Open(appConfig.InputFile)
		if err != nil {
			logger.Errorf("Error when reading input file %s, error: %+v", appConfig.InputFile, err)
			return
		}
		defer func(fp *os.File) {
			_ = fp.Close()
		}(fp)

		bufferReader := bufio.NewReader(fp)
		for {
			line, err := bufferReader.ReadString('\n')
			if err == io.EOF {
				break
			} else if err != nil {
				logger.Errorf("Error when reading input file %s, error: %+v", appConfig.InputFile, err)
				continue
			}

			line = strings.TrimSpace(line)
			if net.ParseIP(line) == nil {
				logger.Errorf("Illegal IP address found: %s, skip it.", line)
				continue
			}
			// TODO 略过内网IP

			// 把任务塞到队列里
			*b.masscanJobChan <- line
			successfulCount += 1
		}

		logger.Infof("%d jobs were successfully added.", successfulCount)
	} else {
		// 输入有问题，结束
		logger.Error("appConfig.Target and appConfig.InputFile cannot be empty at the same time.")
	}

}
