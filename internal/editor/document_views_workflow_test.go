package editor_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/editor"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func TestDocumentViewsShareEditsAndKeepHistoryTargets(t *testing.T) {
	session := testutil.Open(t, testutil.NewMinimalFE8U())
	workspace := editor.New()
	workspace.SetSession(session)
	tool := workspace.NewView()
	before := session.Document.Snapshot()
	region, err := session.Bytes.Read(0x200, 2)
	if err != nil {
		t.Fatal(err)
	}
	replacement := region.Bytes()
	replacement[0] ^= 0xFF
	if err := tool.Edit(
		func() error { return tool.Session.Bytes.Apply(region, replacement) },
	); err != nil {
		t.Fatal(err)
	}
	if !workspace.Session.Document.State().Dirty {
		t.Fatal("tool edit did not reach the workspace document")
	}

	workspace.ResolveHistoryFocus(true)
	draftUndone := false
	workspace.DraftHistory = &editor.History{
		Undo: func() { draftUndone = true },
	}
	tool.ResolveHistoryFocus(false)
	epoch := workspace.HistoryEpoch
	tool.Undo()
	if draftUndone || !workspace.TextFocused || tool.TextFocused ||
		!bytes.Equal(session.Document.Snapshot().Data, before.Data) ||
		workspace.HistoryEpoch != epoch+1 || tool.HistoryEpoch != workspace.HistoryEpoch {
		t.Fatal("tool undo crossed window focus or lost shared history state")
	}
	tool.Redo()
	if !bytes.Equal(
		session.Document.Snapshot().Data[0x200:0x202],
		replacement,
	) ||
		workspace.HistoryEpoch != epoch+2 {
		t.Fatal("tool redo did not refresh shared document history")
	}
	workspace.ResolveHistoryFocus(false)
	workspace.Undo()
	if !draftUndone || !session.Document.State().Dirty {
		t.Fatal("workspace draft history fell through to the document")
	}

	notified := false
	workspace.Changed = func() { notified = true }
	workspace.SetBusy(true)
	if !notified || !tool.Busy || tool.History().Undo != nil ||
		tool.History().Redo != nil {
		t.Fatal("workspace operation did not block tool document commands")
	}
	tool.SetBusy(false)
	workspace.SetExternal(true)
	workspace.SetErr(rom.ErrExternalChange)
	if !tool.External || !errors.Is(tool.Err, rom.ErrExternalChange) {
		t.Fatal("operation diagnostics are not shared across document views")
	}

	reopened := testutil.Open(t, testutil.NewMinimalFE8U())
	workspace.SetSession(reopened)
	if tool.Session != reopened || tool.Err != nil || tool.External ||
		tool.HistoryEpoch != workspace.HistoryEpoch {
		t.Fatal("tool retained the old document after reloading the workspace")
	}
	if err := tool.Session.Bytes.Apply(
		region,
		replacement,
	); !errors.Is(
		err,
		rom.ErrStale,
	) ||
		reopened.Document.State().Dirty {
		t.Fatalf("old byte draft changed the reloaded document: %v", err)
	}
}
