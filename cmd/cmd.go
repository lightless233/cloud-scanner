package cmd

import (
	"cloud-scanner/config"
	"cloud-scanner/config/constant"
	"cloud-scanner/logging"
	"cloud-scanner/service"
	"fmt"
	"github.com/urfave/cli/v2"
	"os"
	"runtime"
	"strings"
	"sync"
)

var logger = logging.GetSugar()
var appConfig = config.GetAppConfig()

func RunApp() error {
	app := &cli.App{
		Usage:   "Cloud Assets Scanner",
		Action:  MainAction,
		Version: "0.1.0",
		Flags: []cli.Flag{

			&cli.StringFlag{
				Name:        "target",
				Usage:       "Scan target IP, multiple IPs separated by commas",
				Destination: &appConfig.Target,
				Aliases:     []string{"t"},
			},

			&cli.StringFlag{
				Name:        "input",
				Usage:       "A file contains a list of IPs to be scanned, one line per IP",
				Destination: &appConfig.InputFile,
				Aliases:     []string{"i"},
			},

			&cli.UintFlag{
				Name:        "masscanWorkerCount",
				Usage:       "MASSCAN worker count",
				Destination: &appConfig.MasscanWorkerCount,
				Value:       uint(runtime.NumCPU()),
				DefaultText: "CPU Count",
				Aliases:     []string{"m"},
			},

			&cli.UintFlag{
				Name:        "nmapWorkerCount",
				Usage:       "NMAP worker count",
				Destination: &appConfig.NmapWorkerCount,
				Value:       2 * uint(runtime.NumCPU()),
				DefaultText: "2 * CPU Count",
				Aliases:     []string{"n"},
			},

			&cli.UintFlag{
				Name:        "masscanRate",
				Usage:       "Masscan scan rate",
				Value:       2000,
				Destination: &appConfig.MasscanRate,
				Aliases:     []string{"r"},
			},

			&cli.StringFlag{
				Name:        "output",
				Usage:       "Output filename",
				Aliases:     []string{"o"},
				Destination: &appConfig.OutputFile,
				DefaultText: "./<target>_out.txt",
			},

			&cli.BoolFlag{
				Name:        "debug",
				Usage:       "Debug mode",
				Value:       false,
				Destination: &appConfig.Debug,
			},
		},
		Before: func(context *cli.Context) error {
			// 初始化日志系统
			debug := context.Bool("debug")
			logging.InitLogger(debug)

			// 如果临时文件夹不存在，就创建一个
			tmpDir := fmt.Sprintf("./%s/", constant.TempDir)
			_, err := os.Stat(tmpDir)
			if err != nil {
				if os.IsNotExist(err) {
					if err := os.Mkdir(tmpDir, os.ModePerm); err != nil {
						logger.Warnf("create temp dir failed. error: %+v", err)
					} else {
						logger.Infof("create temp dir.")
					}
				}
			}

			// 修改输出文件为真实值
			if appConfig.OutputFile == "" {
				// 如果是 target 模式，需要取第一个输入的 IP 作为文件名
				// 如果是文件模式，在文件名后面追加 _out 作为输出文件
				if appConfig.Target != "" {
					parts := strings.Split(appConfig.Target, ",")
					if len(parts) == 1 {
						appConfig.OutputFile = fmt.Sprintf("%s_out.txt", appConfig.Target)
					} else if len(parts) > 1 {
						appConfig.OutputFile = fmt.Sprintf("%s_etc_out.txt", parts[0])
					}
				} else if appConfig.InputFile != "" {
					index := strings.LastIndex(appConfig.InputFile, ".")
					if index >= 0 {
						p1 := appConfig.InputFile[:index]
						p2 := appConfig.InputFile[index:]
						appConfig.OutputFile = fmt.Sprintf("%s_out%s", p1, p2)
					} else {
						appConfig.OutputFile = fmt.Sprintf("%s_out", appConfig.InputFile)
					}
				}
			}
			if appConfig.OutputFile == "" {
				logger.Warnf("Failed to generate output filename, use default output filename: ./out.txt")
				appConfig.OutputFile = "./out.txt"
			}

			return nil
		},
	}

	return app.Run(os.Args)
}

func MainAction(c *cli.Context) error {

	// 程序的真正入口，调用不同的服务开始扫描
	logger.Debugf("appConfig: %+v", appConfig)

	// 检查参数是否有冲突
	if appConfig.InputFile != "" && appConfig.Target != "" {
		err := "the 'target' and 'input' parameters cannot be set at the same time"
		logger.Error(err)
		return fmt.Errorf(err)
	}
	if appConfig.InputFile == "" && appConfig.Target == "" {
		err := "the 'target' and 'input' cannot be empty at the same time"
		return fmt.Errorf(err)
	}

	// masscan 引擎的任务队列，可以设置的大一点
	masscanJobChan := make(chan string, 64)
	nmapJobChan := make(chan service.NmapJob, 64)
	resultsChan := make(chan service.PortResult, 4)

	var mainWg sync.WaitGroup

	// 启动 saver engine
	saverEngine := service.NewSaverEngine(&mainWg, &resultsChan)
	mainWg.Add(1)
	go saverEngine.Run()

	// 启动 nmap engine
	nmapEngine := service.NewNmapEngine(&mainWg, &nmapJobChan, &resultsChan)
	mainWg.Add(1)
	go nmapEngine.Run()

	// 启动 masscan engine
	masscanEngine := service.NewMasscanEngine(&mainWg, &masscanJobChan, &nmapJobChan)
	mainWg.Add(1)
	go masscanEngine.Run()

	// 启动 TaskBuilder
	taskBuilderEngine := service.NewTaskBuilder(&mainWg, &masscanJobChan)
	mainWg.Add(1)
	go taskBuilderEngine.Run()

	mainWg.Wait()
	logger.Debugf("MainAction end")
	logger.Infof("Write result to file: %s", appConfig.OutputFile)
	return nil
}
