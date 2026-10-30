package components

import "github.com/egoist/mygo/ui"

// Sidebar retains the preferred navigation width while making room for the
// editor when the window narrows or the interface zoom increases.
type Sidebar struct {
	preferred, shown, available float32
}

func (s *Sidebar) Width() float32 { return s.shown }

func (s *Sidebar) Render(
	c *ui.Context,
	initial float32,
	navigation, editor func(),
) ui.Element {
	if s.preferred == 0 {
		s.preferred = initial
	}
	width, _ := c.Size()
	if width != s.available {
		s.available = width
		s.shown = min(s.preferred, max(120, width-420))
	}
	return ui.Split(c, &s.shown, navigation, editor).
		Grow(1).
		OnChange(func() { s.preferred = s.shown })
}
