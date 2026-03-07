package logs

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/kronveil/kronveil/core/engine"
)

// Config holds the log collector configuration.
type Config struct {
	Sources        []LogSource   `yaml:"sources" json:"sources"`
	ErrorPatterns  []string      `yaml:"error_patterns" json:"error_patterns"`
	ParseFormat    string        `yaml:"parse_format" json:"parse_format"` // "json", "logfmt", "raw"
	BufferSize     int           `yaml:"buffer_size" json:"buffer_size"`
	PollInterval   time.Duration `yaml:"poll_interval" json:"poll_interval"`
}

// LogSource defines a log collection source.
type LogSource struct {
	Name string `yaml:"name" json:"name"`
	Type string `yaml:"type" json:"type"` // "file", "k8s_container", "syslog"
	Path string `yaml:"path" json:"path"`
}

// Collector gathers and parses logs from various sources.
type Collector struct {
	config       Config
	events       chan *engine.TelemetryEvent
	errorRegexes []*regexp.Regexp
	mu           sync.RWMutex
	running      bool
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// New creates a new log collector.
func New(config Config) *Collector {
	if config.BufferSize == 0 {
		config.BufferSize = 500
	}
	if len(config.ErrorPatterns) == 0 {
		config.ErrorPatterns = []string{
			`(?i)error`,
			`(?i)fatal`,
			`(?i)panic`,
			`(?i)exception`,
			`(?i)oom`,
			`(?i)out of memory`,
			`(?i)killed`,
		}
	}

	var regexes []*regexp.Regexp
	for _, pattern := range config.ErrorPatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			regexes = append(regexes, re)
		}
	}

	return &Collector{
		config:       config,
		events:       make(chan *engine.TelemetryEvent, config.BufferSize),
		errorRegexes: regexes,
	}
}

func (c *Collector) Name() string { return "logs" }

func (c *Collector) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("log collector already running")
	}
	c.running = true
	_, c.cancel = context.WithCancel(ctx)
	c.mu.Unlock()

	log.Printf("[log-collector] Starting log collector (%d sources, %d error patterns)",
		len(c.config.Sources), len(c.config.ErrorPatterns))

	for _, source := range c.config.Sources {
		switch source.Type {
		case "file":
			c.wg.Add(1)
			go c.tailFile(ctx, source)
		}
	}

	return nil
}

func (c *Collector) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		return nil
	}
	c.running = false
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	close(c.events)
	return nil
}

func (c *Collector) Events() <-chan *engine.TelemetryEvent { return c.events }

func (c *Collector) Health() engine.ComponentHealth {
	return engine.ComponentHealth{
		Name:      "log-collector",
		Status:    "healthy",
		Message:   fmt.Sprintf("%d sources configured", len(c.config.Sources)),
		LastCheck: time.Now(),
	}
}

func (c *Collector) tailFile(ctx context.Context, source LogSource) {
	defer c.wg.Done()

	file, err := os.Open(source.Path)
	if err != nil {
		log.Printf("[log-collector] Cannot open %s: %v", source.Path, err)
		return
	}
	defer file.Close()

	// Seek to end of file for tail behavior.
	if _, err := file.Seek(0, 2); err != nil {
		log.Printf("[log-collector] Cannot seek %s: %v", source.Path, err)
	}

	scanner := bufio.NewScanner(file)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for scanner.Scan() {
				line := scanner.Text()
				c.processLine(source, line)
			}
		}
	}
}

func (c *Collector) processLine(source LogSource, line string) {
	severity := engine.SeverityInfo
	for _, re := range c.errorRegexes {
		if re.MatchString(line) {
			severity = engine.SeverityHigh
			break
		}
	}

	payload := map[string]interface{}{
		"raw":    line,
		"source": source.Name,
	}

	// Attempt structured parsing.
	if c.config.ParseFormat == "json" || strings.HasPrefix(strings.TrimSpace(line), "{") {
		var structured map[string]interface{}
		if err := json.Unmarshal([]byte(line), &structured); err == nil {
			payload["structured"] = structured
			if level, ok := structured["level"].(string); ok {
				severity = mapLogLevel(level)
			}
		}
	}

	event := &engine.TelemetryEvent{
		ID:        fmt.Sprintf("log-%d", time.Now().UnixNano()),
		Source:    "logs",
		Type:      "log_line",
		Timestamp: time.Now(),
		Payload:   payload,
		Metadata:  map[string]string{"collector": "logs", "log_source": source.Name},
		Severity:  severity,
	}
	select {
	case c.events <- event:
	default:
	}
}

func mapLogLevel(level string) string {
	switch strings.ToLower(level) {
	case "fatal", "critical", "emergency":
		return engine.SeverityCritical
	case "error", "err":
		return engine.SeverityHigh
	case "warn", "warning":
		return engine.SeverityMedium
	case "info":
		return engine.SeverityInfo
	default:
		return engine.SeverityInfo
	}
}
