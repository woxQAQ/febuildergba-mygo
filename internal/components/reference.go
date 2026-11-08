package components

import (
	"github.com/egoist/mygo/ui"

	"github.com/woxQAQ/febuildergba-mygo/internal/components/icons"
)

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
	return ActionButton(
		c.Key("open-reference"),
		icons.Lucide.ArrowUpRight(),
		label,
	).
		Width(26).
		Tooltip(label + "\nAlt+Enter").
		Disabled(disabled).
		OnClick(open)
}
