package app

import (
	"os"

	"github.com/egoist/mygo"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/locales"
	"github.com/woxQAQ/febuildergba-mygo/pages"
)

func (m *application) openDialog(win *mygo.Window, a *pages.Workspace) {
	if a.Editor.Busy {
		return
	}
	a.Editor.Busy = true
	title := locales.Open()
	go func() {
		paths, err := mygo.Dialog.Open(
			mygo.OpenDialogOptions{
				Parent: win,
				Title:  title,
				Filters: []mygo.FileFilter{
					{Name: "Game Boy Advance ROM", Extensions: []string{"gba"}},
				},
			},
		)
		var session *game.Session
		if err == nil && len(paths) > 0 {
			session, err = game.Open(paths[0])
		}
		win.Update(func() {
			a.Editor.Busy = false
			a.Editor.Err = err
			if session == nil {
				return
			}
			for existing, state := range m.windows {
				if state.Editor.Session != nil &&
					state.Editor.Session.Document.State().Path == session.Document.State().Path {
					existing.Focus()
					return
				}
			}
			if a.Editor.Session == nil {
				a.SetSession(session)
				updateTitle(win, a)
			} else {
				m.newWindow(session)
			}
		})
	}()
}

func saveDocument(win *mygo.Window, a *pages.Workspace, as bool) {
	if a.Editor.Session == nil || a.Editor.Busy {
		return
	}
	doc := a.Editor.Session.Document
	title, replaceQuestion, replace, cancel := locales.SaveAs(), locales.ReplaceQuestion(), locales.Replace(), locales.Cancel()
	background(win, a, func() error {
		if !as {
			return doc.Save()
		}
		path, err := mygo.Dialog.Save(
			mygo.SaveDialogOptions{
				Parent:      win,
				Title:       title,
				DefaultPath: doc.State().Path,
				Filters: []mygo.FileFilter{
					{Name: "Game Boy Advance ROM", Extensions: []string{"gba"}},
				},
			},
		)
		if err != nil || path == "" {
			return err
		}
		overwrite := false
		if _, err := os.Stat(path); err == nil && path != doc.State().Path {
			res, err := mygo.Dialog.Message(
				mygo.MessageOptions{
					Parent:       win,
					Type:         mygo.MessageQuestion,
					Message:      replaceQuestion,
					Detail:       path,
					Buttons:      []string{replace, cancel},
					CancelButton: 1,
				},
			)
			if err != nil {
				return err
			}
			if res.Button != 0 {
				return nil
			}
			overwrite = true
		}
		return doc.SaveAs(path, overwrite)
	})
}

func reloadDocument(win *mygo.Window, a *pages.Workspace) {
	if a.Editor.Session == nil || a.Editor.Busy {
		return
	}
	a.Editor.Busy = true
	state := a.Editor.Session.Document.State()
	question, reload, cancel := locales.ReloadQuestion(), locales.Reload(), locales.Cancel()
	go func() {
		var err error
		load := true
		if state.Dirty {
			var res mygo.MessageResult
			res, err = mygo.Dialog.Message(
				mygo.MessageOptions{
					Parent:       win,
					Type:         mygo.MessageQuestion,
					Message:      question,
					Buttons:      []string{reload, cancel},
					CancelButton: 1,
				},
			)
			load = err == nil && res.Button == 0
		}
		var session *game.Session
		if load {
			session, err = game.Open(state.Path)
		}
		win.Update(func() {
			a.Editor.Busy = false
			a.Editor.Err = err
			if session != nil {
				a.SetSession(session)
			}
		})
	}()
}
