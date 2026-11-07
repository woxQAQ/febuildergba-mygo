package applog

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"
)

// Entry is a detached snapshot. Mutating returned attributes cannot change logs.
type Entry struct {
	// Sequence identifies receipt order across ring-buffer eviction.
	Sequence  uint64
	Time      time.Time
	Level     slog.Level
	Message   string
	Operation string
	Attrs     []slog.Attr
}

// RingBuffer retains a fixed number of recent entries in receipt order.
// All writes and snapshots are safe across workers and the UI thread.
type RingBuffer struct {
	mu          sync.RWMutex
	entries     []Entry
	next, count int
	sequence    uint64
}

// Sequence returns the last received sequence without copying entries.
func (b *RingBuffer) Sequence() uint64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.sequence
}

// Capacity is the maximum number of entries retained by this buffer.
func (b *RingBuffer) Capacity() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.entries)
}

func (b *RingBuffer) Snapshot() []Entry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	entries := make([]Entry, b.count)
	for i := range entries {
		entry := b.entries[(b.next-b.count+i+len(b.entries))%len(b.entries)]
		entry.Attrs = cloneAttrs(entry.Attrs)
		entries[i] = entry
	}
	return entries
}

type bufferHandler struct {
	buffer *RingBuffer
	level  slog.Leveler
	attrs  []slog.Attr
	groups []string
}

func (h *bufferHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *bufferHandler) Handle(_ context.Context, record slog.Record) error {
	attrs := make([]slog.Attr, 0, record.NumAttrs())
	record.Attrs(
		func(attr slog.Attr) bool { attrs = append(attrs, attr); return true },
	)
	attrs = appendScoped(slices.Clone(h.attrs), h.groups, cloneAttrs(attrs))
	entry := Entry{
		Time:    record.Time,
		Level:   record.Level,
		Message: record.Message,
		Attrs:   attrs,
	}
	for _, attr := range attrs {
		if attr.Key == "operation" && attr.Value.Kind() == slog.KindString {
			entry.Operation = attr.Value.String()
		}
	}
	b := h.buffer
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.entries) > 0 {
		b.sequence++
		entry.Sequence = b.sequence
		b.entries[b.next] = entry
		b.next = (b.next + 1) % len(b.entries)
		if b.count < len(b.entries) {
			b.count++
		}
	}
	return nil
}

func (h *bufferHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = appendScoped(
		slices.Clone(h.attrs),
		h.groups,
		cloneAttrs(attrs),
	)
	return &next
}

func (h *bufferHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	next := *h
	next.groups = append(slices.Clone(h.groups), name)
	return &next
}

func appendScoped(
	current []slog.Attr,
	groups []string,
	attrs []slog.Attr,
) []slog.Attr {
	if len(attrs) == 0 {
		return current
	}
	if len(groups) == 0 {
		return append(current, attrs...)
	}
	last := len(current) - 1
	if last >= 0 && current[last].Key == groups[0] &&
		current[last].Value.Kind() == slog.KindGroup {
		children := appendScoped(
			slices.Clone(current[last].Value.Group()),
			groups[1:],
			attrs,
		)
		current[last].Value = slog.GroupValue(children...)
		return current
	}
	children := appendScoped(nil, groups[1:], attrs)
	return append(
		current,
		slog.Attr{Key: groups[0], Value: slog.GroupValue(children...)},
	)
}

func cloneAttrs(attrs []slog.Attr) []slog.Attr {
	result := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		attr.Value = attr.Value.Resolve()
		switch attr.Value.Kind() {
		case slog.KindGroup:
			children := cloneAttrs(attr.Value.Group())
			if len(children) == 0 {
				continue
			}
			if attr.Key == "" {
				result = append(result, children...)
				continue
			}
			attr.Value = slog.GroupValue(children...)
		case slog.KindAny:
			// Resolve mutable values now, then detach them again for each reader.
			encoded, err := json.Marshal(attr.Value.Any())
			var value any
			if err == nil {
				err = json.Unmarshal(encoded, &value)
			}
			if err != nil {
				attr.Value = slog.StringValue(fmt.Sprint(attr.Value.Any()))
			} else {
				attr.Value = slog.AnyValue(value)
			}
		}
		if !attr.Equal(slog.Attr{}) {
			result = append(result, attr)
		}
	}
	return result
}
