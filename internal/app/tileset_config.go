package app

import (
	"github.com/egoist/mygo"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
	"github.com/woxQAQ/febuildergba-mygo/internal/pages"
)

var (
	errTilesetExportPath = fault.Code{
		ID:   "app.tileset_export_path",
		Kind: fault.InvalidInput,
	}
	errTilesetExportRom = fault.Code{
		ID:   "app.tileset_export_rom",
		Kind: fault.Conflict,
	}
)

func importTilesetConfig(
	win *mygo.Window,
	workspace *pages.Workspace,
	source *maps.TilesetConfig,
	done func(*maps.TilesetDraft, error),
) {
	if workspace.Editor.Busy || done == nil {
		return
	}
	workspace.Editor.SetBusy(true)
	title := locales.MapTilesetImport()
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: win,
			Title:  title,
			Filters: []mygo.FileFilter{
				{
					Name:       "mapchip_config",
					Extensions: []string{"mapchip_config"},
				},
			},
		})
		var draft *maps.TilesetDraft
		if err == nil && len(paths) > 0 {
			var content []byte
			content, err = readResourceFile(
				paths[0],
				source.Size(),
				maps.ErrTilesetConfigSize,
			)
			if err == nil {
				draft, err = source.Import(content)
			}
		}
		err = fault.Wrap(err, "app.import_tileset_config", fault.Context{})
		win.Update(func() {
			workspace.Editor.SetBusy(false)
			workspace.Editor.Report("app.import_tileset_config", err)
			done(draft, err)
		})
	}()
}

func (m *application) exportTilesetConfig(
	win *mygo.Window,
	workspace *pages.Workspace,
	source *maps.TilesetConfig,
) {
	m.exportResourceContent(win, workspace, resourceExport{
		title:     locales.MapTilesetExport(),
		suffix:    "map-tileset",
		extension: "mapchip_config",
		pathError: errTilesetExportPath,
		romError:  errTilesetExportRom,
	}, func() ([]byte, error) { return source.Bytes(), nil })
}
