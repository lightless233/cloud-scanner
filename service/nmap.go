package service

import (
	"bufio"
	"bytes"
	"cloud-scanner/config/constant"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

type NmapEngine struct {
	// 引擎状态
	Status []constant.EngineStatus

	// 存放主线程的 waitGroup
	mainWaitGroup *sync.WaitGroup

	// 引擎自己的 wait group
	waitGroup *sync.WaitGroup

	// 接受扫描任务的队列
	nmapJobChan *chan NmapJob

	// 存放扫描结果的队列
	// TODO 修改类型
	saverJobChan *chan PortResult
}

// NewNmapEngine 创建新的NmapEngine
func NewNmapEngine(mainWaitGroup *sync.WaitGroup, nmapJobChan *chan NmapJob, saverJobChan *chan PortResult) *NmapEngine {
	status := make([]constant.EngineStatus, appConfig.NmapWorkerCount)
	for i := range status {
		status[i] = constant.EngineInit
	}

	var wg sync.WaitGroup
	return &NmapEngine{
		mainWaitGroup: mainWaitGroup,
		Status:        status,
		nmapJobChan:   nmapJobChan,
		saverJobChan:  saverJobChan,
		waitGroup:     &wg,
	}
}

// Run 启动 NmapEngine
func (engine *NmapEngine) Run() {
	defer func() {
		engine.mainWaitGroup.Done()
		close(*engine.saverJobChan)
	}()

	// 创建引擎自己的 waitGroup
	var i uint = 0
	for ; i < appConfig.NmapWorkerCount; i++ {
		engine.waitGroup.Add(1)
		go engine.worker(i)
		engine.Status[i] = constant.EngineRunning
	}

	// 等待所有协程
	engine.waitGroup.Wait()
	logger.Infof("NmapEngine exit.")
}

// worker 引擎的真正工作函数
func (engine *NmapEngine) worker(idx uint) {
	defer engine.waitGroup.Done()
	tag := fmt.Sprintf("[NmapEngine-%d]", idx)
	logger.Debugf("%s worker start.", tag)
	for {
		// 当任务队列关闭了之后，退出
		// TODO 是否需要继续检查一遍 masscan engine 的状态？
		task, opened := <-*engine.nmapJobChan
		// logger.Debugf("%s received from nmapJobChan, task: %+v, opened: %+v", tag, task, opened)
		if !opened {
			break
		}
		logger.Debugf("%s Get task %+v", tag, task)

		// 跳过没有端口开放的 IP
		if len(task.value) == 0 {
			continue
		}
		host := task.value[0].Host

		// 生成临时文件名字
		tmpOutFile := fmt.Sprintf("./%s/nmap_%s", constant.TempDir, task.UUID)

		// 构造 port 参数
		tmpPorts := make([]string, 0, len(task.value))
		for _, mr := range task.value {
			tmpPorts = append(tmpPorts, strconv.Itoa(int(mr.Port)))
		}

		// 构造 nmap cmd
		cmd := exec.Command(
			"nmap", host, "-T5", "-sV", "-p", strings.Join(tmpPorts, ","), "-oG", tmpOutFile,
		)
		logger.Debugf("%s cmd: %s", tag, cmd.String())
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		strOut, strErr := string(stdout.Bytes()), string(stderr.Bytes())
		if err != nil {
			logger.Errorf("%s error when exec cmd, error: %+v\nstdout: %s\nstderr: %s", tag, err, strOut, strErr)
			continue
		}
		if appConfig.Debug {
			logger.Debugf("%s stdout: %s\nstderr: %s", tag, strOut, strErr)
		}

		// 解析 nmap 扫描结果
		// # Nmap 7.80 scan initiated Sun Dec  3 15:49:03 2023 as: nmap -sV -p10022,80,12022 -oG=/tmp/111.txt --open 45.159.49.184
		// Host: 45.159.49.184 ()  Status: Up
		// Host: 45.159.49.184 ()  Ports: 80/open/tcp//http//nginx 1.24.0/, 10022/open/tcp//ssh//OpenSSH 9.5p1 Debian 2 (protocol 2.0)/, 12022/open/tcp//ssl|unknown///
		// # Nmap done at Sun Dec  3 15:50:44 2023 -- 1 IP address (1 host up) scanned in 101.25 seconds
		fp, _ := os.Open(tmpOutFile)
		reader := bufio.NewReader(fp)
		for {
			line, err := reader.ReadString('\n')
			if err == io.EOF {
				break
			} else if err != nil {
				logger.Warnf("%s Error when reading nmap temp result file %s, err: %+v", tag, tmpOutFile, err)
				continue
			}
			line = strings.TrimSpace(line)

			// 跳过空行或者井号开头的行
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			// 检查是否为 port 行数据
			if !strings.Contains(line, "Ports:") {
				continue
			}

			lineParts := strings.Split(line, "Ports:")
			portsRawList := strings.Split(lineParts[1], ",")
			for _, rawItem := range portsRawList {
				rawItem = strings.TrimSpace(rawItem)
				itemPart := strings.Split(rawItem, "/")
				port, _ := strconv.ParseUint(itemPart[0], 10, 32)
				protocol := itemPart[2]
				service := itemPart[4]
				banner := itemPart[6]

				portResult := PortResult{
					Host:     host,
					Port:     uint(port),
					Protocol: protocol,
					Service:  service,
					Banner:   banner,
				}

				*engine.saverJobChan <- portResult
				logger.Debugf("%s Put port result `%+v` to channel.", tag, portResult)
			}
		}

		// 如果开了 debug 选项，则不删除中间文件
		if !appConfig.Debug {
			if err := os.Remove(tmpOutFile); err != nil {
				logger.Warnf("%s Error when delete masscan output file. filename: %s, error: %+v", tag, tmpOutFile, err)
			}
		}

	}

	logger.Debugf("%s worker stop.", tag)
}
