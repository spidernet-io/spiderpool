// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package logutils

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Pre-define log instance with default info level.
var (
	Logger       *zap.Logger
	LoggerStderr *zap.Logger
	LoggerFile   *zap.Logger
)

// LogFormat is a type help you choose the log output format, which supports "json" and "console".
type LogFormat string

const (
	JsonLogFormat    LogFormat = "json"
	ConsoleLogFormat LogFormat = "console"
)

// FileOutputOption supports the configuration for log file
type FileOutputOption struct {
	Filename   string
	MaxSize    int
	MaxAge     int
	MaxBackups int
}

// LogMode is a type help you choose the log output mode, which supports "stderr","stdout","file".
type LogMode uint32

const (
	OUTPUT_FILE   LogMode = 1 // 0001
	OUTPUT_STDERR LogMode = 2 // 0010
	OUTPUT_STDOUT LogMode = 4 // 0100
)

// LogMode is a type help you choose the log level.
type LogLevel string

var (
	LogDebug LogLevel = "debug"
	LogInfo  LogLevel = "info"
	LogWarn  LogLevel = "warn"
	LogError LogLevel = "error"
	LogPanic LogLevel = "panic"
	LogFatal LogLevel = "fatal"
)

func init() {
	err := InitStdoutLogger()
	if nil != err {
		panic(err)
	}

	err = InitStderrLogger()
	if nil != err {
		panic(err)
	}

	err = InitFileLogger("/tmp/cni.log")
	if nil != err {
		panic(err)
	}

}

// NewLoggerWithOption provides the ability to custom log with options.
// You can choose 'output format', 'output mode' and decide to use 'time prefix', 'function caller suffix'
// 'log level prefix' or not.
// If you choose 'file output mode', you can use 'stdout and file' or 'stderr and file' together.
func NewLoggerWithOption(format LogFormat, outputMode LogMode, fileOutputOption *FileOutputOption,
	addTimePrefix, addLogLevelPrefix, addFuncCallerSuffix bool, logLevel LogLevel) (*zap.Logger, error) {

	var fileLoggerConf lumberjack.Logger
	if fileOutputOption != nil {
		fileLoggerConf.Filename = fileOutputOption.Filename
		fileLoggerConf.MaxSize = fileOutputOption.MaxSize
		fileLoggerConf.MaxAge = fileOutputOption.MaxAge
		fileLoggerConf.MaxBackups = fileOutputOption.MaxBackups
	}

	var ws zapcore.WriteSyncer
	switch outputMode {
	case OUTPUT_FILE:
		ws = zapcore.AddSync(&fileLoggerConf)
	case OUTPUT_STDOUT:
		ws = zapcore.AddSync(os.Stdout)
	case OUTPUT_STDERR:
		ws = zapcore.AddSync(os.Stderr)
	case OUTPUT_STDOUT | OUTPUT_FILE:
		ws = zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(&fileLoggerConf))
	case OUTPUT_STDERR | OUTPUT_FILE:
		ws = zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stderr), zapcore.AddSync(&fileLoggerConf))
	case OUTPUT_STDOUT | OUTPUT_STDERR:
		return nil, fmt.Errorf("log output mode can't set to stdout with stderr together.")
	}

	// set zap encoder configuration
	encoderConfig := zap.NewProductionEncoderConfig()
	if addTimePrefix {
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		encoderConfig.EncodeTime = nil
	}

	if addFuncCallerSuffix {
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	} else {
		encoderConfig.EncodeCaller = nil
	}

	if addLogLevelPrefix {
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	} else {
		encoderConfig.EncodeLevel = nil
	}

	logger := zap.New(zapcore.NewCore(
		convertToZapFormatEncoder(format, encoderConfig),
		ws,
		zap.NewAtomicLevelAt(ConvertToZapLevel(logLevel)),
	), zap.AddCaller())
	return logger, nil
}

// InitStdoutLogger create  Logger instance with default configuration for 'stdout' usage, it's JsonLogFormat.
func InitStdoutLogger() error {
	l, err := NewLoggerWithOption(JsonLogFormat, OUTPUT_STDOUT, nil, true, true, true, LogInfo)
	if nil != err {
		return fmt.Errorf("Failed to init logger for stdout: %v", err)
	}
	Logger = l
	return nil
}

// InitStderrLogger create LoggerStderr instance for 'stderr' usage, it's ConsoleLogFormat.
// It wouldn't provide 'time prefix', 'function caller suffix' and 'log level prefix' in output.
func InitStderrLogger() error {
	l, err := NewLoggerWithOption(ConsoleLogFormat, OUTPUT_STDERR, nil, false, false, false, LogInfo)
	if nil != err {
		return fmt.Errorf("Failed to init logger for stderr: %v", err)
	}
	LoggerStderr = l
	return nil
}

// InitFileLogger sets LoggerFile configuration for 'file output' usage.
func InitFileLogger(filePath string) error {
	err := os.MkdirAll(filepath.Dir(filePath), 0755)
	if nil != err {
		return fmt.Errorf("Failed to create path for CNI log file: %v", filepath.Dir(filePath))
	}

	// MaxSize    = 100 // MB
	// MaxAge     = 30 // days (no limit)
	// MaxBackups = 10 // no limit
	// LocalTime  = false // use computers local time, UTC by default
	// Compress   = false // compress the rotated log in gzip format
	fileLoggerConf := FileOutputOption{
		Filename:   filePath,
		MaxSize:    100,
		MaxAge:     30,
		MaxBackups: 10,
	}
	l, err := NewLoggerWithOption(JsonLogFormat, OUTPUT_FILE, &fileLoggerConf, true, true, true, LogInfo)
	if nil != err {
		return fmt.Errorf("Failed to init logger for file: %v", err)
	}
	LoggerFile = l
	return nil
}

// ConvertToZapLevel converts log level string to zapcore.Level.
// if error is not nil it will return the info log level
func ConvertToZapLevel(level LogLevel) zapcore.Level {
	var lvl zapcore.Level
	if err := lvl.Set(string(level)); nil != err {
		fmt.Fprintf(os.Stderr, "error setting log level %q: %s, and it will set as Info level\n", level, err)
		return zapcore.InfoLevel
	}
	return lvl
}

// convertToZapFormatEncoder converts and validated log format string.
func convertToZapFormatEncoder(format LogFormat, encoderConfig zapcore.EncoderConfig) zapcore.Encoder {
	switch format {
	case ConsoleLogFormat, "":
		return zapcore.NewConsoleEncoder(encoderConfig)
	case JsonLogFormat:
		return zapcore.NewJSONEncoder(encoderConfig)
	default:
		fmt.Fprintf(os.Stderr, "unknown log format: %s, supported values json, console and it will be set as %s\n",
			format, JsonLogFormat)
		return zapcore.NewConsoleEncoder(encoderConfig)
	}
}
