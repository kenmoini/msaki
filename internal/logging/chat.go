package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kenmoini/msaki/internal/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

// ChatLogger handles logging of chat messages based on collections
type ChatLogger struct {
	config      *config.ChatLogsConfig
	writers     map[string]*lumberjack.Logger
	collections map[string]*config.ChatLogCollection
	mu          sync.RWMutex
}

// ChatLogEntry represents a single chat log entry
type ChatLogEntry struct {
	Timestamp string      `json:"timestamp"`
	Model     string      `json:"model"`
	UserID    string      `json:"user_id"`
	Type      string      `json:"type"` // "request" or "response"
	Content   interface{} `json:"content"`
}

// NewChatLogger creates a new chat logger
func NewChatLogger(cfg *config.ChatLogsConfig) (*ChatLogger, error) {
	cl := &ChatLogger{
		config:      cfg,
		writers:     make(map[string]*lumberjack.Logger),
		collections: make(map[string]*config.ChatLogCollection),
	}

	// Index collections by model name for quick lookup
	for i := range cfg.Collections {
		collection := &cfg.Collections[i]
		for _, model := range collection.Models {
			cl.collections[model] = collection
		}
	}

	// Ensure log directory exists
	if cfg.LogDirectory != "" {
		if err := os.MkdirAll(cfg.LogDirectory, 0755); err != nil {
			return nil, err
		}
	}

	return cl, nil
}

// LogRequest logs a chat request if configured for the model
func (cl *ChatLogger) LogRequest(model, userID string, content interface{}) {
	cl.log(model, userID, "request", content)
}

// LogResponse logs a chat response if configured for the model
func (cl *ChatLogger) LogResponse(model, userID string, content interface{}) {
	cl.log(model, userID, "response", content)
}

func (cl *ChatLogger) log(model, userID, logType string, content interface{}) {
	if cl.config == nil || !cl.config.Enabled {
		return
	}

	// Find collection for this model
	cl.mu.RLock()
	collection, exists := cl.collections[model]
	cl.mu.RUnlock()

	if !exists {
		return
	}

	// Check if this log type is enabled for the collection
	if logType == "request" && !collection.Requests {
		return
	}
	if logType == "response" && !collection.Responses {
		return
	}

	// Get or create writer for this collection/model/user/date combination
	writer := cl.getWriter(collection, model, userID)
	if writer == nil {
		return
	}

	// Create log entry
	entry := ChatLogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Model:     model,
		UserID:    userID,
		Type:      logType,
		Content:   content,
	}

	// Write as JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	cl.mu.Lock()
	defer cl.mu.Unlock()
	writer.Write(append(data, '\n'))
}

func (cl *ChatLogger) getWriter(collection *config.ChatLogCollection, model, userID string) *lumberjack.Logger {
	// Resolve filename template
	filename := cl.resolveFilename(collection.Filename, model, userID)
	fullPath := filepath.Join(cl.config.LogDirectory, filename)

	// Use filename as key for writer cache
	cl.mu.Lock()
	defer cl.mu.Unlock()

	if writer, exists := cl.writers[fullPath]; exists {
		return writer
	}

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil
	}

	// Create new writer
	writer := &lumberjack.Logger{
		Filename:   fullPath,
		MaxSize:    100, // 100MB
		MaxBackups: 10,
		MaxAge:     90,
		Compress:   true,
	}

	cl.writers[fullPath] = writer
	return writer
}

func (cl *ChatLogger) resolveFilename(template, model, userID string) string {
	now := time.Now()

	replacements := map[string]string{
		"${MODEL}":    sanitizeFilename(model),
		"${USERID}":   sanitizeFilename(userID),
		"${DATE-ymd}": now.Format("2006-01-02"),
		"${DATE-ym}":  now.Format("2006-01"),
		"${DATE-y}":   now.Format("2006"),
	}

	result := template
	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}

// sanitizeFilename removes or replaces characters that are unsafe for filenames
func sanitizeFilename(s string) string {
	// Replace unsafe characters with underscores
	unsafe := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", " "}
	result := s
	for _, char := range unsafe {
		result = strings.ReplaceAll(result, char, "_")
	}
	return result
}

// Close closes all writers
func (cl *ChatLogger) Close() error {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	for _, writer := range cl.writers {
		writer.Close()
	}
	cl.writers = make(map[string]*lumberjack.Logger)
	return nil
}
