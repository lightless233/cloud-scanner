package logging

import (
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var logger zap.Logger
var sugarLogger zap.SugaredLogger

func GetLogger() *zap.Logger {
	return &logger
}

func GetSugar() *zap.SugaredLogger {
	return &sugarLogger
}

func InitLogger(debug bool) {

	// 获取可执行文件的路径，默认将日志文件放到可执行文件同级
	// 如果有其他需求再修改这部分代码
	file, _ := exec.LookPath(os.Args[0])
	execPath, _ := filepath.Abs(file)
	execPath = execPath[:strings.LastIndex(execPath, string(os.PathSeparator))]

	lumberjackLogger := &lumberjack.Logger{
		Filename:   fmt.Sprintf("%s%clog.log", execPath, os.PathSeparator),
		MaxSize:    10,
		MaxBackups: 5,
		MaxAge:     28,
		Compress:   false,
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:          "time",
		LevelKey:         "level",
		NameKey:          "name",
		CallerKey:        "caller",
		FunctionKey:      "function",
		MessageKey:       "message",
		StacktraceKey:    zapcore.OmitKey,
		ConsoleSeparator: "|",
		LineEnding:       zapcore.DefaultLineEnding,
		EncodeLevel:      zapcore.CapitalLevelEncoder,
		EncodeTime: func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.Format(time.RFC3339Nano))
		},
		EncodeName:     zapcore.FullNameEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	if debug {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderConfig.ConsoleSeparator = " "
	}
	encoder := zapcore.NewConsoleEncoder(encoderConfig)

	syncer := zapcore.NewMultiWriteSyncer(zapcore.AddSync(lumberjackLogger), zapcore.AddSync(os.Stdout))
	core := zapcore.NewCore(encoder, syncer, zapcore.DebugLevel)

	l := zap.New(core, zap.AddCaller())

	logger = *l
	sugarLogger = *l.Sugar()
}
