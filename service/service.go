package service

import (
	"cloud-scanner/config"
	"cloud-scanner/logging"
)

var logger = logging.GetSugar()
var appConfig = config.GetAppConfig()

// MasscanResult 存储 Masscan 的扫描结果
// 这样设计是为了以后方便单独保存 Masscan 和 Nmap 的扫描结果
type MasscanResult struct {
	Host     string
	Port     uint
	Protocol string
}

// NmapJob 表示一个 nmap 任务
type NmapJob struct {
	value []MasscanResult
	UUID  string
}

// PortResult 表示一个扫描结果
type PortResult struct {
	Host     string
	Port     uint
	Protocol string
	Service  string
	Banner   string
}
