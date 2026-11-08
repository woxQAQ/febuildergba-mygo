package game

import (
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

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
		if ref.Kind == graphics.ItemIcon && claim.Site &&
			claim.Owner == (romwrite.Owner{Resource: "itemicon"}) &&
			claim.Range == (romwrite.Range{Start: g.icons.Pointer, End: g.icons.Pointer + 4}) {
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
