package app

import (
	"github.com/egoist/mygo"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages"
)

// Importers decode immutable resource snapshots on the worker; the callback
// validates its document and page identity after returning to the UI thread.
func importResourcePNG[T any](
	win *mygo.Window,
	workspace *pages.Workspace,
	title string,
	decode func([]byte) (T, error),
	done func(T, error),
) {
	if workspace.Editor.Busy || done == nil {
		return
	}
	workspace.Editor.Busy = true
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
		var result T
		if err == nil && len(paths) > 0 {
			var content []byte
			content, err = readPNG(paths[0])
			if err == nil {
				result, err = decode(content)
			}
		}
		err = fault.Wrap(err, "app.import_resource_png", fault.Context{})
		win.Update(func() {
			workspace.Editor.Busy = false
			workspace.Editor.Report("app.import_resource_png", err)
			done(result, err)
		})
	}()
}
