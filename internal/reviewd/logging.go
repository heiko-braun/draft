package reviewd

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// Logger provides structured JSON logging.
type Logger struct {
	mu     sync.Mutex
	out    io.Writer
	level  LogLevel
	fields map[string]string
}

// LogLevel represents log severity.
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

// NewLogger creates a logger that writes JSON lines to stdout.
func NewLogger(level string) *Logger {
	return &Logger{
		out:   os.Stdout,
		level: parseLevel(level),
	}
}

// With returns a new logger with additional static fields.
func (l *Logger) With(key, value string) *Logger {
	fields := make(map[string]string, len(l.fields)+1)
	for k, v := range l.fields {
		fields[k] = v
	}
	fields[key] = value
	return &Logger{out: l.out, level: l.level, fields: fields}
}

func (l *Logger) Debug(msg string, kvs ...string) { l.log(LevelDebug, msg, kvs) }
func (l *Logger) Info(msg string, kvs ...string)  { l.log(LevelInfo, msg, kvs) }
func (l *Logger) Warn(msg string, kvs ...string)  { l.log(LevelWarn, msg, kvs) }
func (l *Logger) Error(msg string, kvs ...string) { l.log(LevelError, msg, kvs) }

func (l *Logger) log(level LogLevel, msg string, kvs []string) {
	if level < l.level {
		return
	}

	entry := map[string]string{
		"time":  time.Now().UTC().Format(time.RFC3339),
		"level": levelString(level),
		"msg":   msg,
	}
	for k, v := range l.fields {
		entry[k] = v
	}
	for i := 0; i+1 < len(kvs); i += 2 {
		entry[kvs[i]] = kvs[i+1]
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	json.NewEncoder(l.out).Encode(entry)
}

func parseLevel(s string) LogLevel {
	switch s {
	case "debug":
		return LevelDebug
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

func levelString(l LogLevel) string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "info"
	}
}
