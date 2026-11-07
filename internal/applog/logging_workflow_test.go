package applog_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/slogtest"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/internal/applog"
)

func TestDiagnosticSession(t *testing.T) {
	for _, scenario := range []struct {
		name, setting string
		dev, debug    bool
	}{
		{name: "production"},
		{name: "development", dev: true, debug: true},
		{name: "explicit debug", setting: "debug", debug: true},
		{name: "explicit info", dev: true, setting: "info"},
		{name: "invalid production", setting: "verbose"},
		{name: "invalid development", setting: "verbose", dev: true, debug: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Setenv("FEBUILDER_LOG_DIR", t.TempDir())
			t.Setenv("FEBUILDER_LOG_LEVEL", scenario.setting)
			config := applog.FromEnv(scenario.dev)
			var console bytes.Buffer
			config.Stderr = &console
			sink := applog.New(config)
			t.Cleanup(func() { _ = sink.Close() })
			previous := slog.Default()
			slog.SetDefault(sink.Logger)
			t.Cleanup(func() { slog.SetDefault(previous) })

			applog.RecordError("app.cancel", context.Canceled)
			applog.RecordError("app.success", nil)
			kinds := []fault.Kind{
				fault.InvalidInput, fault.Conflict, fault.Unsupported,
				fault.InvalidData, fault.IO, fault.Capacity, fault.Permission,
				fault.NotFound, fault.Unknown,
			}
			for _, kind := range kinds {
				applog.RecordError(
					"app."+string(kind),
					fault.Code{ID: "sample." + string(kind), Kind: kind},
				)
			}
			joined := fault.Wrap(errors.Join(
				fault.New(
					fault.Code{ID: "sample.corrupt", Kind: fault.InvalidData},
					"game.decode",
					fault.Context{RecordID: new(0)},
					errors.New("truncated"),
				),
				&os.PathError{
					Op:   "remove",
					Path: "temporary.gba",
					Err:  os.ErrPermission,
				},
			), "app.import", fault.Context{Path: "sample.gba"})
			applog.RecordError("app.import", joined)

			records := readLog(
				t,
				filepath.Join(sink.LogDir, "febuildergba.jsonl"),
			)
			want := len(kinds) + 1
			if !scenario.debug {
				want--
			}
			if len(records) != want || len(sink.Buffer.Snapshot()) != want {
				t.Fatalf(
					"record counts: file=%d memory=%d want=%d",
					len(records),
					len(sink.Buffer.Snapshot()),
					want,
				)
			}
			for _, record := range records {
				operation := record["operation"].(string)
				wantLevel := "ERROR"
				switch operation {
				case "app.invalid_input":
					wantLevel = "DEBUG"
				case "app.conflict", "app.unsupported":
					wantLevel = "WARN"
				}
				if record["level"] != wantLevel ||
					record["msg"] != "operation failed" ||
					!strings.Contains(console.String(), operation) {
					t.Fatalf(
						"unexpected diagnostic: %#v; console=%s",
						record,
						console.String(),
					)
				}
			}
			root := records[len(records)-1]["error"].(map[string]any)
			causes := root["cause"].(map[string]any)["causes"].(map[string]any)
			primary, secondary := causes["0"].(map[string]any), causes["1"].(map[string]any)
			if root["operation"] != "app.import" ||
				root["path"] != "sample.gba" ||
				primary["record_id"] != float64(0) ||
				primary["code"] != "sample.corrupt" ||
				primary["cause"].(map[string]any)["detail"] != "truncated" ||
				secondary["path"] != "temporary.gba" ||
				secondary["operation"] != "remove" {
				t.Fatalf("lost error tree: %#v", root)
			}
			entries := sink.Buffer.Snapshot()
			memory := entryAttrs(t, entries[len(entries)-1])
			encodedFile, _ := json.Marshal(root)
			encodedMemory, _ := json.Marshal(memory["error"])
			if !bytes.Equal(encodedFile, encodedMemory) ||
				entries[len(entries)-1].Operation != "app.import" {
				t.Fatalf(
					"memory and disk disagree: file=%s memory=%s",
					encodedFile,
					encodedMemory,
				)
			}
		})
	}
}

func TestStructuredDestinations(t *testing.T) {
	sink := applog.New(
		applog.Config{
			LogDir: t.TempDir(),
			Level:  slog.LevelDebug,
			Stderr: io.Discard,
		},
	)
	defer sink.Close()
	if err := slogtest.TestHandler(
		sink.Logger.Handler(),
		func() []map[string]any {
			entries := sink.Buffer.Snapshot()
			result := make([]map[string]any, len(entries))
			for index, entry := range entries {
				result[index] = entryAttrs(t, entry)
			}
			return result
		},
	); err != nil {
		t.Fatal(err)
	}
}

func TestPersistentRetention(t *testing.T) {
	directory := t.TempDir()
	config := applog.Config{
		LogDir:      directory,
		MaxFileSize: 300,
		Stderr:      io.Discard,
	}
	sink := applog.New(config)
	sink.Logger.Info(strings.Repeat("x", 120), "sequence", 0)
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	// Reopening must include the existing file's length in the rotation decision.
	sink = applog.New(config)
	t.Cleanup(func() { _ = sink.Close() })
	for sequence := 1; sequence < 8; sequence++ {
		sink.Logger.Info(strings.Repeat("x", 120), "sequence", sequence)
	}
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(directory)
	if err != nil || len(files) != 4 {
		t.Fatalf("retained files: %v %v", files, err)
	}
	for generation := range 4 {
		path := filepath.Join(directory, "febuildergba.jsonl")
		if generation > 0 {
			path += "." + strconv.Itoa(generation)
		}
		records := readLog(t, path)
		info, err := os.Stat(path)
		if err != nil || info.Size() > 300 || len(records) != 1 ||
			records[0]["sequence"] != float64(7-generation) {
			t.Fatalf(
				"generation %d: records=%v info=%v err=%v",
				generation,
				records,
				info,
				err,
			)
		}
	}

	// A large record stays whole; the next record moves it into history.
	sink = applog.New(config)
	large := strings.Repeat("z", 600)
	sink.Logger.Info(large)
	sink.Logger.Info("after large record")
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	records := readLog(t, filepath.Join(directory, "febuildergba.jsonl.1"))
	if len(records) != 1 || records[0]["msg"] != large {
		t.Fatalf("large record was split or lost: %v", records)
	}
}

func TestUnavailablePersistence(t *testing.T) {
	t.Run("directory blocked", func(t *testing.T) {
		blocked := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(blocked, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		var console bytes.Buffer
		sink := applog.New(
			applog.Config{
				LogDir: filepath.Join(blocked, "logs"),
				Level:  slog.LevelError,
				Stderr: &console,
			},
		)
		defer sink.Close()
		sink.Logger.Error("still available", "operation", "app.open")
		entries := sink.Buffer.Snapshot()
		if sink.LogDir != "" || len(entries) != 2 ||
			entries[0].Level != slog.LevelWarn ||
			entries[1].Message != "still available" ||
			strings.Count(console.String(), "log file unavailable") != 1 {
			t.Fatalf(
				"fallback lost diagnostics: directory=%q entries=%v console=%s",
				sink.LogDir,
				entries,
				console.String(),
			)
		}
	})
	t.Run("cache resolution failed", func(t *testing.T) {
		t.Setenv("HOME", "")
		t.Setenv("XDG_CACHE_HOME", "")
		if _, err := os.UserCacheDir(); err == nil {
			t.Skip(
				"platform cache resolution does not require a home directory",
			)
		}
		sink := applog.New(applog.Config{Stderr: io.Discard})
		defer sink.Close()
		sink.Logger.Info("cache fallback")
		if sink.LogDir != "" || len(sink.Buffer.Snapshot()) != 2 {
			t.Fatalf(
				"cache failure interrupted memory logs: %v",
				sink.Buffer.Snapshot(),
			)
		}
	})
	t.Run("rotation blocked", func(t *testing.T) {
		directory := t.TempDir()
		var console bytes.Buffer
		sink := applog.New(
			applog.Config{LogDir: directory, MaxFileSize: 1, Stderr: &console},
		)
		defer sink.Close()
		sink.Logger.Info("before failure")
		blocked := filepath.Join(directory, "febuildergba.jsonl.3")
		if err := os.Mkdir(blocked, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			filepath.Join(blocked, "keep"),
			nil,
			0o600,
		); err != nil {
			t.Fatal(err)
		}
		sink.Logger.Info("rotation failed")
		sink.Logger.Info("after failure")
		entries := sink.Buffer.Snapshot()
		if len(entries) != 4 || entries[3].Message != "after failure" ||
			strings.Count(console.String(), "log file unavailable") != 1 {
			t.Fatalf(
				"runtime fallback failed: entries=%v console=%s",
				entries,
				console.String(),
			)
		}
	})
}

func TestConcurrentDiagnosticSnapshots(t *testing.T) {
	sink := applog.New(
		applog.Config{
			LogDir:         t.TempDir(),
			BufferCapacity: 32,
			Stderr:         io.Discard,
		},
	)
	defer sink.Close()
	metadata := map[string]any{"name": "original"}
	logger := sink.Logger.With("metadata", metadata).
		WithGroup("request").
		With("origin", "worker")
	logger.Info(
		"captured",
		"operation",
		"nested",
		"value",
		map[string]any{"count": 1},
	)
	metadata["name"] = "changed"
	captured := sink.Buffer.Snapshot()[0]
	attrs := entryAttrs(t, captured)
	if attrs["metadata"].(map[string]any)["name"] != "original" ||
		attrs["request"].(map[string]any)["origin"] != "worker" ||
		attrs["request"].(map[string]any)["value"].(map[string]any)["count"] != float64(
			1,
		) {
		t.Fatalf("mutable attributes leaked: %v", attrs)
	}
	captured.Attrs[0].Value.Any().(map[string]any)["name"] = "reader mutation"
	if entryAttrs(t, sink.Buffer.Snapshot()[0])["metadata"].(map[string]any)["name"] != "original" {
		t.Fatal("snapshot mutation changed the ring")
	}

	var workers sync.WaitGroup
	for worker := range 8 {
		workers.Go(func() {
			for index := range 80 {
				logger.Info("worker", "worker", worker, "index", index)
			}
		})
	}
	for range 3 {
		workers.Go(func() {
			for range 120 {
				var previous uint64
				for _, entry := range sink.Buffer.Snapshot() {
					if entry.Sequence <= previous {
						t.Error("concurrent snapshot lost receipt order")
					}
					previous = entry.Sequence
					if entry.Message == "worker" {
						entry.Attrs[0].Value.Any().(map[string]any)["name"] = "isolated reader"
					}
				}
			}
		})
	}
	workers.Wait()
	for sequence := range 33 {
		sink.Logger.Info("tail", "operation", "app.tail", "sequence", sequence)
	}
	entries := sink.Buffer.Snapshot()
	if len(entries) != 32 {
		t.Fatalf("ring size=%d", len(entries))
	}
	sequence := entries[0].Sequence
	for index, entry := range entries {
		if entry.Sequence != sequence {
			t.Fatal("ring sequence changed across eviction", entry.Sequence)
		}
		sequence++
		if entry.Operation != "app.tail" || entry.Time.IsZero() ||
			entryAttrs(t, entry)["sequence"] != float64(index+1) {
			t.Fatalf("ring order at %d: %v", index, entry)
		}
	}
	if entries[len(entries)-1].Sequence != sink.Buffer.Sequence() {
		t.Fatal("last sequence does not match the recent stream")
	}
}

func readLog(t *testing.T, path string) []map[string]any {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	var records []map[string]any
	for {
		var record map[string]any
		err := decoder.Decode(&record)
		if errors.Is(err, io.EOF) {
			return records
		}
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
}

func entryAttrs(t *testing.T, entry applog.Entry) map[string]any {
	t.Helper()
	var buffer bytes.Buffer
	record := slog.NewRecord(entry.Time, entry.Level, entry.Message, 0)
	record.AddAttrs(entry.Attrs...)
	if err := slog.NewJSONHandler(&buffer, nil).
		Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	var attrs map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &attrs); err != nil {
		t.Fatal(err)
	}
	return attrs
}
