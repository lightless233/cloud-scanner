package service

import (
	"bufio"
	"bytes"
	"cloud-scanner/config/constant"
	"fmt"
	"github.com/google/uuid"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

type MasscanEngine struct {
	// 引擎状态
	Status []constant.EngineStatus

	// 存放主线程的 wg
	mainWaitGroup *sync.WaitGroup

	// 接受扫描任务的队列
	masscanJobChan *chan string

	// 存放扫描结果的队列
	nmapJobChan *chan NmapJob
}

// NewMasscanEngine 创建新的 MasscanEngine
func NewMasscanEngine(mainWaitGroup *sync.WaitGroup, masscanJobChan *chan string, nmapJobChan *chan NmapJob) *MasscanEngine {
	status := make([]constant.EngineStatus, appConfig.MasscanWorkerCount)
	for i := range status {
		status[i] = constant.EngineInit
	}
	return &MasscanEngine{
		mainWaitGroup:  mainWaitGroup,
		Status:         status,
		masscanJobChan: masscanJobChan,
		nmapJobChan:    nmapJobChan,
	}
}

// Run 启动 Masscan Engine
func (engine *MasscanEngine) Run() {

	defer func() {
		engine.mainWaitGroup.Done()
		close(*engine.nmapJobChan)
	}()

	// 引擎自己控制自己协程组的 waitGroup
	var waitGroup sync.WaitGroup
	var i uint = 0
	for ; i < appConfig.MasscanWorkerCount; i++ {
		waitGroup.Add(1)
		go engine.worker(i, &waitGroup)
	}

	// 等待所有的 worker 运行完成
	waitGroup.Wait()
	logger.Infof("MasscanEngine exit.")
}

func (engine *MasscanEngine) worker(idx uint, wg *sync.WaitGroup) {
	// 从 masscanChan 中获取任务，当 chan 关闭了之后，就结束 worker
	defer func() {
		wg.Done()
	}()
	logger.Debugf("[MasscanEngine-%d] worker start.", idx)

	for {
		task, opened := <-*engine.masscanJobChan
		// logger.Debugf("[ME-%d] task:%+v, opened:%+v", idx, task, opened)
		if !opened {
			break
		}

		logger.Infof("[MasscanEngine-%d] Get ip: %s", idx, task)

		// tmpOutFile 放到单独的文件夹中
		randomUUID := uuid.NewString()
		tmpOutFile := fmt.Sprintf("./%s/masscan_%s", constant.TempDir, randomUUID)
		cmd := exec.Command(
			"masscan", task, fmt.Sprintf("--rate=%d", appConfig.MasscanRate), "-p-", "-oL", tmpOutFile,
		)
		logger.Debugf("[MasscanEngine-%d] CMD: %s", idx, cmd.String())
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			logger.Errorf("[MasscanEngine-%d] Error when exec cmd, error: %+v, stdout: %+v, stderr: %+v", idx, err, string(stdout.Bytes()), string(stderr.Bytes()))
			continue
		}

		if appConfig.Debug {
			logger.Debugf("[MasscanEngine-%d] stdout: %s", idx, string(stdout.Bytes()))
			logger.Debugf("[MasscanEngine-%d] stderr: %s", idx, string(stderr.Bytes()))
		}

		// 读取 masscan 的输出，解析出端口信息
		// #masscan
		// open tcp 80 1.1.1.1 1701436172
		// # end
		fp, _ := os.Open(tmpOutFile)
		reader := bufio.NewReader(fp)
		tmpResult := make([]MasscanResult, 0)
		for {
			line, err := reader.ReadString('\n')
			if err == io.EOF {
				break
			} else if err != nil {
				logger.Warnf("[MasscanEngine-%d] Error when reading masscan temp result file %s, err: %+v", idx, tmpOutFile, err)
				continue
			}
			line = strings.TrimSpace(line)

			// 跳过空行或者井号开头的行
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			// 按照空格切分，取出数据
			lineParts := strings.Split(line, " ")
			if len(lineParts) < 5 {
				logger.Warnf("[MasscanEngine-%d] Error when split line: %s", idx, line)
				continue
			}

			port, _ := strconv.ParseUint(lineParts[2], 10, 32)
			r := MasscanResult{
				Host:     lineParts[3],
				Protocol: lineParts[1],
				Port:     uint(port),
			}
			tmpResult = append(tmpResult, r)
		}

		// TODO 这里未来加个选项，判断是否单独保存 masscan 的结构化扫描结果
		// 构造 nmap job
		nmapJob := NmapJob{
			value: tmpResult,
			UUID:  randomUUID,
		}
		// 添加到下一个任务队列中
		*engine.nmapJobChan <- nmapJob
		logger.Debugf("[MasscanEngine-%d] Put task %+v to nmap channel", idx, nmapJob)

		// 如果开了 debug 选项，则不删除中间文件
		if !appConfig.Debug {
			if err := os.Remove(tmpOutFile); err != nil {
				logger.Warnf("[MasscanEngine-%d] Error when delete masscan output file. filename: %s, error: %+v", idx, tmpOutFile, err)
			}
		}
	}

	logger.Debugf("[MasscanEngine-%d] worker stop.", idx)
}
