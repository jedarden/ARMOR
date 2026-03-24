// Package logging provides structured JSON logging for ARMOR.
package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

// Level represents a log level.
type Level int

const (
	// LevelDebug is for debug messages.
	LevelDebug Level = iota
	// LevelInfo is for informational messages.
	LevelInfo
	// LevelWarn is for warning messages.
	LevelWarn
	// LevelError is for error messages.
	LevelError
)

// String returns the string representation of the level.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger is a structured JSON logger.
type Logger struct {
	mu      sync.Mutex
	writer  io.Writer
	level   Level
	service string
	fields  map[string]interface{}
}

// Entry represents a log entry.
type Entry struct {
	Time    string                 `json:"time"`
	Level   string                 `json:"level"`
	Service string                 `json:"service,omitempty"`
	Msg     string                 `json:"msg"`
	Fields  map[string]interface{} `json:",omitempty"`
}

// New creates a new logger.
func New(service string) *Logger {
	return &Logger{
		writer:  os.Stdout,
		level:   LevelInfo,
		service: service,
		fields:  make(map[string]interface{}),
	}
}

// SetLevel sets the minimum log level.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetOutput sets the output writer.
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.writer = w
}

// WithField returns a new logger with an additional field.
func (l *Logger) WithField(key string, value interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := &Logger{
		writer:  l.writer,
		level:   l.level,
		service: l.service,
		fields:  make(map[string]interface{}),
	}

	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	newLogger.fields[key] = value

	return newLogger
}

// WithFields returns a new logger with additional fields.
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := &Logger{
		writer:  l.writer,
		level:   l.level,
		service: l.service,
		fields:  make(map[string]interface{}),
	}

	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	for k, v := range fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

// log writes a log entry.
func (l *Logger) log(level Level, msg string, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry := Entry{
		Time:    time.Now().UTC().Format(time.RFC3339Nano),
		Level:   level.String(),
		Service: l.service,
		Msg:     msg,
		Fields:  fields,
	}

	// Merge base fields
	if len(l.fields) > 0 {
		if entry.Fields == nil {
			entry.Fields = make(map[string]interface{})
		}
		for k, v := range l.fields {
			entry.Fields[k] = v
		}
	}

	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("failed to marshal log entry: %v", err)
		return
	}

	l.writer.Write(data)
	l.writer.Write([]byte("\n"))
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string) {
	l.log(LevelDebug, msg, nil)
}

// Debugf logs a formatted debug message.
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(LevelDebug, fmt.Sprintf(format, args...), nil)
}

// Info logs an informational message.
func (l *Logger) Info(msg string) {
	l.log(LevelInfo, msg, nil)
}

// Infof logs a formatted informational message.
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(LevelInfo, fmt.Sprintf(format, args...), nil)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string) {
	l.log(LevelWarn, msg, nil)
}

// Warnf logs a formatted warning message.
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(LevelWarn, fmt.Sprintf(format, args...), nil)
}

// Error logs an error message.
func (l *Logger) Error(msg string) {
	l.log(LevelError, msg, nil)
}

// Errorf logs a formatted error message.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(LevelError, fmt.Sprintf(format, args...), nil)
}

// Fatal logs an error message and exits.
func (l *Logger) Fatal(msg string) {
	l.log(LevelError, msg, nil)
	os.Exit(1)
}

// Fatalf logs a formatted error message and exits.
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log(LevelError, fmt.Sprintf(format, args...), nil)
	os.Exit(1)
}

// Default logger instance.
var defaultLogger = New("armor")

// SetDefault sets the default logger.
func SetDefault(l *Logger) {
	defaultLogger = l
}

// Default returns the default logger.
func Default() *Logger {
	return defaultLogger
}

// Package-level convenience functions using the default logger.

// Debug logs a debug message with the default logger.
func Debug(msg string) {
	defaultLogger.Debug(msg)
}

// Debugf logs a formatted debug message with the default logger.
func Debugf(format string, args ...interface{}) {
	defaultLogger.Debugf(format, args...)
}

// Info logs an informational message with the default logger.
func Info(msg string) {
	defaultLogger.Info(msg)
}

// Infof logs a formatted informational message with the default logger.
func Infof(format string, args ...interface{}) {
	defaultLogger.Infof(format, args...)
}

// Warn logs a warning message with the default logger.
func Warn(msg string) {
	defaultLogger.Warn(msg)
}

// Warnf logs a formatted warning message with the default logger.
func Warnf(format string, args ...interface{}) {
	defaultLogger.Warnf(format, args...)
}

// Error logs an error message with the default logger.
func Error(msg string) {
	defaultLogger.Error(msg)
}

// Errorf logs a formatted error message with the default logger.
func Errorf(format string, args ...interface{}) {
	defaultLogger.Errorf(format, args...)
}

// Fatal logs an error message and exits with the default logger.
func Fatal(msg string) {
	defaultLogger.Fatal(msg)
}

// Fatalf logs a formatted error message and exits with the default logger.
func Fatalf(format string, args ...interface{}) {
	defaultLogger.Fatalf(format, args...)
}

// WithField returns a new logger with an additional field using the default logger.
func WithField(key string, value interface{}) *Logger {
	return defaultLogger.WithField(key, value)
}

// WithFields returns a new logger with additional fields using the default logger.
func WithFields(fields map[string]interface{}) *Logger {
	return defaultLogger.WithFields(fields)
}
