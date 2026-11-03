package worldmaps

import (
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/internal/components"
	"github.com/woxQAQ/febuildergba-mygo/internal/locales"
)

// File commands are optional; pending previews and failures stay visible even
// while the commands are closed. The trigger also receives focus after a draft
// is applied or canceled, so keyboard navigation does not follow a removed row.
func (p *Page) imageControls(c *ui.Context) {
	ui.Column(c.Key("worldmap-images")).Gap(6).Children(func() {
		ui.Row(c.Key("image-tools")).
			Wrap().
			Gap(8).
			AlignItems(ui.Center).
			Children(func() {
				ui.Tabs(c.Key("background-tone"), &p.backgroundTone, locales.WorldmapDaytime(), locales.WorldmapDarkened()).
					Label(locales.WorldmapBackgroundTone()).
					Disabled(p.state.Busy)
				ui.Button(c.Key("image-files"), locales.WorldmapImageFiles()).
					Bind(&p.imageFiles).
					Selected(p.imagesOpen).
					Disabled(p.state.Busy).
					OnClick(func() { p.imagesOpen = !p.imagesOpen })
			})
		if p.imagesOpen {
			ui.Column(c.Key("image-file-actions")).Gap(8).Children(func() {
				p.backgroundFiles(c)
				p.roadTilesetFiles(c)
			})
		}
		if p.backgrounds[p.backgroundTone].draft != nil ||
			p.roadTileset.draft != nil {
			ui.Text(c, locales.WorldmapImageDrafts()).
				FontSize(components.FontCaption)
		}
		p.backgroundPreview(c)
		p.roadTilesetPreview(c)
	})
}
