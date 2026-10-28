package game

import (
	"maps"

	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

// RecordPortrait separates the stored reference from its resolved class
// fallback and the independent generic miniature resource.
type RecordPortrait struct {
	Ref              AssetRef
	Inherited        bool
	ClassID          int
	GenericMini      int
	Main, Mini       Preview
	MainErr, MiniErr error
	MiniResources    []RecordResource
	document         *rom.Document
	revision         uint64
	layouts          map[Kind]Layout
}

func (p *RecordPortrait) Current(s *Session) bool {
	return p != nil && p.document == s.Document &&
		p.revision == s.Document.State().Revision &&
		maps.Equal(p.layouts, s.layouts)
}

func (s *Session) PortraitMini(id int) (Preview, error) {
	s.Refresh()
	entry, err := s.PortraitEntry(id)
	if err != nil {
		return Preview{}, err
	}
	if entry.Empty {
		return Preview{}, nil
	}
	g, ok := s.Adapter.(gameFormat)
	if !ok || !g.Match(s.image.Data) {
		return Preview{}, ErrVersion
	}
	if entry.ClassCard {
		return Preview{}, nil
	}
	palette, ok := pointer(s.image.Data, entry.Address+8)
	if !ok || palette < 0xC0 || !within(s.image.Data, palette, 32) {
		return Preview{}, ErrImage
	}
	mini, err := portraitPart(
		s.image.Data,
		entry.Address+4,
		32,
		512,
		g.portraits.stride != 16,
		s.image.Data[palette:palette+32],
	)
	return Preview{Image: mini}, err
}

func (s *Session) RecordPortrait(kind Kind, id int) (RecordPortrait, error) {
	s.Refresh()
	result := RecordPortrait{
		document: s.Document,
		revision: s.image.Revision,
		layouts:  maps.Clone(s.layouts),
	}
	if kind != Units && kind != Classes {
		return result, ErrField
	}
	value, err := s.Value(kind, id, "portrait_id")
	if err != nil {
		return result, err
	}
	if value == 0 && kind == Units {
		class, classErr := s.Value(kind, id, "class_id")
		if classErr != nil {
			return result, classErr
		}
		result.ClassID = int(class)
		value, err = s.Value(Classes, int(class), "portrait_id")
		if err != nil {
			result.MainErr = err
			value = 0
		}
		result.Inherited = true
	}
	result.Ref = AssetRef{Kind: PortraitAsset, ID: int(value)}
	if result.MainErr == nil {
		result.Main, result.MainErr = s.Preview(result.Ref)
	}
	if kind == Units {
		mini, miniErr := s.Value(kind, id, "mini_portrait")
		if miniErr != nil {
			result.MiniErr = miniErr
			return result, nil
		}
		result.GenericMini = int(mini)
	}
	if result.GenericMini != 0 {
		result.Mini, result.MiniResources, result.MiniErr = s.genericPortrait(
			result.GenericMini,
		)
	} else if value != 0 {
		result.Mini, result.MiniErr = s.PortraitMini(int(value))
	}
	return result, nil
}

type genericPortraitFormat struct{ pointer, hook, count int }

func (g gameFormat) genericPortraitFormat() genericPortraitFormat {
	switch g.id {
	case fe6ID:
		return genericPortraitFormat{0x8DF0, 0x8DB8, 7}
	case fe7UID:
		return genericPortraitFormat{0x75C0, 0x7598, 6}
	case fe7JID:
		return genericPortraitFormat{0x7450, 0x7428, 6}
	case fe8UID:
		return genericPortraitFormat{0x5F94, 0x5F6C, 8}
	case fe8JID:
		return genericPortraitFormat{0x5E98, 0x5E70, 8}
	}
	return genericPortraitFormat{}
}

func (s *Session) genericPortrait(id int) (Preview, []RecordResource, error) {
	g, ok := s.Adapter.(gameFormat)
	if !ok || !g.Match(s.image.Data) {
		return Preview{}, nil, ErrVersion
	}
	f, data := g.genericPortraitFormat(), s.image.Data
	if id < 0 || id >= f.count {
		return Preview{}, nil, ErrValue
	}
	// The extension replaces the native image/palette lookup with a different
	// table shape. Its resources must not be interpreted as native pointers.
	if u32(data, f.hook) == 0x21FFB500 {
		return Preview{}, nil, ErrAssetFormat
	}
	base, ok := pointer(data, f.pointer)
	if !ok || base < 0xC0 || !within(data, base, 64) {
		return Preview{}, nil, ErrImage
	}
	resources := []RecordResource{
		portraitResource(data, "portrait_mini", base+id*4, 512),
		portraitResource(data, "portrait_palette", base+32+id*4, 32),
	}
	for _, r := range resources {
		if r.Err != nil {
			return Preview{}, resources, r.Err
		}
		if r.Empty {
			return Preview{}, resources, ErrImage
		}
	}
	bitmap, err := gba.Tiles4BPP(
		data[resources[0].Address:resources[0].Address+512],
		data[resources[1].Address:resources[1].Address+32],
		32,
		32,
	)
	return Preview{Image: bitmap}, resources, err
}
