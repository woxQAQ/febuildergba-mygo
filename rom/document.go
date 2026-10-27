// Package rom owns ROM bytes, edit history and atomic saves with backups.
package rom

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/woxQAQ/febuildergba-mygo/fault"
)

const MaxSize = 32 << 20

var (
	ErrRange          = fault.Code{ID: "rom.range", Kind: fault.InvalidInput}
	ErrStale          = fault.Code{ID: "rom.stale_edit", Kind: fault.Conflict}
	ErrExternalChange = fault.Code{
		ID:   "rom.external_change",
		Kind: fault.Conflict,
	}
	ErrExists = fault.Code{
		ID:   "rom.target_exists",
		Kind: fault.Conflict,
	}
	ErrReadOnly = fault.Code{ID: "rom.read_only", Kind: fault.Permission}
	ErrSize     = fault.Code{
		ID:   "rom.invalid_size",
		Kind: fault.InvalidData,
	}
)

// Change replaces a range in a document edit.
type Change struct {
	Offset int
	Data   []byte
}

// Edit commits writes and optional expansion together. New bytes are filled
// with FF.
// Size must be at least the current size; shrinking is reserved for undo.
type Edit struct {
	Size    int
	Changes []Change
}

type delta struct {
	offset        int
	before, after []byte
}
type operation struct {
	label                 string
	beforeID, afterID     uint64
	beforeSize, afterSize int
	changes               []delta
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

func Open(path string) (doc *Document, err error) {
	ctx := fault.Context{Path: path}
	defer func() { err = fault.Wrap(err, "rom.open", ctx) }()
	path, err = filepath.Abs(path)
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
	defer func() { err = fault.Wrap(err, "rom.read", fault.Context{Path: path}) }()
	// #nosec G304 -- ROM paths are explicitly selected by the user via a dialog
	// or CLI.
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer func() { err = errors.Join(err, fault.Wrap(f.Close(), "rom.close", fault.Context{Path: path})) }()
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
	s := State{
		Path:     d.path,
		Size:     len(d.data),
		Revision: d.revision,
		Dirty:    d.stateID != d.savedID,
		CanUndo:  d.cursor > 0,
		CanRedo:  d.cursor < len(d.history),
	}
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

// Apply replaces existing bytes without changing the document size.
func (d *Document) Apply(
	revision uint64,
	label string,
	changes []Change,
) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.apply(revision, label, Edit{Size: len(d.data), Changes: changes})
}

// ApplyEdit validates the complete edit before changing bytes, size or history.
func (d *Document) ApplyEdit(revision uint64, label string, edit Edit) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.apply(revision, label, edit)
}

func (d *Document) apply(revision uint64, label string, edit Edit) error {
	if revision != d.revision {
		return fault.Wrap(
			ErrStale,
			"rom.apply",
			fault.Context{
				Revision: &fault.Revision{
					Expected: revision,
					Actual:   d.revision,
				},
			},
		)
	}
	if edit.Size < len(d.data) || edit.Size > MaxSize || edit.Size < 1 {
		return fault.Wrap(
			ErrSize,
			"rom.apply",
			fault.Context{
				Range: &fault.Range{Length: edit.Size, Size: len(d.data)},
			},
		)
	}
	ordered := append([]Change(nil), edit.Changes...)
	slices.SortFunc(ordered, func(a, b Change) int {
		return cmp.Compare(a.Offset, b.Offset)
	})
	op := operation{
		label:      label,
		beforeID:   d.stateID,
		beforeSize: len(d.data),
		afterSize:  edit.Size,
	}
	end := 0
	for _, c := range ordered {
		if c.Offset < end || c.Offset < 0 || c.Offset > edit.Size ||
			len(c.Data) > edit.Size-c.Offset {
			return fault.Wrap(
				ErrRange,
				"rom.apply",
				fault.Context{
					Range: &fault.Range{
						Offset: c.Offset,
						Length: len(c.Data),
						Size:   edit.Size,
					},
				},
			)
		}
		end = c.Offset + len(c.Data)
		before := d.data[min(c.Offset, len(d.data)):min(end, len(d.data))]
		if !bytes.Equal(before, c.Data) {
			op.changes = append(
				op.changes,
				delta{c.Offset, bytes.Clone(before), bytes.Clone(c.Data)},
			)
		}
	}
	if len(op.changes) == 0 && op.beforeSize == op.afterSize {
		return nil
	}
	d.nextID++
	op.afterID = d.nextID
	d.expand(op.afterSize)
	for _, c := range op.changes {
		copy(d.data[c.offset:], c.after)
	}
	d.history = append(d.history[:d.cursor], op)
	d.cursor++
	d.stateID = op.afterID
	d.revision++
	return nil
}

func (d *Document) expand(size int) {
	d.data = append(d.data, bytes.Repeat([]byte{0xFF}, size-len(d.data))...)
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
	d.data = d.data[:op.beforeSize]
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
	d.expand(op.afterSize)
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
		return true, fault.Wrap(
			err,
			"rom.check_external",
			fault.Context{Path: path},
		)
	}
	return sha256.Sum256(data) != expected, nil
}

func (d *Document) Save() error {
	d.saveMu.Lock()
	defer d.saveMu.Unlock()
	d.mu.RLock()
	path := d.path
	d.mu.RUnlock()
	return fault.Wrap(
		d.save(path, false),
		"rom.save",
		fault.Context{Path: path},
	)
}

// SaveAs requires explicit consent to replace a different existing target.
func (d *Document) SaveAs(path string, replace bool) (err error) {
	ctx := fault.Context{Path: path}
	defer func() { err = fault.Wrap(err, "rom.save_as", ctx) }()
	d.saveMu.Lock()
	defer d.saveMu.Unlock()
	path, err = filepath.Abs(path)
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
	// The saved checkpoint belongs to these bytes, even if edits continue
	// during I/O.
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
			return fault.New(
				ErrExternalChange,
				"rom.check_external",
				fault.Context{Path: path},
				err,
			)
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
		return fault.Wrap(err, "rom.create_temp", fault.Context{Path: path})
	}
	tmpPath := tmp.Name()
	defer func() {
		if cleanupErr := os.Remove(
			tmpPath,
		); cleanupErr != nil &&
			!errors.Is(cleanupErr, os.ErrNotExist) {
			err = errors.Join(
				err,
				fault.Wrap(
					cleanupErr,
					"rom.cleanup",
					fault.Context{Path: tmpPath},
				),
			)
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
	// Recheck immediately before replacement; an external editor may have saved
	// during I/O.
	latest, _, checkErr := readFile(path)
	if exists {
		if checkErr != nil || !bytes.Equal(latest, previous) {
			return fault.New(
				ErrExternalChange,
				"rom.recheck",
				fault.Context{Path: path},
				checkErr,
			)
		}
	} else if !errors.Is(checkErr, os.ErrNotExist) {
		return fault.New(
			ErrExternalChange,
			"rom.recheck",
			fault.Context{Path: path},
			checkErr,
		)
	}
	if err = os.Rename(tmpPath, path); err != nil {
		return fault.Wrap(err, "rom.replace", fault.Context{Path: path})
	}
	publish()
	return nil
}

func backupFile(path string, data []byte) (err error) {
	defer func() { err = fault.Wrap(err, "rom.backup", fault.Context{Path: path}) }()
	backupDir := path + ".backups"
	if err = os.MkdirAll(backupDir, 0o700); err != nil {
		return err
	}
	backup, err := os.CreateTemp(
		backupDir,
		time.Now().UTC().Format("20060102T150405.000000000")+"-*.gba",
	)
	if err != nil {
		return err
	}
	if err = writeSync(backup, data, 0o600); err != nil {
		return errors.Join(
			err,
			fault.Wrap(
				os.Remove(backup.Name()),
				"rom.cleanup",
				fault.Context{Path: backup.Name()},
			),
		)
	}
	return nil
}

func writeSync(f *os.File, data []byte, mode os.FileMode) (err error) {
	ctx := fault.Context{Path: f.Name()}
	defer func() { err = errors.Join(err, fault.Wrap(f.Close(), "rom.close", ctx)) }()
	if err = f.Chmod(mode); err != nil {
		return fault.Wrap(err, "rom.chmod", ctx)
	}
	if _, err = f.Write(data); err != nil {
		return fault.Wrap(err, "rom.write", ctx)
	}
	return fault.Wrap(f.Sync(), "rom.sync", ctx)
}
