package game

import (
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

// protectResourcePointer checks owners shared by independent native resources.
func (s *Session) protectResourcePointer(data []byte, site int) error {
	g := s.format
	if site < 0xC0 || site%4 != 0 || !gba.Within(data, site, 4) {
		return edit.ErrLayout
	}
	overlap := func(at, length int) bool { return at >= 0xC0 && length > 0 && site < at+length && at < site+4 }
	for _, table := range s.Records.Inspect(data).Tables {
		if overlap(table.Pointer, 4) {
			return edit.ErrLayout
		}
		for _, row := range table.Rows {
			if overlap(row.Address, table.Stride) {
				return edit.ErrLayout
			}
		}
	}
	for _, at := range []int{g.text.Pointer, g.text.TreePointer, g.text.RootPointer, g.icons.Pointer, g.icons.Palette, g.portraits.Pointer, g.worldmap.Pointer} {
		if overlap(at, 4) {
			return edit.ErrLayout
		}
	}
	if table, err := g.text.Table(
		data,
	); err == nil &&
		overlap(table.Address, table.Count*4) {
		return edit.ErrLayout
	}
	chapters := g.chapterConfig()
	chapters.External = nil
	for _, region := range chapters.Ranges(data) {
		if overlap(region.Start, region.End-region.Start) {
			return edit.ErrLayout
		}
	}
	return nil
}
