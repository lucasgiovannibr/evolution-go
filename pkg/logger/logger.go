package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	"github.com/gomessguii/logger"
	"gopkg.in/natefinch/lumberjack.v2"
)

type LoggerManager struct {
	config  *config.Config
	loggers map[string]*Logger
	// released instances no longer get a file logger (see Release).
	released map[string]struct{}
	// discard is the shared console-only logger handed out for released instances.
	discard *Logger
	mu      sync.RWMutex
}

type Logger struct {
	config     *config.Config
	instanceId string
	mu         sync.Mutex
	writer     *lumberjack.Logger
}

type LogEntry struct {
	Timestamp  time.Time       `json:"timestamp"`
	Level      string          `json:"level"`
	InstanceId string          `json:"instance_id"`
	Message    string          `json:"message"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
}

func NewLoggerManager(config *config.Config) *LoggerManager {
	// Garante que o diretório base de logs existe
	if err := os.MkdirAll(config.LogDirectory, 0755); err != nil {
		logger.LogError("Falha ao criar diretório base de logs: %v", err)
	}

	return &LoggerManager{
		config:   config,
		loggers:  make(map[string]*Logger),
		released: make(map[string]struct{}),
		discard:  &Logger{config: config, instanceId: "released"},
	}
}

// Release frees the file logger of an instance that no longer exists: it closes the
// log file (one descriptor per instance was kept open for the life of the process)
// and forgets the logger. Later calls for that instance are still printed to the
// console but no longer create a file or a new logger.
//
// Known limit: the rotating-file library (lumberjack v2) starts a background
// goroutine per logger that it has no way to stop, so that one goroutine per
// instance ever created remains until the process exits.
func (lm *LoggerManager) Release(instanceId string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if l, ok := lm.loggers[instanceId]; ok {
		_ = l.Close()
		delete(lm.loggers, instanceId)
	}
	lm.released[instanceId] = struct{}{}
}

func (lm *LoggerManager) GetLogger(instanceId string) *Logger {
	lm.mu.RLock()
	logger, exists := lm.loggers[instanceId]
	_, isReleased := lm.released[instanceId]
	lm.mu.RUnlock()

	if exists {
		return logger
	}
	if isReleased {
		return lm.discard
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Verificar novamente após obter o lock de escrita
	if logger, exists = lm.loggers[instanceId]; exists {
		return logger
	}
	if _, isReleased = lm.released[instanceId]; isReleased {
		return lm.discard
	}

	// Criar novo logger para a instância
	logger = newLogger(instanceId, lm.config)
	lm.loggers[instanceId] = logger
	return logger
}

func newLogger(instanceId string, config *config.Config) *Logger {
	// Garante que o diretório existe
	logPath := filepath.Join(config.LogDirectory, instanceId)
	os.MkdirAll(logPath, 0755)

	logFile := filepath.Join(logPath, "instance.log")

	writer := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    config.LogMaxSize,
		MaxBackups: config.LogMaxBackups,
		MaxAge:     config.LogMaxAge,
		Compress:   config.LogCompress,
	}

	return &Logger{
		config:     config,
		instanceId: instanceId,
		writer:     writer,
	}
}

func (l *Logger) LogInfo(format string, args ...interface{}) {
	l.log("INFO", format, args...)
	logger.LogInfo(format, args...)
}

func (l *Logger) LogError(format string, args ...interface{}) {
	l.log("ERROR", format, args...)
	logger.LogError(format, args...)
}

func (l *Logger) LogWarn(format string, args ...interface{}) {
	l.log("WARN", format, args...)
	logger.LogWarn(format, args...)
}

func (l *Logger) LogDebug(format string, args ...interface{}) {
	l.log("DEBUG", format, args...)
	logger.LogDebug(format, args...)
}

func (l *Logger) log(level string, format string, args ...interface{}) {
	if l.writer == nil { // released instance: console only
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry := LogEntry{
		Timestamp:  time.Now(),
		Level:      level,
		InstanceId: l.instanceId,
		Message:    fmt.Sprintf(format, args...),
	}

	jsonEntry, err := json.Marshal(entry)
	if err != nil {
		logger.LogError("Failed to marshal log entry: %v", err)
		return
	}

	if _, err := l.writer.Write(append(jsonEntry, '\n')); err != nil {
		logger.LogError("Failed to write log: %v", err)
	}
}

func (l *Logger) Close() error {
	if l.writer == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.writer.Close()
}

// GetLogs retorna os logs da instância com filtros opcionais
func (l *Logger) GetLogs(startDate, endDate time.Time, level string, limit int) ([]LogEntry, error) {
	// Implementação movida para o service
	return nil, fmt.Errorf("método movido para instance_service")
}
