package game

import (
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/portrait"
	"github.com/woxQAQ/febuildergba-mygo/game/version"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func (g format) genericPortraitFormat() portrait.GenericFormat {
	switch g.id {
	case version.FE6:
		return portrait.GenericFormat{Pointer: 0x8DF0, Hook: 0x8DB8, Count: 7}
	case version.FE7U:
		return portrait.GenericFormat{Pointer: 0x75C0, Hook: 0x7598, Count: 6}
	case version.FE7J:
		return portrait.GenericFormat{Pointer: 0x7450, Hook: 0x7428, Count: 6}
	case version.FE8U:
		return portrait.GenericFormat{Pointer: 0x5F94, Hook: 0x5F6C, Count: 8}
	case version.FE8J:
		return portrait.GenericFormat{Pointer: 0x5E98, Hook: 0x5E70, Count: 8}
	}
	return portrait.GenericFormat{}
}

// imageReferences protects confirmed resources against an asset write. The
// claimed ranges exclude the asset's own root pointer: the item-icon pointer
// is the write target itself.
func (s *Session) imageReferences(
	data []byte,
	ref graphics.Ref,
) graphics.References {
	g := s.format
	result := s.Records.ImageReferences(data, ref)
	for _, claim := range s.claims(data) {
		if ref.Kind == graphics.ItemIcon &&
			claim.Range == (space.Range{Start: g.icons.Pointer, End: g.icons.Pointer + 4}) {
			continue
		}
		result.Ranges = append(result.Ranges, claim.Range)
	}
	result.Sites = append(
		result.Sites,
		g.text.Pointer,
		g.text.RootPointer,
		g.text.TreePointer,
		g.portraits.Pointer,
		g.icons.Palette,
	)
	if ref.Kind != graphics.ItemIcon {
		result.Sites = append(result.Sites, g.icons.Pointer)
	}
	return result
}
