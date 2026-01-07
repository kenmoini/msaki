package logging

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/kenmoini/msaki/internal/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger wraps standard logging with file rotation support
type Logger struct {
	*log.Logger
	writer io.WriteCloser
}

// LogManager manages all application loggers
type LogManager struct {
	AccessLogger *Logger
	ErrorLogger  *Logger
	ChatLogger   *ChatLogger
	config       *config.ObservabilityConfig
}

// NewLogManager creates a new log manager based on configuration
func NewLogManager(cfg *config.ObservabilityConfig) (*LogManager, error) {
	lm := &LogManager{
		config: cfg,
	}

	var err error

	// Initialize access logger
	if cfg.AccessLogs.Enabled {
		lm.AccessLogger, err = newLogger(cfg.AccessLogs, "ACCESS")
		if err != nil {
			return nil, err
		}
	}

	// Initialize error logger
	if cfg.ErrorLogs.Enabled {
		lm.ErrorLogger, err = newLogger(cfg.ErrorLogs, "ERROR")
		if err != nil {
			return nil, err
		}
	}

	// Initialize chat logger
	if cfg.ChatLogs.Enabled {
		lm.ChatLogger, err = NewChatLogger(&cfg.ChatLogs)
		if err != nil {
			return nil, err
		}
	}

	return lm, nil
}

// newLogger creates a new logger with the given configuration
func newLogger(cfg config.LogConfig, prefix string) (*Logger, error) {
	var writers []io.Writer

	// Parse file rotation size
	maxSize, err := config.ParseFileSize(cfg.FileRotation)
	if err != nil {
		return nil, err
	}
	// Convert bytes to megabytes for lumberjack
	maxSizeMB := int(maxSize / (1024 * 1024))
	if maxSizeMB < 1 {
		maxSizeMB = 1
	}

	// Create file writer with rotation if file is specified
	var fileWriter *lumberjack.Logger
	if cfg.File != "" {
		// Ensure directory exists
		dir := filepath.Dir(cfg.File)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}

		fileWriter = &lumberjack.Logger{
			Filename:   cfg.File,
			MaxSize:    maxSizeMB,
			MaxBackups: 5,
			MaxAge:     30,
			Compress:   true,
		}
		writers = append(writers, fileWriter)
	}

	// Add stdout if shared output is enabled
	if cfg.SharedOutput {
		writers = append(writers, os.Stdout)
	}

	// If no writers configured, default to stdout
	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}

	// Create multi-writer
	var writer io.Writer
	if len(writers) == 1 {
		writer = writers[0]
	} else {
		writer = io.MultiWriter(writers...)
	}

	logger := &Logger{
		Logger: log.New(writer, "["+prefix+"] ", log.LstdFlags),
	}

	if fileWriter != nil {
		logger.writer = fileWriter
	}

	return logger, nil
}

// Close closes all loggers
func (lm *LogManager) Close() error {
	if lm.AccessLogger != nil && lm.AccessLogger.writer != nil {
		lm.AccessLogger.writer.Close()
	}
	if lm.ErrorLogger != nil && lm.ErrorLogger.writer != nil {
		lm.ErrorLogger.writer.Close()
	}
	if lm.ChatLogger != nil {
		lm.ChatLogger.Close()
	}
	return nil
}

// Info logs an info message to access logger
func (lm *LogManager) Info(format string, v ...interface{}) {
	if lm.AccessLogger != nil {
		lm.AccessLogger.Printf(format, v...)
	}
}

// Error logs an error message to error logger
func (lm *LogManager) Error(format string, v ...interface{}) {
	if lm.ErrorLogger != nil {
		lm.ErrorLogger.Printf(format, v...)
	}
}
