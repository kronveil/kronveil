package audit

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

// EventType identifies the kind of audit event.
type EventType string

const (
	EventAuth          EventType = "auth"
	EventIncident      EventType = "incident"
	EventRemediation   EventType = "remediation"
	EventPolicyChange  EventType = "policy_change"
	EventConfigChange  EventType = "config_change"
	EventSecretAccess  EventType = "secret_access"
	EventAPICall       EventType = "api_call"
)

// Entry represents a single audit log entry.
type Entry struct {
	ID        string            `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	Type      EventType         `json:"type"`
	Actor     string            `json:"actor"`
	Action    string            `json:"action"`
	Resource  string            `json:"resource"`
	Result    string            `json:"result"`
	Details   map[string]string `json:"details,omitempty"`
	SourceIP  string            `json:"source_ip,omitempty"`
}

// Store provides persistent audit logging with an in-memory buffer and file sink.
type Store struct {
	mu       sync.RWMutex
	entries  []Entry
	maxSize  int
	filePath string
	file     *os.File
	logger   *slog.Logger
}

// Config holds audit store configuration.
type Config struct {
	FilePath   string `yaml:"file_path" json:"file_path"`
	MaxEntries int    `yaml:"max_entries" json:"max_entries"`
}

// NewStore creates a new audit store.
func NewStore(config Config) (*Store, error) {
	if config.MaxEntries <= 0 {
		config.MaxEntries = 10000
	}

	s := &Store{
		entries: make([]Entry, 0, 256),
		maxSize: config.MaxEntries,
		logger:  slog.Default().With("component", "audit"),
	}

	if config.FilePath != "" {
		f, err := os.OpenFile(config.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return nil, fmt.Errorf("failed to open audit log file: %w", err)
		}
		s.file = f
		s.filePath = config.FilePath
	}

	return s, nil
}

// Log records an audit entry.
func (s *Store) Log(entry Entry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("audit-%d", time.Now().UnixNano())
	}

	s.mu.Lock()
	s.entries = append(s.entries, entry)
	if len(s.entries) > s.maxSize {
		s.entries = s.entries[len(s.entries)-s.maxSize:]
	}
	s.mu.Unlock()

	// Write to structured log.
	s.logger.Info("audit",
		"id", entry.ID,
		"type", entry.Type,
		"actor", entry.Actor,
		"action", entry.Action,
		"resource", entry.Resource,
		"result", entry.Result,
		"source_ip", entry.SourceIP,
	)

	// Write to file sink if configured.
	if s.file != nil {
		data, err := json.Marshal(entry)
		if err == nil {
			data = append(data, '\n')
			s.mu.Lock()
			_, _ = s.file.Write(data)
			s.mu.Unlock()
		}
	}
}

// Query returns audit entries matching the given filters.
func (s *Store) Query(eventType EventType, actor string, since time.Time, limit int) []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []Entry
	for i := len(s.entries) - 1; i >= 0; i-- {
		e := s.entries[i]
		if eventType != "" && e.Type != eventType {
			continue
		}
		if actor != "" && e.Actor != actor {
			continue
		}
		if !since.IsZero() && e.Timestamp.Before(since) {
			continue
		}
		results = append(results, e)
		if limit > 0 && len(results) >= limit {
			break
		}
	}
	return results
}

// Recent returns the most recent n audit entries.
func (s *Store) Recent(n int) []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if n <= 0 || n > len(s.entries) {
		n = len(s.entries)
	}
	start := len(s.entries) - n
	result := make([]Entry, n)
	copy(result, s.entries[start:])
	return result
}

// Close flushes and closes the audit store.
func (s *Store) Close() error {
	if s.file != nil {
		return s.file.Close()
	}
	return nil
}
