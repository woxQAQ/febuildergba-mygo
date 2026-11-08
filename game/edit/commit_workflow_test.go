package edit_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

func TestCapturedCommitLifecycle(t *testing.T) {
	data := bytes.Repeat([]byte{0xCC}, 129)
	copy(data[4:12], []byte{0x20, 0, 0, 8, 0x20, 0, 0, 8})
	doc, context := capturedDocument(t, data)
	context.Interpret("automatic")
	input := context.Capture()
	owner := romwrite.Owner{Resource: "resource"}
	claims := []romwrite.Claim{{Start: 4, End: 8, Owner: owner, Site: true}}
	builder := romwrite.NewBuilder(input.Data, romwrite.Protection{
		Pointer: func(_ []byte, owner romwrite.Owner, site int) error {
			return romwrite.CheckPointerWrite(claims, owner, site)
		},
	})
	var encoded []byte
	if _, err := builder.ReplaceEncoded(func(at int) ([]byte, error) {
		encoded = binary.LittleEndian.AppendUint32(
			nil,
			testutil.PointerWord(at+4),
		)
		return append(encoded, 0xA1, 0xA2, 0xA3, 0xA4), nil
	}, romwrite.PointerSite{Owner: owner, Address: 4, ExpectedWord: 0x08000020}); err != nil {
		t.Fatal(err)
	}
	plan := builder.Plan()
	commit := edit.NewCommit(input.Stamp(), "replace", plan.Edit)
	clear(encoded)
	for _, change := range plan.Edit.Changes {
		clear(change.Data)
	}
	before := doc.State()
	_, other := capturedDocument(t, data)
	other.Interpret("automatic")
	if err := commit.Apply(other); !errors.Is(err, rom.ErrStale) {
		t.Fatalf(
			"commit accepted another document at the same revision: %v",
			err,
		)
	}
	context.Interpret("manual")
	if commit.Current(context) || input.Stamp().Current(context) {
		t.Fatal("capture followed a later interpretation")
	}
	if err := commit.Apply(
		context,
	); !errors.Is(err, rom.ErrStale) ||
		doc.State() != before {
		t.Fatalf("invalid interpretation changed the document: %v", err)
	}
	context.Interpret("automatic")
	if !commit.Current(context) {
		t.Fatal("restoring the same interpretation did not restore validity")
	}
	if err := commit.Apply(context); err != nil {
		t.Fatal(err)
	}
	want := append(
		bytes.Clone(data),
		0xFF,
		0xFF,
		0xFF,
		0x88,
		0,
		0,
		8,
		0xA1,
		0xA2,
		0xA3,
		0xA4,
	)
	copy(want[4:8], []byte{0x84, 0, 0, 8})
	if !bytes.Equal(doc.Snapshot().Data, want) ||
		!bytes.Equal(input.Data, data) {
		t.Fatal("frozen write, independent reference or captured input differs")
	}
	if !doc.Undo() || !bytes.Equal(doc.Snapshot().Data, data) ||
		doc.State().CanUndo {
		t.Fatal("one undo did not restore the original document")
	}
	before = doc.State()
	if err := commit.Apply(
		context,
	); !errors.Is(err, rom.ErrStale) ||
		doc.State() != before {
		t.Fatalf(
			"old capture became valid after undo restored its bytes: %v",
			err,
		)
	}
	if !doc.Redo() || !bytes.Equal(doc.Snapshot().Data, want) {
		t.Fatal("redo did not restore the complete replacement")
	}
	latest := context.Capture()
	if latest.Stamp().Revision() != doc.State().Revision ||
		!latest.Stamp().Current(context) ||
		!bytes.Equal(latest.Data, want) {
		t.Fatal("new input did not capture matching bytes and revision")
	}
}

func TestCapturedRawEditIgnoresOnlyInterpretation(t *testing.T) {
	data := bytes.Repeat([]byte{0xCC}, 64)
	doc, context := capturedDocument(t, data)
	context.Interpret("first")
	interpreted := context.Capture()
	raw := context.CaptureBytes()
	editor := edit.NewBytes(context)
	region, err := editor.Read(20, 2)
	if err != nil {
		t.Fatal(err)
	}
	context.Interpret("second")
	if interpreted.Stamp().Current(context) || !raw.Stamp().Current(context) {
		t.Fatal(
			"byte-only and interpreted captures lost their different scopes",
		)
	}
	commit := edit.NewCommit(raw.Stamp(), "raw", rom.Edit{
		Size: len(
			raw.Data,
		),
		Changes: []rom.Change{{Offset: 20, Data: []byte{1, 2}}},
	})
	if err := commit.Apply(context); err != nil {
		t.Fatal(err)
	}
	before := doc.State()
	if err := editor.Apply(
		region,
		[]byte{3, 4},
	); !errors.Is(err, rom.ErrStale) ||
		doc.State() != before {
		t.Fatalf("raw draft ignored a byte revision change: %v", err)
	}
	context.Interpret("first")
	if interpreted.Stamp().Current(context) || raw.Stamp().Current(context) {
		t.Fatal("restoring interpretation bypassed byte revision checks")
	}
	if !doc.Undo() || !bytes.Equal(doc.Snapshot().Data, data) {
		t.Fatal("raw edit did not use document history")
	}
}

func capturedDocument(
	t *testing.T,
	data []byte,
) (*rom.Document, *edit.Context) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "capture.gba")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	document, err := rom.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return document, edit.New(document, func([]byte) bool { return true })
}
