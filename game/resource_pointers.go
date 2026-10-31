package game

import (
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

// protectResourcePointer checks owners shared by independent native resources.
func (s *Session) protectResourcePointer(data []byte, site int) error {
	return s.protectCharacterPointer(data, site, 0)
}

func (s *Session) protectCharacterPointer(data []byte, site, owner int) error {
	kind := records.Kind("")
	if owner > 0 {
		kind = records.Units
	}
	return s.protectRecordPointer(data, site, kind, owner, 44)
}

func (s *Session) protectRecordPointer(
	data []byte,
	site int,
	kind records.Kind,
	owner, offset int,
) error {
	g := s.format
	if site < 0xC0 || site%4 != 0 || !gba.Within(data, site, 4) {
		return edit.ErrLayout
	}
	overlap := func(at, length int) bool { return at >= 0xC0 && length > 0 && site < at+length && at < site+4 }
	for _, rule := range g.tableRules(data) {
		for _, pointer := range rule.Pointers {
			if overlap(pointer, 4) {
				return edit.ErrLayout
			}
		}
	}
	for _, table := range s.Records.Inspect(data).Tables {
		if overlap(table.Pointer, 4) {
			return edit.ErrLayout
		}
		for _, row := range table.Rows {
			if table.Kind == kind && row.ID == owner &&
				site == row.Address+offset && offset >= 0 &&
				table.Stride >= offset+4 {
				continue
			}
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
	for _, region := range s.terrainRanges(data) {
		if kind == records.Classes && region.Start == site &&
			region.End == site+4 {
			continue
		}
		if overlap(region.Start, region.End-region.Start) {
			return edit.ErrLayout
		}
	}
	return nil
}
