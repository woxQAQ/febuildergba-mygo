// Package app coordinates native windows, menus, dialogs and external processes.
package app

import (
	"log"

	"github.com/woxQAQ/febuildergba-mygo/internal/components"

	"github.com/egoist/mygo"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/pages"
)

type application struct {
	windows map[*mygo.Window]*pages.Workspace
}

func Run(paths []string) error {
	m := &application{windows: make(map[*mygo.Window]*pages.Workspace)}
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
				log.Printf("open ROM: %q", components.ErrorText(err))
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
