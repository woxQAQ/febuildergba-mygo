package chapters

import "github.com/egoist/mygo/ui"

// mapWorkspace keeps the map and its controls alongside each other. Small
// windows scroll the workspace after both columns reach usable widths.
func mapWorkspace(
	c *ui.Context,
	width float32,
	mapView, controls func(*ui.Context),
) {
	mapWidth := max(160, (width-16)*0.45)
	controlsWidth := max(280, width-mapWidth-16)
	ui.ScrollHorizontal(c.Key("map-workspace")).
		Width(width).MaxWidthPercent(100).
		Children(func() {
			ui.Row(c).
				Width(mapWidth + controlsWidth + 16).
				Gap(16).
				AlignItems(ui.Start).
				Children(func() {
					ui.Column(c.Key("map")).
						Width(mapWidth).
						Shrink(0).
						Gap(8).
						Children(func() { mapView(c) })
					ui.Column(c.Key("controls")).
						Width(controlsWidth).
						Shrink(0).
						Gap(10).
						Children(func() { controls(c) })
				})
		})
}
