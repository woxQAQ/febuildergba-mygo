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
	"github.com/woxQAQ/febuildergba-mygo/internal/pages"
)

func logError(err error) {
	if err != nil {
		slog.Error("operation failed", "error", fault.LogValue(err))
	}
}

type application struct {
	windows     map[*mygo.Window]*pages.Workspace
	development *developmentSession
}

func Run(paths []string) error {
	m := &application{windows: make(map[*mygo.Window]*pages.Workspace)}
	if mygo.IsDev() {
		directory, err := os.UserCacheDir()
		if err != nil {
			logError(err)
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
	logError(err)
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
		})
		for _, path := range paths {
			session, err := game.Open(path)
			if err != nil {
				logError(err)
				continue
			}
			m.newWindow(session)
		}
		if len(m.windows) == 0 {
			m.newWindow(nil)
		}
	})
	return mygo.App.Run()
}
