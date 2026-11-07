package applog_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/woxQAQ/febuildergba-mygo/internal/applog"
)

func TestRecentLogViewerStream(t *testing.T) {
	sink := applog.New(
		applog.Config{
			LogDir:         t.TempDir(),
			Level:          slog.LevelDebug,
			BufferCapacity: 3,
			Stderr:         io.Discard,
		},
	)
	defer sink.Close()
	stamp := time.Date(2026, 7, 3, 12, 34, 56, 0, time.UTC)
	record := slog.NewRecord(stamp, slog.LevelWarn, "operation failed", 0)
	record.AddAttrs(
		slog.String("operation", "app.open"),
		slog.Group(
			"error",
			slog.String("code", "rom.invalid_header"),
			slog.Group("cause", slog.String("detail", "truncated")),
		),
	)
	if err := sink.Logger.Handler().
		Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	first := sink.Buffer.Snapshot()
	if len(first) != 1 || first[0].Sequence != 1 ||
		sink.Buffer.Sequence() != 1 ||
		sink.Buffer.Capacity() != 3 {
		t.Fatal("initial log stream", first)
	}
	var encoded map[string]any
	if err := json.Unmarshal(first[0].JSON(), &encoded); err != nil {
		t.Fatal(err)
	}
	if encoded["time"] != "2026-07-03T12:34:56Z" ||
		encoded["level"] != "WARN" ||
		encoded["msg"] != "operation failed" ||
		encoded["operation"] != "app.open" {
		t.Fatal("copied log lost its envelope", encoded)
	}
	if _, found := encoded["Sequence"]; found {
		t.Fatal("internal identity entered the log format")
	}
	root := encoded["error"].(map[string]any)
	if root["code"] != "rom.invalid_header" ||
		root["cause"].(map[string]any)["detail"] != "truncated" {
		t.Fatal("copied log lost error causes", root)
	}
	disk := readLog(t, filepath.Join(sink.LogDir, "febuildergba.jsonl"))
	diskJSON, _ := json.Marshal(disk[0])
	copyJSON, _ := json.Marshal(encoded)
	if string(diskJSON) != string(copyJSON) {
		t.Fatal(
			"viewer and persistent record differ",
			string(diskJSON),
			string(copyJSON),
		)
	}
	first[0].Attrs[1].Value.Group()[0] = slog.String("code", "reader mutation")
	var fresh map[string]any
	if err := json.Unmarshal(
		sink.Buffer.Snapshot()[0].JSON(),
		&fresh,
	); err != nil {
		t.Fatal(err)
	}
	if fresh["error"].(map[string]any)["code"] != "rom.invalid_header" {
		t.Fatal("reader changed captured error")
	}
	left := sink.Logger.With("path", "first.gba")
	right := sink.Logger.With("path", "second.gba")
	left.Info("rom opened")
	right.Info("rom opened")
	left.Debug("document history", "action", "apply")
	right.Error("operation failed", "operation", "app.save")
	recent := sink.Buffer.Snapshot()
	if len(recent) != 3 || recent[0].Sequence != 3 || recent[2].Sequence != 5 ||
		sink.Buffer.Sequence() != 5 {
		t.Fatal("receipt identity did not survive eviction", recent)
	}
	if first[0].Sequence != 1 || first[0].Message != "operation failed" {
		t.Fatal("detached selection changed after eviction")
	}
	for _, entry := range recent {
		var value map[string]any
		if err := json.Unmarshal(entry.JSON(), &value); err != nil {
			t.Fatal(err)
		}
		if value["path"] == nil {
			t.Fatal("window context lost", value)
		}
	}
}
