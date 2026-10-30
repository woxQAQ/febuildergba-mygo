package game

import (
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/portrait"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/version"
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

func (g format) imageReferences(
	records *records.Editor,
) func([]byte, graphics.Ref) graphics.References {
	return func(data []byte, ref graphics.Ref) graphics.References {
		result := records.ImageReferences(data, ref)
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
}
