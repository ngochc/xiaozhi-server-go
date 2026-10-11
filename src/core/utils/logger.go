package utils

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// LogLevel represents log level
type LogLevel string

const (
	DebugLevel LogLevel = "debug"
	InfoLevel  LogLevel = "info"
	WarnLevel  LogLevel = "warn"
	ErrorLevel LogLevel = "error"
)

const (
	LogRetentionDays = 7 // Log retention in days, hardcoded to 7 days
)

var DefaultLogger *Logger

type LogCfg struct {
	LogFormat string `yaml:"log_format" json:"log_format"`
	LogLevel  string `yaml:"log_level" json:"log_level"`
	LogDir    string `yaml:"log_dir" json:"log_dir"`
	LogFile   string `yaml:"log_file" json:"log_file"`
}

// CustomTextHandler custom text handler supporting color output and formatting
type CustomTextHandler struct {
	writer io.Writer
	level  slog.Level
	mu     sync.Mutex
}

var (
	colorReset  = "\x1b[0m"
	colorTime   = "\x1b[93m" // Time: bright yellow
	colorDebug  = "\x1b[36m" // DEBUG: cyan
	colorInfo   = "\x1b[32m" // INFO: green
	colorWarn   = "\x1b[33m" // WARN: yellow
	colorError  = "\x1b[31m" // ERROR: red
	colorASR    = "\x1b[35m" // ASR: magenta
	colorLLM    = "\x1b[34m" // LLM: blue
	colorTTS    = "\x1b[95m" // TTS: bright magenta
	colorTiming = "\x1b[92m" // Timing: bright green
)

func (h *CustomTextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *CustomTextHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Get timestamp
	timeStr := r.Time.Format("2006-01-02 15:04:05.000")

	// Get log level
	levelStr := r.Level.String()

	// Apply colors
	var levelColor string
	switch r.Level {
	case slog.LevelDebug:
		levelColor = colorDebug
	case slog.LevelInfo:
		levelColor = colorInfo
	case slog.LevelWarn:
		levelColor = colorWarn
	case slog.LevelError:
		levelColor = colorError
	default:
		levelColor = colorReset
	}

	// Check if this is a special stage log
	var stageColor string
	var isStageLog bool
	msg := r.Message

	if strings.HasPrefix(msg, "[ASR]") {
		stageColor = colorASR
		isStageLog = true
	} else if strings.HasPrefix(msg, "[LLM]") {
		stageColor = colorLLM
		isStageLog = true
	} else if strings.HasPrefix(msg, "[TTS]") {
		stageColor = colorTTS
		isStageLog = true
	} else if strings.HasPrefix(msg, "[TIMING]") {
		stageColor = colorTiming
		isStageLog = true
	}

	// Build output
	var output string
	if isStageLog {
		// Stage log format: [time] [stage] message
		output = fmt.Sprintf("%s[%s]%s %s%s%s",
			colorTime, timeStr, colorReset,
			stageColor, msg, colorReset)
	} else {
		// Normal log format: [time] [level] message
		output = fmt.Sprintf("%s[%s]%s %s[%s]%s %s",
			colorTime, timeStr, colorReset,
			levelColor, levelStr, colorReset,
			msg)
	}

	// Append attributes if any
	if r.NumAttrs() > 0 {
		output += " {"
		r.Attrs(func(a slog.Attr) bool {
			output += fmt.Sprintf(" %s=%v", a.Key, a.Value)
			return true
		})
		output += " }"
	}
	output += "\n"

	_, err := h.writer.Write([]byte(output))
	return err
}

func (h *CustomTextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h // Simplified implementation
}

func (h *CustomTextHandler) WithGroup(name string) slog.Handler {
	return h // Simplified implementation
}

// Logger implementation
type Logger struct {
	config      *LogCfg
	jsonLogger  *slog.Logger // File JSON output
	textLogger  *slog.Logger // Console text output
	logFile     *os.File
	currentDate string        // Current date YYYY-MM-DD
	mu          sync.RWMutex  // RWMutex for concurrency
	ticker      *time.Ticker  // Timer
	stopCh      chan struct{} // Stop signal
}

// configLogLevelToSlogLevel converts config log level string to slog.Level
func configLogLevelToSlogLevel(configLevel string) slog.Level {
	switch strings.ToLower(configLevel) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// NewLogger creates a new logger instance
func NewLogger(config *LogCfg) (*Logger, error) {
	// Ensure log directory exists
	if err := os.MkdirAll(config.LogDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	// Open or create log file
	logPath := filepath.Join(config.LogDir, config.LogFile)
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	// Set slog level
	slogLevel := configLogLevelToSlogLevel(config.LogLevel)

	// Create JSON handler (for file output)
	jsonHandler := slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level: slogLevel,
	})

	// Create custom text handler (for console output)
	customHandler := &CustomTextHandler{
		writer: os.Stdout,
		level:  slogLevel,
	}

	// Create logger instance
	jsonLogger := slog.New(jsonHandler)
	textLogger := slog.New(customHandler)

	logger := &Logger{
		config:      config,
		jsonLogger:  jsonLogger,
		textLogger:  textLogger,
		logFile:     file,
		currentDate: time.Now().Format("2006-01-02"),
		stopCh:      make(chan struct{}),
	}

	// Start log rotation checker
	logger.startRotationChecker()
	if DefaultLogger == nil {
		DefaultLogger = logger
	}

	return logger, nil
}

// startRotationChecker starts background timer checker
func (l *Logger) startRotationChecker() {
	l.ticker = time.NewTicker(1 * time.Minute) // Check every minute
	go func() {
		for {
			select {
			case <-l.ticker.C:
				l.checkAndRotate()
			case <-l.stopCh:
				return
			}
		}
	}()
}

// checkAndRotate checks and executes log rotation
func (l *Logger) checkAndRotate() {
	today := time.Now().Format("2006-01-02")
	if today != l.currentDate {
		l.rotateLogFile(today)
		l.cleanOldLogs()
	}
}

// rotateLogFile performs log rotation
func (l *Logger) rotateLogFile(newDate string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Close current log file
	if l.logFile != nil {
		l.logFile.Close()
	}

	// Construct old and new filenames
	logDir := l.config.LogDir
	currentLogPath := filepath.Join(logDir, l.config.LogFile)

	// Generate filename with date
	baseFileName := strings.TrimSuffix(l.config.LogFile, filepath.Ext(l.config.LogFile))
	ext := filepath.Ext(l.config.LogFile)
	archivedLogPath := filepath.Join(logDir, fmt.Sprintf("%s-%s%s", baseFileName, l.currentDate, ext))

	// Rename current log file to dated file
	if _, err := os.Stat(currentLogPath); err == nil {
		if err := os.Rename(currentLogPath, archivedLogPath); err != nil {
			// If rename fails, log to console
			l.textLogger.Error("failed to rename log file", slog.String("error", err.Error()))
		}
	}

	// Create new log file
	file, err := os.OpenFile(currentLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		l.textLogger.Error("failed to create new log file", slog.String("error", err.Error()))
		return
	}

	// Update logger configuration
	l.logFile = file
	l.currentDate = newDate

	// Recreate JSON handler
	slogLevel := configLogLevelToSlogLevel(l.config.LogLevel)
	jsonHandler := slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level: slogLevel,
	})
	l.jsonLogger = slog.New(jsonHandler)

	// Log rotation information
	l.textLogger.Info("log file rotated", slog.String("new_date", newDate))
}

// cleanOldLogs cleans up outdated log files
func (l *Logger) cleanOldLogs() {
	logDir := l.config.LogDir

	// Read log directory
	entries, err := os.ReadDir(logDir)
	if err != nil {
		l.textLogger.Error("failed to read log directory", slog.String("error", err.Error()))
		return
	}

	// Calculate retention cutoff date
	cutoffDate := time.Now().AddDate(0, 0, -LogRetentionDays)
	baseFileName := strings.TrimSuffix(l.config.LogFile, filepath.Ext(l.config.LogFile))
	ext := filepath.Ext(l.config.LogFile)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		// Check if file matches dated log pattern: server-YYYY-MM-DD.log
		if strings.HasPrefix(fileName, baseFileName+"-") && strings.HasSuffix(fileName, ext) {
			// Extract date portion
			dateStr := strings.TrimPrefix(fileName, baseFileName+"-")
			dateStr = strings.TrimSuffix(dateStr, ext)

			// Parse date
			fileDate, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				continue // If date format is invalid, skip
			}

			// If file date is earlier than cutoff, delete file
			if fileDate.Before(cutoffDate) {
				filePath := filepath.Join(logDir, fileName)
				if err := os.Remove(filePath); err != nil {
					l.textLogger.Error("failed to delete old log file",
						slog.String("file", fileName),
						slog.String("error", err.Error()))
				} else {
					l.textLogger.Info("deleted old log file", slog.String("file", fileName))
				}
			}
		}
	}
}

// Close closes log file
func (l *Logger) Close() error {
	// Stop timer
	if l.ticker != nil {
		l.ticker.Stop()
	}
	// Send stop signal
	close(l.stopCh)
	// Close log file
	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

// log internal general logging helper
func (l *Logger) log(level slog.Level, msg string, fields ...interface{}) {
	// Protect concurrent access with read lock
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Build slog attributes
	var attrs []slog.Attr
	if len(fields) > 0 && fields[0] != nil {
		// Process fields parameter
		if fieldsMap, ok := fields[0].(map[string]interface{}); ok {
			// Extract and sort keys
			keys := make([]string, 0, len(fieldsMap))
			for k := range fieldsMap {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			// Append attributes in sorted key order
			for _, k := range keys {
				attrs = append(attrs, slog.Any(k, fieldsMap[k]))
			}
		} else {
			// If not a map, add directly as fields attribute
			attrs = append(attrs, slog.Any("fields", fields[0]))
		}
	}

	// Write to both file (JSON) and console (text)
	ctx := context.Background()
	l.jsonLogger.LogAttrs(ctx, level, msg, attrs...)
	l.textLogger.LogAttrs(ctx, level, msg, attrs...)
}

// Debug logs debug level message
func (l *Logger) Debug(msg any, args ...interface{}) {
	if l.config.LogLevel == "DEBUG" {
		msgText := logMessageText(msg)
		if len(args) > 0 && containsFormatPlaceholders(msgText) {
			formattedMsg := fmt.Sprintf(msgText, args...)
			l.log(slog.LevelDebug, formattedMsg)
		} else {
			l.log(slog.LevelDebug, msgText, args...)
		}
	}
}

func logMessageText(msg any) string {
	if s, ok := msg.(string); ok {
		return s
	}

	return fmt.Sprint(msg)
}

func containsFormatPlaceholders(s string) bool {
	return strings.Contains(s, "%")
}

// Info logs info level message
func (l *Logger) Info(msg any, args ...interface{}) {
	msgText := logMessageText(msg)
	// Check if format string mode
	if len(args) > 0 && containsFormatPlaceholders(msgText) {
		// Format mode: formatted message
		formattedMsg := fmt.Sprintf(msgText, args...)
		l.log(slog.LevelInfo, formattedMsg)
	} else {
		// Structured mode: structured attributes
		l.log(slog.LevelInfo, msgText, args...)
	}
}

// Warn logs warning level message
func (l *Logger) Warn(msg any, args ...interface{}) {
	msgText := logMessageText(msg)
	if len(args) > 0 && containsFormatPlaceholders(msgText) {
		formattedMsg := fmt.Sprintf(msgText, args...)
		l.log(slog.LevelWarn, formattedMsg)
	} else {
		l.log(slog.LevelWarn, msgText, args...)
	}
}

// Error logs error level message
func (l *Logger) Error(msg any, args ...interface{}) {
	msgText := logMessageText(msg)
	if len(args) > 0 && containsFormatPlaceholders(msgText) {
		formattedMsg := fmt.Sprintf(msgText, args...)
		l.log(slog.LevelError, formattedMsg)
	} else {
		l.log(slog.LevelError, msgText, args...)
	}
}

// InfoASR logs ASR stage message
func (l *Logger) InfoASR(msg string, args ...interface{}) {
	prefixedMsg := "[ASR] " + msg
	l.Info(prefixedMsg, args...)
}

// InfoLLM logs LLM stage message
func (l *Logger) InfoLLM(msg string, args ...interface{}) {
	prefixedMsg := "[LLM] " + msg
	l.Info(prefixedMsg, args...)
}

// InfoTTS logs TTS stage message
func (l *Logger) InfoTTS(msg string, args ...interface{}) {
	prefixedMsg := "[TTS] " + msg
	l.Info(prefixedMsg, args...)
}

// InfoTiming logs timing metrics message
func (l *Logger) InfoTiming(msg string, args ...interface{}) {
	prefixedMsg := "[TIMING] " + msg
	l.Info(prefixedMsg, args...)
}
