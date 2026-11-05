package components

import "github.com/egoist/mygo/ui"

// ActionButton names a compact, familiar toolbar action for keyboard users and
// assistive technology. Actions with distinct consequences keep their text.
func ActionButton(c *ui.Context, symbol, label string) ui.Element {
	return ui.Button(c, symbol).
		Size(28, 26).Padding(0).Shrink(0).
		Label(label).Tooltip(label)
}
