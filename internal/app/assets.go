package app

import (
	"fmt"

	"github.com/egoist/mygo"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages"
)

var (
	errPngExportPath = fault.Code{
		ID:   "app.png_export_path",
		Kind: fault.InvalidInput,
	}
	errPngExportRom = fault.Code{
		ID:   "app.png_export_rom",
		Kind: fault.Conflict,
	}
)

func importPNG(
	win *mygo.Window,
	workspace *pages.Workspace,
	done func([]byte, error),
) {
	if workspace.Editor.Busy || done == nil {
		return
	}
	workspace.Editor.Busy = true
	title := locales.ReplacePng()
	go func() {
		paths, err := mygo.Dialog.Open(
			mygo.OpenDialogOptions{
				Parent: win,
				Title:  title,
				Filters: []mygo.FileFilter{
					{Name: "PNG", Extensions: []string{"png"}},
				},
			},
		)
		var content []byte
		if err == nil && len(paths) > 0 {
			content, err = readPNG(paths[0])
		}
		err = fault.Wrap(err, "app.import_png", fault.Context{})
		win.Update(func() {
			workspace.Editor.Busy = false
			workspace.Editor.Report("app.import_png", err)
			done(content, err)
		})
	}()
}

func readPNG(path string) (result []byte, err error) {
	defer func() { err = fault.Wrap(err, "app.read_png", fault.Context{Path: path}) }()
	return readResourceFile(path, graphics.MaxPNG, graphics.ErrSize)
}

func (m *application) exportPNG(
	win *mygo.Window,
	workspace *pages.Workspace,
	ref graphics.Ref,
) {
	if workspace.Editor.Session == nil || workspace.Editor.Busy {
		return
	}
	content, err := workspace.Editor.Session.Images.PNG(ref)
	if err != nil {
		workspace.Editor.SetErr(err)
		return
	}
	m.exportPNGContent(
		win,
		workspace,
		fmt.Sprintf("%s-%04X", ref.Kind, ref.ID),
		func() ([]byte, error) { return content, nil },
	)
}

func (m *application) exportPNGContent(
	win *mygo.Window,
	workspace *pages.Workspace,
	suffix string,
	encode func() ([]byte, error),
) {
	m.exportResourceContent(win, workspace, resourceExport{
		title: locales.ExportPng(), suffix: suffix, extension: "png",
		pathError: errPngExportPath, romError: errPngExportRom,
	}, encode)
}
