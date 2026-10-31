package game_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestOpenFailureCanBeDiagnosedAndRecovered(t *testing.T) {
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "missing.gba")
	session, err := game.Open(path)
	if session != nil || fault.CodeOf(err).Kind != fault.NotFound ||
		!errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing file classification: %v", err)
	}
	pathErr, ok := errors.AsType[*fs.PathError](err)
	if !ok || pathErr.Path != path {
		t.Fatalf("lost OS error: %v", err)
	}
	frame, ok := errors.AsType[*fault.Error](err)
	if !ok || frame.Op != "game.open" || frame.Context.Path != path {
		t.Fatalf("missing open context: %v", err)
	}

	// Inject a secondary cleanup failure at the reporting boundary. Both
	// causes must remain diagnosable without changing the recovery decision.
	cleanup := &fs.PathError{
		Op:   "remove",
		Path: path + ".tmp",
		Err:  fs.ErrPermission,
	}
	report := fault.Wrap(errors.Join(err, cleanup), "app.open", fault.Context{})
	if fault.CodeOf(report) != fault.CodeOf(err) ||
		!errors.Is(
			report,
			fs.ErrNotExist,
		) || !errors.Is(report, fs.ErrPermission) {
		t.Fatalf(
			"report lost a cause or replaced the primary error: %v",
			report,
		)
	}
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	logger.Error("open failed", "error", fault.LogValue(report))
	type diagnostic struct {
		Code      string                 `json:"code"`
		Kind      fault.Kind             `json:"kind"`
		Operation string                 `json:"operation"`
		Path      string                 `json:"path"`
		Cause     *diagnostic            `json:"cause"`
		Causes    map[string]*diagnostic `json:"causes"`
	}
	var entry struct {
		Error diagnostic `json:"error"`
	}
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Error.Code != "fault.not_found" ||
		entry.Error.Kind != fault.NotFound ||
		entry.Error.Operation != "app.open" ||
		entry.Error.Cause == nil {
		t.Fatalf("missing report classification: %s", &output)
	}
	causes := entry.Error.Cause.Causes
	primary, secondary := causes["0"], causes["1"]
	if len(causes) != 2 || primary == nil || secondary == nil ||
		primary.Operation != "game.open" || primary.Path != path ||
		primary.Code != "fault.not_found" ||
		secondary.Operation != "remove" || secondary.Path != cleanup.Path ||
		secondary.Code != "fault.permission" || secondary.Kind != fault.Permission {
		t.Fatalf("report omitted structured causes: %s", &output)
	}

	// An unclassified primary cause must not inherit a secondary cause's code.
	report = fault.Wrap(
		errors.Join(io.ErrUnexpectedEOF, err),
		"app.open",
		fault.Context{},
	)
	if fault.CodeOf(report).Kind != fault.Unknown ||
		!errors.Is(
			report,
			io.ErrUnexpectedEOF,
		) || !errors.Is(report, fs.ErrNotExist) {
		t.Fatalf("unknown primary failure was reclassified: %v", report)
	}

	data := testutil.NewMinimalFE8U().Bytes()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	session, err = game.Open(path)
	if err != nil {
		t.Fatalf("open did not recover after restoring the input: %v", err)
	}
	if session.Document.State().Dirty ||
		!bytes.Equal(data, session.Document.Snapshot().Data) {
		t.Fatal("recovered open changed the input")
	}
}
