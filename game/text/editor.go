// Package text reads, encodes and edits game text and its control language.
package text

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

const Resource = "text"

// Use identifies a known record field sharing a text ID. Table names are
// stable record identifiers; the text codec does not interpret record schemas.
type Use struct {
	Kind  string
	ID    int
	Field string
}

// References contains confirmed protection and usage facts supplied by the
// resource coordinator for the same ROM snapshot as the text operation.
type References struct {
	Uses    []Use
	Ranges  []space.Range
	Sites   []int
	Manual  []space.Range
	Aliases map[byte]string
}

// Editor owns text browsing, encoding and replacement plans. The reference
// source supplies cross-resource facts without exposing a game session.
type Editor struct {
	context    *edit.Context
	format     Format
	references func([]byte, uint16) References
	image      rom.Snapshot
	stamp      edit.Stamp
	entries    []Entry
}

func New(
	context *edit.Context,
	format Format,
	references func([]byte, uint16) References,
) *Editor {
	return &Editor{context: context, format: format, references: references}
}

func (s *Editor) refresh() error {
	if !s.stamp.Current(s.context) {
		s.image = s.context.Document().Snapshot()
		s.stamp = s.context.Bind(s.image.Revision)
		s.entries = nil
	}
	return s.context.Validate(s.image.Data)
}

func (s *Editor) Commands() []string { return s.format.Commands() }

func (s *Editor) Uses(id uint16) []Use {
	if s.refresh() != nil {
		return nil
	}
	return slices.Clone(s.references(s.image.Data, id).Uses)
}

// Value reads a field's text reference, including the native empty ID zero.
func (s *Editor) Value(id uint16) (Value, error) {
	if err := s.refresh(); err != nil {
		return Value{}, err
	}
	return s.format.Read(s.image.Data, id)
}

func (s *Editor) Entries() (entries []Entry, err error) {
	defer func() { err = fault.Wrap(err, "game.list_text", fault.Context{Resource: Resource}) }()
	if err = s.refresh(); err != nil {
		return nil, err
	}
	if s.entries != nil {
		return slices.Clone(s.entries), nil
	}
	table, err := s.format.Table(s.image.Data)
	if err != nil {
		return nil, err
	}
	s.entries = make([]Entry, 0, table.Count-1)
	for id := 1; id < table.Count; id++ {
		textID := uint16(id) // #nosec G115 -- TextTable bounds Count by 0x7FFF.
		value, readErr := s.format.Read(s.image.Data, textID)
		s.entries = append(
			s.entries,
			Entry{
				ID:       textID,
				Content:  value.Content,
				Plain:    value.Plain,
				Editable: value.Editable && !table.Recovered,
				Err:      readErr,
			},
		)
	}
	return slices.Clone(s.entries), nil
}

func (s *Editor) Read(id uint16) (result Value, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.text_by_id",
			fault.Context{Resource: Resource, ResourceID: new(int(id))},
		)
	}()
	if err = s.refresh(); err != nil {
		return Value{}, err
	}
	table, err := s.format.Table(s.image.Data)
	if err != nil {
		return Value{}, err
	}
	if id == 0 || int(id) >= table.Count {
		return Value{}, ErrText
	}
	value, err := s.format.Read(s.image.Data, id)
	if table.Recovered && err == nil {
		value.Editable, value.ReadOnlyReason = false, fault.Wrap(
			ErrText,
			"game.recovered_text_table",
			fault.Context{Resource: Resource, ResourceID: new(int(id))},
		)
	}
	return value, err
}
