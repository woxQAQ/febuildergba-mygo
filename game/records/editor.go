// Package records owns record schemas, table interpretation and field editing.
package records

import (
	"fmt"
	"maps"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

// Editor owns record interpretation, manual table layouts and field edits.
// Its format source resolves version-specific rules for the supplied snapshot.
type Editor struct {
	context    *edit.Context
	source     func([]byte) Format
	definition Format
	texts      *text.Editor
	image      rom.Snapshot
	binding    edit.Stamp
	layouts    map[Kind]Layout
	tables     []Table
	patches    []string
}

func New(
	context *edit.Context,
	source func([]byte) Format,
	texts *text.Editor,
) *Editor {
	s := &Editor{context: context, source: source, texts: texts}
	s.Refresh()
	return s
}

func (s *Editor) Refresh() {
	if !s.binding.Current(s.context) {
		s.refresh(s.context.Document().Snapshot())
	}
}

func (s *Editor) refresh(image rom.Snapshot) {
	s.image = image
	s.syncInterpretation()
	s.binding = s.context.Bind(image.Revision)
	s.definition = s.source(image.Data)
	inspection := s.definition.Inspect(image.Data, s.layouts)
	s.tables, s.patches = inspection.Tables, inspection.Patches
	if err := s.context.Validate(image.Data); err != nil {
		for i := range s.tables {
			s.tables[i].Err = err
		}
	}
}

func (s *Editor) Tables() []Table { s.Refresh(); return slices.Clone(s.tables) }

func (s *Editor) Patches() []string { s.Refresh(); return slices.Clone(s.patches) }

// Inspect interprets a supplied snapshot with this editor's effective layouts.
// Coordinated operations use it to protect manual regions in the same input.
func (s *Editor) Inspect(
	data []byte,
) Inspection {
	return s.source(data).Inspect(data, s.layouts)
}

func (s *Editor) syncInterpretation() {
	var key []byte
	for _, kind := range Kinds {
		if layout, ok := s.layouts[kind]; ok {
			key = fmt.Appendf(
				key,
				"%s:%d:%d:%d;",
				kind,
				layout.Address,
				layout.Stride,
				layout.Count,
			)
		}
	}
	s.context.Interpret(string(key))
}

func (s *Editor) Table(kind Kind) *Table {
	s.Refresh()
	for i := range s.tables {
		if s.tables[i].Kind == kind {
			return &s.tables[i]
		}
	}
	return nil
}

func (s *Editor) Value(kind Kind, id int, fieldID string) (int64, error) {
	table, row, field, err := s.Resolve(kind, id, fieldID)
	if err != nil {
		return 0, err
	}
	if table.Err != nil {
		return 0, table.Err
	}
	return field.Value(s.image.Data, row.Address), nil
}

func (s *Editor) Resolve(
	kind Kind,
	id int,
	fieldID string,
) (table *Table, row Row, field Field, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.locate_field",
			fault.Context{
				Table:    string(kind),
				RecordID: new(id),
				Field:    fieldID,
			},
		)
	}()
	return resolveField(s.Table(kind), id, fieldID)
}

// Set applies one field to one or more records as a single undoable operation.
func (s *Editor) Set(
	kind Kind,
	ids []int,
	fieldID string,
	value int64,
) (err error) {
	ctx := fault.Context{Table: string(kind), Field: fieldID, Value: new(value)}
	defer func() { err = fault.Wrap(err, "game.set", ctx) }()
	if err := s.binding.Check(s.context); err != nil {
		return err
	}
	var changes []rom.Change
	for _, id := range ids {
		ctx.RecordID = new(id)
		_, row, field, err := s.Resolve(kind, id, fieldID)
		if err != nil {
			return err
		}
		low, high := field.Bounds()
		if value < low || value > high {
			ctx.Bounds = &fault.Bounds{Min: low, Max: high}
			return edit.ErrValue
		}
		data := make([]byte, field.Width)
		for i := range data {
			data[i] = byte((value >> (i * 8)) & 0xFF)
		}
		changes = append(
			changes,
			rom.Change{Offset: row.Address + field.Offset, Data: data},
		)
	}
	ctx.RecordID = nil
	if err := s.context.Document().Apply(
		s.image.Revision,
		string(kind)+"."+fieldID,
		changes,
	); err != nil {
		return err
	}
	s.Refresh()
	return nil
}

// UseLayout changes interpretation for this session, without rewriting pointers
// or ROM bytes.
func (s *Editor) UseLayout(kind Kind, layout Layout) (err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.use_layout",
			fault.Context{
				Table: string(kind),
				Range: &fault.Range{
					Offset: layout.Address,
					Length: layout.Stride,
					Size:   len(s.image.Data),
				},
			},
		)
	}()
	s.Refresh()
	if s.context.Validate(s.image.Data) != nil {
		return edit.ErrVersion
	}
	if s.Table(kind) == nil {
		return edit.ErrLayout
	}
	layouts := maps.Clone(s.layouts)
	if layouts == nil {
		layouts = make(map[Kind]Layout)
	}
	layouts[kind] = layout
	inspection := s.source(s.image.Data).Inspect(s.image.Data, layouts)
	for _, table := range inspection.Tables {
		if table.Kind != kind {
			continue
		}
		if table.Err != nil {
			return table.Err
		}
		s.layouts, s.tables, s.patches = layouts, inspection.Tables, inspection.Patches
		s.syncInterpretation()
		s.binding = s.context.Bind(s.image.Revision)
		return nil
	}
	return edit.ErrLayout
}

func (s *Editor) UseAutomaticLayout(kind Kind) {
	delete(s.layouts, kind)
	s.refresh(s.context.Document().Snapshot())
}

func resolveField(
	t *Table,
	id int,
	fieldID string,
) (*Table, Row, Field, error) {
	if t == nil {
		return nil, Row{}, Field{}, edit.ErrLayout
	}
	if t.Err != nil {
		return nil, Row{}, Field{}, t.Err
	}
	for _, row := range t.Rows {
		if row.ID != id {
			continue
		}
		for _, f := range t.Fields {
			if f.ID == fieldID {
				return t, row, f, nil
			}
		}
	}
	return nil, Row{}, Field{}, edit.ErrField
}

// ResolveSnapshot locates a field in a supplied ROM image using this editor's
// effective layouts. Resource coordinators can keep all discovery on one input.
func (s *Editor) ResolveSnapshot(
	data []byte,
	kind Kind,
	id int,
	fieldID string,
) (table *Table, row Row, field Field, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.locate_field",
			fault.Context{
				Table:    string(kind),
				RecordID: new(id),
				Field:    fieldID,
			},
		)
	}()
	if err := s.context.Validate(data); err != nil {
		return nil, Row{}, Field{}, err
	}
	inspection := s.Inspect(data)
	for i := range inspection.Tables {
		if inspection.Tables[i].Kind == kind {
			return resolveField(&inspection.Tables[i], id, fieldID)
		}
	}
	return nil, Row{}, Field{}, edit.ErrLayout
}
