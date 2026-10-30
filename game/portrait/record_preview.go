package portrait

import (
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

// RecordPortrait separates the stored reference from its resolved class
// fallback and the independent generic miniature resource.
type RecordPreview struct {
	records.PortraitSelection
	Main, Mini    graphics.Preview
	MiniResources []records.RecordResource
	binding       edit.Stamp
}

func (p *RecordPreview) Current(s *Editor) bool {
	return p != nil && p.binding.Current(s.context) &&
		p.PortraitSelection.Current(s.records)
}

func (s *Editor) Mini(id int) (graphics.Preview, error) {
	s.Refresh()
	entry, err := s.Entry(id)
	if err != nil {
		return graphics.Preview{}, err
	}
	if entry.Empty {
		return graphics.Preview{}, nil
	}
	g := s.format
	if s.context.Validate(s.image.Data) != nil {
		return graphics.Preview{}, edit.ErrVersion
	}
	if entry.ClassCard {
		return graphics.Preview{}, nil
	}
	palette, ok := gba.Pointer(s.image.Data, entry.Address+8)
	if !ok || palette < 0xC0 || !gba.Within(s.image.Data, palette, 32) {
		return graphics.Preview{}, graphics.ErrImage
	}
	mini, err := portraitPart(
		s.image.Data,
		entry.Address+4,
		32,
		512,
		g.Stride != 16,
		s.image.Data[palette:palette+32],
	)
	return graphics.Preview{Image: mini}, err
}

// Preview decodes a record-owned reference selection from the same document
// and interpretation as this portrait editor.
func (s *Editor) Preview(
	selection records.PortraitSelection,
) (RecordPreview, error) {
	s.Refresh()
	result := RecordPreview{
		PortraitSelection: selection,
		binding:           s.context.Bind(s.image.Revision),
	}
	if !selection.Current(s.records) {
		return result, rom.ErrStale
	}
	if result.MainErr == nil {
		result.Main, result.MainErr = s.format.Preview(
			s.image.Data,
			result.Ref.ID,
		)
	}
	if result.MiniErr != nil {
		return result, nil
	}
	if result.GenericMini != 0 {
		result.Mini, result.MiniResources, result.MiniErr = s.genericPortrait(
			result.GenericMini,
		)
	} else if result.Ref.ID != 0 {
		result.Mini, result.MiniErr = s.Mini(result.Ref.ID)
	}
	return result, nil
}

type GenericFormat struct{ Pointer, Hook, Count int }

func (s *Editor) genericPortrait(
	id int,
) (graphics.Preview, []records.RecordResource, error) {
	if s.context.Validate(s.image.Data) != nil {
		return graphics.Preview{}, nil, edit.ErrVersion
	}
	f, data := s.generic, s.image.Data
	if id < 0 || id >= f.Count {
		return graphics.Preview{}, nil, edit.ErrValue
	}
	// The extension replaces the native image/palette lookup with a different
	// table shape. Its resources must not be interpreted as native pointers.
	if gba.Uint32(data, f.Hook) == 0x21FFB500 {
		return graphics.Preview{}, nil, graphics.ErrFormat
	}
	base, ok := gba.Pointer(data, f.Pointer)
	if !ok || base < 0xC0 || !gba.Within(data, base, 64) {
		return graphics.Preview{}, nil, graphics.ErrImage
	}
	resources := []records.RecordResource{
		portraitResource(data, "portrait_mini", base+id*4, 512),
		portraitResource(data, "portrait_palette", base+32+id*4, 32),
	}
	for _, r := range resources {
		if r.Err != nil {
			return graphics.Preview{}, resources, r.Err
		}
		if r.Empty {
			return graphics.Preview{}, resources, graphics.ErrImage
		}
	}
	bitmap, err := gba.Tiles4BPP(
		data[resources[0].Address:resources[0].Address+512],
		data[resources[1].Address:resources[1].Address+32],
		32,
		32,
	)
	return graphics.Preview{Image: bitmap}, resources, err
}
