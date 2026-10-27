package game

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func TestEditingFailuresCarryRecoveryContext(t *testing.T) {
	s := openFixture(t, testutil.NewMinimalFE8U())
	before := s.Document.Snapshot()
	for _, sample := range []struct {
		ids    []int
		value  int64
		code   fault.Code
		record int
	}{
		{[]int{1, 999}, 20, ErrField, 999},
		{[]int{1, 2}, 128, ErrValue, 1},
	} {
		err := s.Set(Units, sample.ids, "base.hp", sample.value)
		frame, ok := errors.AsType[*fault.Error](err)
		if !ok || frame.Code != sample.code ||
			frame.Code.Kind != fault.InvalidInput ||
			frame.Op != "game.set" ||
			frame.Context.Table != string(Units) ||
			frame.Context.Field != "base.hp" ||
			frame.Context.RecordID == nil ||
			*frame.Context.RecordID != sample.record ||
			frame.Context.Value == nil ||
			*frame.Context.Value != sample.value {
			t.Fatalf("missing failed edit context: %v", err)
		}
		if sample.code == ErrValue && (frame.Context.Bounds == nil ||
			*frame.Context.Bounds != (fault.Bounds{Min: -128, Max: 127})) {
			t.Fatalf("missing field bounds: %v", err)
		}
		if s.Document.State().Revision != before.Revision ||
			s.Document.State().CanUndo ||
			!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
			t.Fatal("failed batch changed bytes or history")
		}
	}

	region, err := s.ReadRegion(0x200, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Set(Units, []int{1}, "base.hp", 20); err != nil {
		t.Fatal(err)
	}
	stable := s.Document.Snapshot()
	err = s.ApplyRegion(region, []byte{0xAA})
	if fault.CodeOf(err) != rom.ErrStale || !errors.Is(err, rom.ErrStale) {
		t.Fatalf("stale draft is not a conflict: %v", err)
	}
	var revisions *fault.Revision
	for frame := range fault.Frames(err) {
		if frame.Context.Revision != nil {
			revisions = frame.Context.Revision
		}
	}
	if revisions == nil || revisions.Expected != region.Revision() ||
		revisions.Actual != stable.Revision {
		t.Fatalf("missing revision evidence: %v", err)
	}
	if !bytes.Equal(stable.Data, s.Document.Snapshot().Data) {
		t.Fatal("stale draft changed bytes")
	}
	region, err = s.ReadRegion(0x200, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyRegion(region, []byte{0xAA}); err != nil {
		t.Fatalf("fresh draft could not recover: %v", err)
	}
}

func TestOpenFailureReportPreservesCauses(t *testing.T) {
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "missing.gba")
	_, err = Open(path)
	if fault.CodeOf(err).Kind != fault.NotFound ||
		!errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing file classification: %v", err)
	}
	if pathErr, ok := errors.AsType[*fs.PathError](
		err,
	); !ok ||
		pathErr.Path != path {
		t.Fatalf("lost OS error: %v", err)
	}
	frame, ok := errors.AsType[*fault.Error](err)
	if !ok || frame.Op != "game.open" || frame.Context.Path != path {
		t.Fatalf("missing open context: %v", err)
	}

	// A later cleanup failure must remain inspectable without taking over the
	// recovery decision for the original operation.
	cleanup := &fs.PathError{
		Op:   "remove",
		Path: path + ".tmp",
		Err:  fs.ErrPermission,
	}
	report := fault.Wrap(errors.Join(err, cleanup), "app.open", fault.Context{})
	if fault.CodeOf(report) != fault.CodeOf(err) ||
		!errors.Is(report, fs.ErrPermission) {
		t.Fatalf(
			"secondary failure replaced the primary error or was lost: %v",
			report,
		)
	}
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	logger.Error("open failed", "error", fault.LogValue(report))
	for _, detail := range []string{`"operation":"game.open"`, `"code":"fault.not_found"`, `"kind":"not_found"`, `"causes"`, "missing.gba.tmp", "permission denied"} {
		if !strings.Contains(output.String(), detail) {
			t.Fatalf("report omitted %s: %s", detail, &output)
		}
	}
	// Unknown primary errors must not inherit a known secondary error's code.
	report = fault.Wrap(
		errors.Join(io.ErrUnexpectedEOF, err),
		"app.open",
		fault.Context{},
	)
	if fault.CodeOf(report).Kind != fault.Unknown ||
		!errors.Is(report, io.ErrUnexpectedEOF) ||
		!errors.Is(report, fs.ErrNotExist) {
		t.Fatalf("unknown primary failure was reclassified: %v", report)
	}
}
