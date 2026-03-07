package logger

import (
	"log"
	"os"
)

// Logger wraps the standard logger with level support.
type Logger struct {
	level  Level
	prefix string
	logger *log.Logger
}

// Level represents a log level.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// New creates a new logger.
func New(level string, prefix string) *Logger {
	l := &Logger{
		prefix: prefix,
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}

	switch level {
	case "debug":
		l.level = LevelDebug
	case "warn":
		l.level = LevelWarn
	case "error":
		l.level = LevelError
	default:
		l.level = LevelInfo
	}

	return l
}

func (l *Logger) Debug(msg string, args ...interface{}) {
	if l.level <= LevelDebug {
		l.logger.Printf("[DEBUG] [%s] "+msg, append([]interface{}{l.prefix}, args...)...)
	}
}

func (l *Logger) Info(msg string, args ...interface{}) {
	if l.level <= LevelInfo {
		l.logger.Printf("[INFO] [%s] "+msg, append([]interface{}{l.prefix}, args...)...)
	}
}

func (l *Logger) Warn(msg string, args ...interface{}) {
	if l.level <= LevelWarn {
		l.logger.Printf("[WARN] [%s] "+msg, append([]interface{}{l.prefix}, args...)...)
	}
}

func (l *Logger) Error(msg string, args ...interface{}) {
	if l.level <= LevelError {
		l.logger.Printf("[ERROR] [%s] "+msg, append([]interface{}{l.prefix}, args...)...)
	}
}
