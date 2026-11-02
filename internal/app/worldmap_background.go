package app

import (
	"github.com/egoist/mygo"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages"
)

func importWorldMapBackground(
	win *mygo.Window,
	workspace *pages.Workspace,
	source *worldmap.Background,
	done func(*worldmap.BackgroundDraft, error),
) {
	if workspace.Editor.Busy || done == nil || source == nil {
		return
	}
	workspace.Editor.Busy = true
	title := locales.WorldmapImportBackground()
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: win,
			Title:  title,
			Filters: []mygo.FileFilter{
				{Name: "PNG", Extensions: []string{"png"}},
			},
		})
		var draft *worldmap.BackgroundDraft
		if err == nil && len(paths) > 0 {
			var content []byte
			content, err = readPNG(paths[0])
			if err == nil {
				draft, err = source.ImportPNG(content)
			}
		}
		err = fault.Wrap(err, "app.import_worldmap_background", fault.Context{})
		logError(err)
		win.Update(func() {
			workspace.Editor.Busy = false
			done(draft, err)
		})
	}()
}
