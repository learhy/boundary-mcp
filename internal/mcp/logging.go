package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// LogLevel for structured stderr logging.
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// Logger writes structured JSON logs to stderr. Never stdout — stdout is the
// MCP transport.
type Logger struct {
	mu  sync.Mutex
	out io.Writer
}

// NewLogger creates a logger writing to stderr.
func NewLogger() *Logger {
	return &Logger{out: os.Stderr}
}

// Log writes a structured log entry.
func (l *Logger) Log(level LogLevel, msg string, fields map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := map[string]interface{}{
		"level": string(level),
		"msg":   msg,
		"time":  time.Now().UTC().Format(time.RFC3339),
	}
	for k, v := range fields {
		entry[k] = v
	}

	data, err := json.Marshal(entry)
	if err != nil {
		// Last resort
		fmt.Fprintf(l.out, `{"level":"error","msg":"failed to marshal log entry","time":"%s"}`+"\n", time.Now().UTC().Format(time.RFC3339))
		return
	}
	l.out.Write(append(data, '\n'))
}

func (l *Logger) Debug(msg string, fields ...map[string]interface{}) {
	m := mergeFields(fields)
	l.Log(LogLevelDebug, msg, m)
}

func (l *Logger) Info(msg string, fields ...map[string]interface{}) {
	m := mergeFields(fields)
	l.Log(LogLevelInfo, msg, m)
}

func (l *Logger) Warn(msg string, fields ...map[string]interface{}) {
	m := mergeFields(fields)
	l.Log(LogLevelWarn, msg, m)
}

func (l *Logger) Error(msg string, fields ...map[string]interface{}) {
	m := mergeFields(fields)
	l.Log(LogLevelError, msg, m)
}

func mergeFields(fields []map[string]interface{}) map[string]interface{} {
	if len(fields) == 0 {
		return nil
	}
	merged := make(map[string]interface{})
	for _, f := range fields {
		for k, v := range f {
			merged[k] = v
		}
	}
	return merged
}