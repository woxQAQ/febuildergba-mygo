package applog

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

type rotatingFile struct {
	mu          sync.Mutex
	path        string
	file        *os.File
	size, limit int64
	generations int
	report      func(error)
	closed      bool
}

func openRotatingFile(
	directory string,
	limit int64,
	generations int,
	report func(error),
) (*rotatingFile, error) {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	writer := &rotatingFile{
		path:        filepath.Join(directory, "febuildergba.jsonl"),
		limit:       limit,
		generations: generations,
		report:      report,
	}
	if err := writer.open(); err != nil {
		return nil, err
	}
	return writer, nil
}

func (w *rotatingFile) open() error {
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return errors.Join(err, file.Close())
	}
	w.file, w.size = file, info.Size()
	return nil
}

func (w *rotatingFile) rotate() error {
	err := w.file.Close()
	w.file = nil
	if err != nil {
		return err
	}
	for generation := w.generations; generation >= 1; generation-- {
		source := w.path
		if generation > 1 {
			source += "." + strconv.Itoa(generation-1)
		}
		target := w.path + "." + strconv.Itoa(generation)
		if err := os.Remove(
			target,
		); err != nil &&
			!errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.Rename(
			source,
			target,
		); err != nil &&
			!errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return w.open()
}

func (w *rotatingFile) Write(data []byte) (int, error) {
	w.mu.Lock()
	if w.closed || w.file == nil {
		w.mu.Unlock()
		return len(data), nil
	}
	var err error
	// Preserve a single JSON record intact even when it alone exceeds the limit.
	if w.size > 0 && (w.size >= w.limit || int64(len(data)) > w.limit-w.size) {
		err = w.rotate()
	}
	if err == nil {
		var written int
		written, err = w.file.Write(data)
		w.size += int64(written)
		if err == nil && written != len(data) {
			err = io.ErrShortWrite
		}
	}
	if err != nil && w.file != nil {
		err = errors.Join(err, w.file.Close())
		w.file = nil
	}
	w.mu.Unlock()
	if err != nil {
		w.report(err)
	}
	// Disk failure disables this destination; other destinations keep working.
	return len(data), nil
}

func (w *rotatingFile) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}
