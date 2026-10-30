package records

import (
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
)

// PortraitSelection resolves a record's stored portrait, class fallback and
// independent generic miniature. Decoding these resources belongs to portrait.
type PortraitSelection struct {
	Ref                  graphics.Ref
	Inherited            bool
	ClassID, GenericMini int
	MainErr, MiniErr     error
	binding              edit.Stamp
}

func (p PortraitSelection) Current(
	s *Editor,
) bool {
	return p.binding.Current(s.context)
}

func (s *Editor) Portrait(kind Kind, id int) (PortraitSelection, error) {
	s.Refresh()
	result := PortraitSelection{binding: s.context.Bind(s.image.Revision)}
	if kind != Units && kind != Classes {
		return result, edit.ErrField
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
	result.Ref = graphics.Ref{Kind: graphics.Portrait, ID: int(value)}
	if kind == Units {
		mini, miniErr := s.Value(kind, id, "mini_portrait")
		result.GenericMini, result.MiniErr = int(mini), miniErr
	}
	return result, nil
}
