package app

import (
	"path/filepath"
	"slices"

	"github.com/egoist/mygo"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages"
)

var (
	errTextExportPath = fault.Code{
		ID:   "app.text_export_path",
		Kind: fault.InvalidInput,
	}
	errTextExportRom = fault.Code{
		ID:   "app.text_export_rom",
		Kind: fault.Conflict,
	}
)

func importTextFile(
	win *mygo.Window,
	workspace *pages.Workspace,
	source *text.File,
	done func(*text.Import, string, error),
) {
	if workspace.Editor.Busy || done == nil {
		return
	}
	workspace.Editor.Busy = true
	title := locales.TextFileImport()
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: win,
			Title:  title,
			Filters: []mygo.FileFilter{
				{Name: "TXT", Extensions: []string{"txt"}},
			},
		})
		var draft *text.Import
		var name string
		if err == nil && len(paths) > 0 {
			name = filepath.Base(paths[0])
			var content []byte
			content, err = readResourceFile(
				paths[0],
				text.MaxFileSize,
				text.ErrFileSize,
			)
			if err == nil {
				draft, err = source.Import(content)
			}
		}
		err = fault.Wrap(err, "app.import_text_file", fault.Context{})
		win.Update(func() {
			workspace.Editor.Busy = false
			workspace.Editor.Report("app.import_text_file", err)
			done(draft, name, err)
		})
	}()
}

func (m *application) exportTextFile(
	win *mygo.Window,
	workspace *pages.Workspace,
	source *text.File,
	ids []uint16,
) {
	selected := slices.Clone(ids)
	m.exportResourceContent(win, workspace, resourceExport{
		title: locales.TextFileExport(), suffix: "text", extension: "txt",
		pathError: errTextExportPath, romError: errTextExportRom,
	}, func() ([]byte, error) { return source.Export(selected) })
}
