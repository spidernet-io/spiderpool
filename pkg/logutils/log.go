// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package logutils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/spidernet-io/spiderpool/pkg/constant"
)

// Pre-define log instance with default info level.
var (
	Logger       *zap.Logger
	LoggerStderr *zap.Logger
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

const (
	DefaultIPAMPluginLogFilePath = "/var/log/spidernet/spiderpool.log"
	// MaxSize    = 100 // MB
	DefaultLogFileMaxSize int = 100
	// MaxAge     = 30 // days (no limit)
	DefaultLogFileMaxAge = 30
	// MaxBackups = 10 // no limit
	DefaultLogFileMaxBackups = 10
)

// LogMode is a type help you choose the log output mode, which supports "stderr","stdout","file".
type LogMode uint32

const (
	OUTPUT_FILE   LogMode = 1 // 0001
	OUTPUT_STDERR LogMode = 2 // 0010
	OUTPUT_STDOUT LogMode = 4 // 0100
)

// LogMode is a type help you choose the log level.
type LogLevel = zapcore.Level

const (
	DebugLevel = zapcore.DebugLevel
	InfoLevel  = zapcore.InfoLevel
	WarnLevel  = zapcore.WarnLevel
	ErrorLevel = zapcore.ErrorLevel
	PanicLevel = zapcore.PanicLevel
	FatalLevel = zapcore.FatalLevel
)

func ConvertLogLevel(level string) *LogLevel {
	var logLevel LogLevel
	if strings.EqualFold(level, constant.LogDebugLevelStr) {
		logLevel = DebugLevel
	} else if strings.EqualFold(level, constant.LogInfoLevelStr) {
		logLevel = InfoLevel
	} else if strings.EqualFold(level, constant.LogWarnLevelStr) {
		logLevel = WarnLevel
	} else if strings.EqualFold(level, constant.LogErrorLevelStr) {
		logLevel = ErrorLevel
	} else if strings.EqualFold(level, constant.LogFatalLevelStr) {
		logLevel = FatalLevel
	} else if strings.EqualFold(level, constant.LogPanicLevelStr) {
		logLevel = PanicLevel
	} else {
		return nil
	}
	return &logLevel
}

func init() {
	err := InitStdoutLogger(InfoLevel)
	if nil != err {
		panic(err)
	}

	err = InitStderrLogger(InfoLevel)
	if nil != err {
		panic(err)
	}
}

// NewLoggerWithOption provides the ability to custom log with options.
// You can choose 'output format', 'output mode' and decide to use 'time prefix', 'function caller suffix'
// 'log level prefix' or not.
// If you choose 'file output mode', you can use 'stdout and file' or 'stderr and file' together with '|'.
// The param fileOutputOption should be a pointer for FileOutputOption , and it could be nill.
// If the param isn't nil, the FileOutputOption.MaxSize, FileOutputOption.MaxAge,
// and FileOutputOption.MaxBackups have to be nonnegative number, or they will be set to default value.
func NewLoggerWithOption(format LogFormat, outputMode LogMode, fileOutputOption *FileOutputOption,
	addTimePrefix, addLogLevelPrefix, addFuncCallerSuffix bool, logLevel LogLevel) (*zap.Logger, error) {

	// MaxSize    = 100 // MB
	// MaxAge     = 30 // days (no limit)
	// MaxBackups = 10 // no limit
	// LocalTime  = false // use computers local time, UTC by default
	// Compress   = false // compress the rotated log in gzip format
	var fileLoggerConf lumberjack.Logger
	if fileOutputOption != nil {
		fileLoggerConf.Filename = fileOutputOption.Filename
		fileLoggerConf.MaxSize = fileOutputOption.MaxSize
		fileLoggerConf.MaxAge = fileOutputOption.MaxAge
		fileLoggerConf.MaxBackups = fileOutputOption.MaxBackups

		if fileOutputOption.Filename == "" {
			fileLoggerConf.Filename = DefaultIPAMPluginLogFilePath
		}
		if fileOutputOption.MaxSize < 0 {
			fileLoggerConf.MaxSize = DefaultLogFileMaxSize
		}
		if fileOutputOption.MaxAge < 0 {
			fileLoggerConf.MaxAge = DefaultLogFileMaxAge
		}
		if fileOutputOption.MaxBackups < 0 {
			fileLoggerConf.MaxBackups = DefaultLogFileMaxBackups
		}

		err := os.MkdirAll(filepath.Dir(fileOutputOption.Filename), 0755)
		if nil != err {
			return nil, fmt.Errorf("Failed to create path for CNI log file: %v", filepath.Dir(fileOutputOption.Filename))
		}
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
		return nil, fmt.Errorf("log output mode can't set to stdout with stderr together")
	default:
		ws = zapcore.AddSync(os.Stdout)
	}

	// set zap encoder configuration
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = nil
	encoderConfig.EncodeCaller = nil
	encoderConfig.EncodeLevel = nil

	if addTimePrefix {
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	if addFuncCallerSuffix {
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	}

	if addLogLevelPrefix {
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	}

	var encoder zapcore.Encoder

	switch format {
	case JsonLogFormat:
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	case ConsoleLogFormat:
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	default:
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	logger := zap.New(zapcore.NewCore(
		encoder,
		ws,
		zap.NewAtomicLevelAt(logLevel),
	), zap.AddCaller())
	return logger, nil
}

// InitStdoutLogger create  Logger instance with default configuration for 'stdout' usage, it's JsonLogFormat.
func InitStdoutLogger(logLevel LogLevel) error {
	l, err := NewLoggerWithOption(JsonLogFormat, OUTPUT_STDOUT, nil, true, true, true, logLevel)
	if nil != err {
		return fmt.Errorf("Failed to init logger for stdout: %v", err)
	}
	Logger = l
	return nil
}

// InitStderrLogger create LoggerStderr instance for 'stderr' usage, it's ConsoleLogFormat.
// It wouldn't provide 'time prefix', 'function caller suffix' and 'log level prefix' in output.
func InitStderrLogger(logLevel LogLevel) error {
	l, err := NewLoggerWithOption(ConsoleLogFormat, OUTPUT_STDERR, nil, false, false, false, logLevel)
	if nil != err {
		return fmt.Errorf("Failed to init logger for stderr: %v", err)
	}
	LoggerStderr = l
	return nil
}

// InitFileLogger sets LoggerFile configuration for 'file output' usage.
// fileMaxSize unit MB, fileMaxAge unit days, fileMaxBackups unit counts.
func InitFileLogger(logLevel LogLevel, filePath string, fileMaxSize, fileMaxAge, fileMaxBackups int) (*zap.Logger, error) {
	fileLoggerConf := FileOutputOption{
		Filename:   filePath,
		MaxSize:    fileMaxSize,
		MaxAge:     fileMaxAge,
		MaxBackups: fileMaxBackups,
	}
	logFile, err := NewLoggerWithOption(JsonLogFormat, OUTPUT_FILE, &fileLoggerConf, true, true, true, logLevel)
	if nil != err {
		return nil, fmt.Errorf("Failed to init logger for file: %v", err)
	}

	return logFile, nil
}

// loggerKey is how we find Loggers in a context.Context.
type loggerKey struct{}

// FromContext returns a logger with predefined values from a context.Context.
func FromContext(ctx context.Context) *zap.Logger {
	log := Logger
	if ctx != nil {
		if logger, ok := ctx.Value(loggerKey{}).(*zap.Logger); ok {
			log = logger
		}
	}

	return log
}

// IntoContext takes a context and sets the logger as one of its values.
// Use FromContext function to retrieve the logger.
func IntoContext(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}
