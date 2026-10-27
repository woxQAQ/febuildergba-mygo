package rom_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

func TestSaveFailureCanBeDiagnosedAndRecovered(t *testing.T) {
	path := filepath.Join(t.TempDir(), "document.gba")
	original := make([]byte, 4096)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	doc, err := rom.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	path = doc.State().Path
	if err := doc.Apply(
		doc.State().Revision,
		"edit",
		[]rom.Change{{Offset: 0x200, Data: []byte{0xAA}}},
	); err != nil {
		t.Fatal(err)
	}
	before := doc.Snapshot()

	// A regular file occupying the backup directory prevents publication.
	backupPath := path + ".backups"
	if err := os.WriteFile(backupPath, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	err = doc.Save()
	if fault.CodeOf(err).Kind != fault.IO {
		t.Fatalf("backup failure category: %v", err)
	}
	pathErr, ok := errors.AsType[*fs.PathError](err)
	if !ok || pathErr.Path != backupPath {
		t.Fatalf("lost backup filesystem cause: %v", err)
	}
	var backup bool
	for frame := range fault.Frames(err) {
		if frame.Op == "rom.backup" && frame.Context.Path == path {
			backup = true
		}
	}
	if !backup {
		t.Fatalf("backup stage is missing: %v", err)
	}
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(onDisk, original) || !doc.State().Dirty ||
		doc.State().Revision != before.Revision || !bytes.Equal(doc.Snapshot().Data, before.Data) {
		t.Fatal("failed save changed the file or checkpoint")
	}
	leftovers, err := filepath.Glob(
		filepath.Join(filepath.Dir(path), ".febuilder-save-*"),
	)
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("failed save left temporary files: %v, %v", leftovers, err)
	}
	if err := os.Remove(backupPath); err != nil {
		t.Fatal(err)
	}
	if err := doc.Save(); err != nil {
		t.Fatalf("save did not recover after fixing backup directory: %v", err)
	}
	if doc.State().Dirty {
		t.Fatal("successful retry did not publish the checkpoint")
	}
	backups, err := filepath.Glob(filepath.Join(backupPath, "*.gba"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("missing backup: %v, %v", backups, err)
	}
	backupData, err := os.ReadFile(backups[0])
	if err != nil || !bytes.Equal(backupData, original) {
		t.Fatalf("backup lost original data: %v", err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	err = doc.Save()
	if fault.CodeOf(err) != rom.ErrExternalChange ||
		!errors.Is(err, rom.ErrExternalChange) ||
		!errors.Is(err, fs.ErrNotExist) {
		t.Fatalf(
			"deleted source must report a conflict while retaining ENOENT: %v",
			err,
		)
	}
	recoveredPath := filepath.Join(filepath.Dir(path), "recovered.gba")
	if err := doc.SaveAs(recoveredPath, false); err != nil {
		t.Fatal(err)
	}
	reopened, err := rom.Open(recoveredPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(reopened.Snapshot().Data, before.Data) ||
		doc.State().Dirty {
		t.Fatal("save-as recovery lost the edited snapshot")
	}
}
