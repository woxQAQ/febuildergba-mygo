// Package text reads, encodes and edits game text and its control language.
package text

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

const Resource = "text"

// Use identifies a confirmed text operand. Script uses retain their entry
// pointer, instruction address and source role for precise navigation.
type Use struct {
	Kind             string
	ID               int
	Field            string
	Pointer, Address int
	Source           string
	Record           int // Physical quote-table index for independent script roots.
}

// References contains confirmed protection and usage facts supplied by the
// resource coordinator for the same ROM snapshot as the text operation.
type References struct {
	Uses       []Use
	Incomplete bool
	Ranges     []romwrite.Range
	Sites      []int
	Manual     []romwrite.Range
	Aliases    map[byte]string
}

// Editor owns text browsing, encoding and replacement plans. The reference
// source supplies cross-resource facts without exposing a game session.
type Editor struct {
	context    *edit.Context
	format     Format
	references func([]byte, uint16) References
	image      edit.Input
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
		s.image = s.context.Capture()
		s.stamp = s.image.Stamp()
		s.entries = nil
	}
	return s.context.Validate(s.image.Data)
}

func (s *Editor) Commands() []string { return s.format.Commands() }

// Uses returns confirmed operands and whether every declared script entry was
// understood. A complete search still excludes undeclared custom consumers.
func (s *Editor) Uses(id uint16) ([]Use, bool) {
	if s.refresh() != nil {
		return nil, false
	}
	references := s.references(s.image.Data, id)
	return slices.Clone(references.Uses), !references.Incomplete
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
		textID := uint16(id)
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
