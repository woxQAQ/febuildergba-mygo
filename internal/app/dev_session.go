package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
)

// developmentSession keeps ROM paths across developer rebuilds. Only path
// snapshots cross the worker boundary; ROM documents remain owned by windows.
type developmentSession struct {
	path    string
	updates chan []string
	done    chan struct{}
}

func newDevelopmentSession(path string, enabled bool) *developmentSession {
	if !enabled {
		return nil
	}
	session := &developmentSession{
		path:    path,
		updates: make(chan []string, 1),
		done:    make(chan struct{}),
	}
	go func() {
		defer close(session.done)
		for paths := range session.updates {
			logError(session.write(paths))
		}
	}()
	return session
}

func (session *developmentSession) startupPaths(
	explicit []string,
) ([]string, error) {
	if len(explicit) > 0 {
		return slices.Clone(explicit), nil
	}
	return session.restore()
}

func (session *developmentSession) restore() ([]string, error) {
	if session == nil {
		return nil, nil
	}
	content, err := os.ReadFile(
		session.path,
	) // #nosec G304 -- Application-owned developer cache.
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fault.Wrap(
			err,
			"app.restore_dev_session",
			fault.Context{Path: session.path},
		)
	}
	var state struct {
		ROMs []string `json:"roms"`
	}
	if err := json.Unmarshal(content, &state); err != nil {
		return nil, fault.Wrap(
			err,
			"app.restore_dev_session",
			fault.Context{Path: session.path},
		)
	}
	return state.ROMs, nil
}

func (session *developmentSession) remember(paths []string) {
	if session == nil || len(paths) == 0 {
		return
	}
	snapshot := slices.Clone(paths)
	slices.Sort(snapshot)
	snapshot = slices.Compact(snapshot)
	select {
	case session.updates <- snapshot:
	default:
		select {
		case <-session.updates:
		default:
		}
		session.updates <- snapshot
	}
}

func (session *developmentSession) close() {
	if session != nil {
		close(session.updates)
		<-session.done
	}
}

func (session *developmentSession) write(paths []string) (err error) {
	defer func() { err = fault.Wrap(err, "app.remember_dev_session", fault.Context{Path: session.path}) }()
	state := struct {
		ROMs []string `json:"roms"`
	}{paths}
	content, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	directory := filepath.Dir(session.path)
	err = os.MkdirAll(directory, 0o700)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".dev-session-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer func() { _ = os.Remove(temporary) }()
	_, err = file.Write(append(content, '\n'))
	if err != nil {
		_ = file.Close()
		return err
	}
	err = file.Close()
	if err != nil {
		return err
	}
	return os.Rename(temporary, session.path)
}

func (m *application) rememberROMs() {
	var paths []string
	for _, page := range m.windows {
		if page.Editor.Session != nil {
			paths = append(paths, page.Editor.Session.Document.State().Path)
		}
	}
	m.development.remember(paths)
}
