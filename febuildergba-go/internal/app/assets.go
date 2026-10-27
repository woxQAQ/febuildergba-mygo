package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"

	"github.com/woxQAQ/febuildergba-mygo/internal/game"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages"
)

func importPNG(win *mygo.Window, workspace *pages.Workspace, done func([]byte, error)) {
	if workspace.Editor.Busy || done == nil {
		return
	}
	workspace.Editor.Busy = true
	title := locales.ReplacePng()
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win, Title: title, Filters: []mygo.FileFilter{{Name: "PNG", Extensions: []string{"png"}}}})
		var content []byte
		if err == nil && len(paths) > 0 {
			content, err = readPNG(paths[0])
		}
		win.Update(func() {
			workspace.Editor.Busy = false
			done(content, err)
		})
	}()
}

func readPNG(path string) ([]byte, error) {
	file, err := os.Open(path) // #nosec G304 -- path is selected in the native import dialog.
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	content, err := io.ReadAll(io.LimitReader(file, game.MaxAssetPNG+1))
	if err != nil {
		return nil, err
	}
	if len(content) > game.MaxAssetPNG {
		return nil, game.ErrAssetSize
	}
	return content, nil
}

func (m *application) exportPNG(win *mygo.Window, workspace *pages.Workspace, ref game.AssetRef) {
	if workspace.Editor.Session == nil || workspace.Editor.Busy {
		return
	}
	content, err := workspace.Editor.Session.AssetPNG(ref)
	if err != nil {
		workspace.Editor.Err = err
		return
	}
	path := workspace.Editor.Session.Document.State().Path
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	defaultPath := filepath.Join(filepath.Dir(path), fmt.Sprintf("%s-%s-%04X.png", stem, ref.Kind, ref.ID))
	var romPaths []string
	for _, workspace := range m.windows {
		if workspace.Editor.Session != nil {
			romPaths = append(romPaths, workspace.Editor.Session.Document.State().Path)
		}
	}
	title, question, replace, cancel := locales.ExportPng(), locales.ReplaceQuestion(), locales.Replace(), locales.Cancel()
	background(win, workspace, func() error {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: win, Title: title, DefaultPath: defaultPath, Filters: []mygo.FileFilter{{Name: "PNG", Extensions: []string{"png"}}}})
		if err != nil || path == "" {
			return err
		}
		if !strings.EqualFold(filepath.Ext(path), ".png") {
			return errors.New("png_export_path")
		}
		existing, err := os.Stat(path)
		if err == nil {
			for _, romPath := range romPaths {
				romInfo, statErr := os.Stat(romPath)
				if statErr == nil && os.SameFile(existing, romInfo) {
					return errors.New("png_export_rom")
				}
			}
			answer, dialogErr := mygo.Dialog.Message(mygo.MessageOptions{Parent: win, Type: mygo.MessageQuestion, Message: question, Detail: path, Buttons: []string{replace, cancel}, CancelButton: 1})
			if dialogErr != nil || answer.Button != 0 {
				return dialogErr
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return os.WriteFile(path, content, 0o600)
	})
}
