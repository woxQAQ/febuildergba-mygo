// Package app coordinates native windows, menus, dialogs and external
// processes.
package app

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/applog"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

type application struct {
	windows     map[*mygo.Window]*pages.Workspace
	hexWindows  map[*mygo.Window]*hexEditorWindow
	development *developmentSession
	logBuffer   *applog.RingBuffer
	logWindow   *mygo.Window
}

func Run(paths []string) (err error) {
	dev := mygo.IsDev()
	sink := applog.New(applog.FromEnv(dev))
	previous := slog.Default()
	slog.SetDefault(sink.Logger)
	defer func() {
		if err != nil {
			slog.Error("app exited", "error", fault.LogValue(err))
		}
		applog.RecordError("app.close_log", sink.Close())
		slog.SetDefault(previous)
	}()
	slog.Info("app started", "dev", dev, "log_dir", sink.LogDir)
	m := &application{
		windows:    make(map[*mygo.Window]*pages.Workspace),
		hexWindows: make(map[*mygo.Window]*hexEditorWindow),
		logBuffer:  sink.Buffer,
	}
	if dev {
		directory, err := os.UserCacheDir()
		if err != nil {
			applog.RecordError("app.dev_cache", err)
		} else {
			m.development = newDevelopmentSession(
				filepath.Join(
					directory,
					"febuildergba-mygo",
					"dev-session.json",
				),
				true,
			)
		}
	}
	defer m.development.close()
	remembered, err := m.development.startupPaths(paths)
	applog.RecordError("app.restore_dev_session", err)
	if err == nil {
		paths = remembered
	}
	mygo.App.WhenReady(func() {
		m.setMenu()
		mygo.App.OnLocaleChanged(func(string) {
			m.setMenu()
			for win, page := range m.windows {
				updateTitle(win, page)
			}
			for owner, tool := range m.hexWindows {
				updateHexTitle(tool.window, m.windows[owner])
				tool.window.Invalidate()
			}
			if m.logWindow != nil {
				m.logWindow.SetTitle(locales.LogViewer())
				m.logWindow.Invalidate()
			}
		})
		for _, path := range paths {
			session, err := game.Open(path)
			if err != nil {
				applog.RecordError("app.open", err)
				continue
			}
			logROMOpened(session, false)
			m.newWindow(session)
		}
		if len(m.windows) == 0 {
			m.newWindow(nil)
		}
	})
	return mygo.App.Run()
}

func logROMOpened(session *game.Session, reloaded bool) {
	state := session.Document.State()
	slog.Info("rom opened", "path", state.Path, "version", session.Version(),
		"size", state.Size, "revision", state.Revision, "reloaded", reloaded)
}

func logROMSaved(state rom.State, saveAs bool) {
	slog.Info("rom saved", "path", state.Path, "size", state.Size,
		"revision", state.Revision, "save_as", saveAs)
}

func setExternal(a *pages.Workspace, changed bool) {
	if changed && !a.Editor.External {
		state := a.Editor.Session.Document.State()
		slog.Warn(
			"external change",
			"path",
			state.Path,
			"revision",
			state.Revision,
		)
	}
	a.Editor.SetExternal(changed)
}
