package components

import "github.com/egoist/mygo/ui"

// ActionButton names a compact, familiar toolbar action for keyboard users and
// assistive technology. Actions with distinct consequences keep their text.
func ActionButton(c *ui.Context, icon *ui.SVG, label string) ui.Element {
	return ui.Button(c, "").
		Size(28, 26).Padding(0).Shrink(0).
		Label(label).Tooltip(label).
		Children(func() { ui.Icon(c, icon).Size(16, 16) })
}

// ActionToggle keeps a toolbar preference visible as a pressed icon button.
func ActionToggle(
	c *ui.Context,
	on *bool,
	icon *ui.SVG,
	label string,
) ui.Element {
	return ui.Toggle(c, on, "").
		Size(28, 26).Padding(0).Shrink(0).
		Label(label).Tooltip(label).
		Children(func() { ui.Icon(c, icon).Size(16, 16) })
}

// IconOption names one choice in an icon-only segmented control.
type IconOption struct {
	Icon  *ui.SVG
	Label string
}

// IconSegments preserves a single Tab stop and arrow-key selection for tools.
func IconSegments(
	c *ui.Context,
	selected *int,
	options ...IconOption,
) ui.Element {
	t := c.Theme()
	parts := ui.SegmentedBase(c, selected, len(options))
	return parts.Track.
		AlignItems(ui.Stretch).Padding(2).Gap(2).
		Radius(t.Radius).Background(t.Surface).Border(1, t.Border).
		Children(func() {
			for index, option := range options {
				segment := parts.Segment(index).
					Size(28, 26).Padding(0).Radius(max(t.Radius-2, 0)).
					Label(option.Label).Tooltip(option.Label)
				if index == *selected {
					background := t.Background
					if t.Dark {
						background = t.SurfacePressed
					}
					segment.Background(background).Border(1, t.Border)
				} else if segment.Hovered() {
					segment.Background(t.SurfaceHover)
				}
				segment.Children(
					func() { ui.Icon(c, option.Icon).Size(16, 16) },
				)
			}
		})
}
