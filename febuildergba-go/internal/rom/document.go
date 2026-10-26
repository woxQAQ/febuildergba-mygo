// Package rom owns ROM bytes, edit history and atomic saves with backups.
package rom

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const MaxSize = 32 << 20

var (
	ErrRange          = errors.New("rom.range")
	ErrStale          = errors.New("rom.stale_edit")
	ErrExternalChange = errors.New("rom.external_change")
	ErrExists         = errors.New("rom.target_exists")
	ErrReadOnly       = errors.New("rom.read_only")
	ErrSize           = errors.New("rom.invalid_size")
)

// Change replaces a range without relocating data. Expansion is a separate operation.
type Change struct {
	Offset int
	Data   []byte
}

type delta struct {
	offset        int
	before, after []byte
}
type operation struct {
	label             string
	beforeID, afterID uint64
	changes           []delta
}

type Snapshot struct {
	Revision uint64
	Data     []byte
}

type State struct {
	Path                    string
	Size                    int
	Revision                uint64
	Dirty, CanUndo, CanRedo bool
	UndoLabel, RedoLabel    string
}

// Document owns the bytes and history. Callers only receive copies.
type Document struct {
	mu                                 sync.RWMutex
	saveMu                             sync.Mutex
	data                               []byte
	path                               string
	diskHash                           [32]byte
	history                            []operation
	cursor                             int
	revision, stateID, nextID, savedID uint64
}

func Open(path string) (*Document, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	data, _, err := readFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, ErrSize
	}
	return &Document{data: data, path: path, diskHash: sha256.Sum256(data)}, nil
}

func readFile(path string) (data []byte, mode os.FileMode, err error) {
	// #nosec G304 -- ROM paths are explicitly selected by the user via a dialog or CLI.
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	info, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxSize {
		return nil, 0, ErrSize
	}
	data, err = io.ReadAll(io.LimitReader(f, MaxSize+1))
	if err != nil {
		return nil, 0, err
	}
	if len(data) > MaxSize {
		return nil, 0, ErrSize
	}
	return data, info.Mode().Perm(), nil
}

func (d *Document) State() State {
	d.mu.RLock()
	defer d.mu.RUnlock()
	s := State{Path: d.path, Size: len(d.data), Revision: d.revision, Dirty: d.stateID != d.savedID, CanUndo: d.cursor > 0, CanRedo: d.cursor < len(d.history)}
	if s.CanUndo {
		s.UndoLabel = d.history[d.cursor-1].label
	}
	if s.CanRedo {
		s.RedoLabel = d.history[d.cursor].label
	}
	return s
}

func (d *Document) Snapshot() Snapshot {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return Snapshot{Revision: d.revision, Data: bytes.Clone(d.data)}
}

// Apply validates every range before changing any bytes. Overlapping writes are ambiguous.
func (d *Document) Apply(revision uint64, label string, changes []Change) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if revision != d.revision {
		return ErrStale
	}
	ordered := append([]Change(nil), changes...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Offset < ordered[j].Offset })
	op := operation{label: label, beforeID: d.stateID}
	end := 0
	for _, c := range ordered {
		if c.Offset < end || c.Offset < 0 || c.Offset > len(d.data) || len(c.Data) > len(d.data)-c.Offset {
			return fmt.Errorf("%w: 0x%X + %d", ErrRange, c.Offset, len(c.Data))
		}
		end = c.Offset + len(c.Data)
		before := d.data[c.Offset:end]
		if !bytes.Equal(before, c.Data) {
			op.changes = append(op.changes, delta{c.Offset, bytes.Clone(before), bytes.Clone(c.Data)})
		}
	}
	if len(op.changes) == 0 {
		return nil
	}
	d.nextID++
	op.afterID = d.nextID
	for _, c := range op.changes {
		copy(d.data[c.offset:], c.after)
	}
	d.history = append(d.history[:d.cursor], op)
	d.cursor++
	d.stateID = op.afterID
	d.revision++
	return nil
}

func (d *Document) Undo() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cursor == 0 {
		return false
	}
	d.cursor--
	op := d.history[d.cursor]
	for _, c := range op.changes {
		copy(d.data[c.offset:], c.before)
	}
	d.stateID = op.beforeID
	d.revision++
	return true
}

func (d *Document) Redo() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cursor == len(d.history) {
		return false
	}
	op := d.history[d.cursor]
	for _, c := range op.changes {
		copy(d.data[c.offset:], c.after)
	}
	d.cursor++
	d.stateID = op.afterID
	d.revision++
	return true
}

func (d *Document) ExternalChanged() (bool, error) {
	d.saveMu.Lock()
	defer d.saveMu.Unlock()
	d.mu.RLock()
	path, expected := d.path, d.diskHash
	d.mu.RUnlock()
	data, _, err := readFile(path)
	if err != nil {
		return true, err
	}
	return sha256.Sum256(data) != expected, nil
}

func (d *Document) Save() error {
	d.saveMu.Lock()
	defer d.saveMu.Unlock()
	d.mu.RLock()
	path := d.path
	d.mu.RUnlock()
	return d.save(path, false)
}

// SaveAs requires explicit consent to replace a different existing target.
func (d *Document) SaveAs(path string, replace bool) error {
	d.saveMu.Lock()
	defer d.saveMu.Unlock()
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(path); err == nil {
		path, err = filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
	}
	return d.save(path, replace)
}

func (d *Document) save(path string, replace bool) (err error) {
	// The saved checkpoint belongs to these bytes, even if edits continue during I/O.
	d.mu.RLock()
	data := bytes.Clone(d.data)
	originalPath, expectedHash, savedID := d.path, d.diskHash, d.stateID
	d.mu.RUnlock()
	dataHash := sha256.Sum256(data)
	publish := func() {
		d.mu.Lock()
		d.path, d.diskHash, d.savedID = path, dataHash, savedID
		d.mu.Unlock()
	}
	previous, mode, err := readFile(path)
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if path == originalPath {
		if !exists || sha256.Sum256(previous) != expectedHash {
			return ErrExternalChange
		}
	} else if exists && !replace {
		return ErrExists
	}
	if exists && mode&0o222 == 0 {
		return ErrReadOnly
	}
	if exists && bytes.Equal(previous, data) {
		publish()
		return nil
	}
	if !exists {
		mode = 0o644
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".febuilder-save-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if cleanupErr := os.Remove(tmpPath); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
			err = errors.Join(err, cleanupErr)
		}
	}()
	if err = writeSync(tmp, data, mode); err != nil {
		return err
	}
	if exists {
		if err = backupFile(path, previous); err != nil {
			return err
		}
	}
	// Recheck immediately before replacement; an external editor may have saved during I/O.
	latest, _, checkErr := readFile(path)
	if exists {
		if checkErr != nil || !bytes.Equal(latest, previous) {
			return ErrExternalChange
		}
	} else if !errors.Is(checkErr, os.ErrNotExist) {
		return ErrExternalChange
	}
	if err = os.Rename(tmpPath, path); err != nil {
		return err
	}
	publish()
	return nil
}

func backupFile(path string, data []byte) error {
	backupDir := path + ".backups"
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return err
	}
	backup, err := os.CreateTemp(backupDir, time.Now().UTC().Format("20060102T150405.000000000")+"-*.gba")
	if err != nil {
		return err
	}
	if err = writeSync(backup, data, 0o600); err != nil {
		return errors.Join(err, os.Remove(backup.Name()))
	}
	return nil
}

func writeSync(f *os.File, data []byte, mode os.FileMode) (err error) {
	defer func() { err = errors.Join(err, f.Close()) }()
	if err = f.Chmod(mode); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}
