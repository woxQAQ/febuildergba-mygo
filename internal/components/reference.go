package components

import "github.com/egoist/mygo/ui"

// ReferenceLink is the trailing open action of a resource field. Its parent row
// scopes Alt+Enter to that field, including the selector's editing focus.
func ReferenceLink(
	c *ui.Context,
	label string,
	disabled bool,
	open func(),
) ui.Element {
	disabled = disabled || open == nil
	if !disabled {
		c.OnShortcut(ui.Alt, ui.KeyEnter, open)
	}
	return ui.Button(c.Key("open-reference"), "↗").
		Size(26, 26).Padding(0).Shrink(0).
		Label(label).Tooltip(label + "\nAlt+Enter").
		Disabled(disabled).OnClick(open)
}
