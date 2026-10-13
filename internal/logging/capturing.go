package logging

import (
	"context"
	"sync"
	"time"
)

// CapturingLogger records log calls for test verification and local diagnostics.
type CapturingLogger struct {
	mu      sync.Mutex
	records []Record
}

// Record represents a single captured log call.
type Record struct {
	Level   string
	Message string
	Fields  map[string]any
	Elapsed time.Time
}

func (c *CapturingLogger) DebugContext(_ context.Context, msg string, args ...any) {
	c.append("debug", msg, args)
}
func (c *CapturingLogger) InfoContext(_ context.Context, msg string, args ...any) {
	c.append("info", msg, args)
}
func (c *CapturingLogger) WarnContext(_ context.Context, msg string, args ...any) {
	c.append("warn", msg, args)
}
func (c *CapturingLogger) ErrorContext(_ context.Context, msg string, args ...any) {
	c.append("error", msg, args)
}

func (c *CapturingLogger) append(level, msg string, args []any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fields := make(map[string]any)
	for i := 0; i+1 < len(args); i += 2 {
		if k, ok := args[i].(string); ok {
			fields[k] = args[i+1]
		}
	}
	c.records = append(c.records, Record{Level: level, Message: msg, Fields: fields, Elapsed: time.Now()})
}

// Records returns a snapshot of all captured records.
func (c *CapturingLogger) Records() []Record {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Record(nil), c.records...)
}

// Last returns the most recent record or nil.
func (c *CapturingLogger) Last() *Record {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.records) == 0 {
		return nil
	}
	record := c.records[len(c.records)-1]
	return &record
}
