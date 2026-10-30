package records

import (
	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

func (s *Editor) Text(
	kind Kind,
	id int,
	fieldID string,
) (result text.Value, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.read_text_field",
			fault.Context{
				Table:    string(kind),
				RecordID: new(id),
				Field:    fieldID,
			},
		)
	}()
	s.Refresh()
	_, row, field, err := s.Resolve(kind, id, fieldID)
	if err != nil {
		return text.Value{}, err
	}
	if field.Text != text.Description || field.Width != 2 {
		return text.Value{}, edit.ErrField
	}
	return s.texts.Value(
		gba.Uint16(s.image.Data, row.Address+field.Offset),
	)
}

func (s *Editor) PlanTextEdit(
	kind Kind,
	id int,
	fieldID, content string,
) (result *text.Edit, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.plan_text_field",
			fault.Context{
				Table:    string(kind),
				RecordID: new(id),
				Field:    fieldID,
			},
		)
	}()
	s.Refresh()
	_, row, field, err := s.Resolve(kind, id, fieldID)
	if err != nil {
		return nil, err
	}
	if field.Text == text.NoRole || field.Width != 2 {
		return nil, edit.ErrField
	}
	return s.texts.Plan(
		gba.Uint16(s.image.Data, row.Address+field.Offset),
		fieldID,
		content,
		field.Text,
	)
}
