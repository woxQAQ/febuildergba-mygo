// Package text reads, encodes and edits game text and its control language.
package text

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/gba"
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

// Edit is an immutable replacement preview bound to the document and parsing
// settings used during planning.
type Edit struct {
	commit      *edit.Plan
	textID      uint16
	fieldID     string
	content     string
	uses        []Use
	destination space.Range
}

func (e *Edit) TextID() uint16 { return e.textID }

func (e *Edit) Content() string { return e.content }

func (e *Edit) Revision() uint64 { return e.commit.Revision() }

func (e *Edit) Destination() int { return e.destination.Start }

func (e *Edit) Length() int { return e.destination.End - e.destination.Start }

func (e *Edit) Size() int { return e.commit.Size() }

func (e *Edit) Uses() []Use { return slices.Clone(e.uses) }

func (s *Editor) Plan(
	id uint16,
	fieldID, content string,
	role Role,
) (result *Edit, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.plan_text",
			fault.Context{
				Resource:   Resource,
				ResourceID: new(int(id)),
				Field:      fieldID,
			},
		)
	}()
	if err = s.refresh(); err != nil {
		return nil, err
	}
	references := s.references(s.image.Data, id)
	entry, err := s.format.entry(s.image.Data, id, references)
	if err != nil {
		return nil, err
	}
	plan, err := s.format.plan(
		s.image.Data,
		id,
		content,
		role,
		entry,
		references.Aliases,
	)
	if err != nil {
		return nil, err
	}
	for _, region := range references.Manual {
		for _, change := range plan.Edit.Changes {
			if change.Offset < region.End &&
				region.Start < change.Offset+len(change.Data) {
				return nil, edit.ErrLayout
			}
		}
	}
	return &Edit{
		commit: edit.NewPlan(s.stamp, "text."+fieldID, plan.Edit),
		textID: id, fieldID: fieldID, content: content,
		uses: slices.Clone(references.Uses), destination: plan.Destination,
	}, nil
}

func (s *Editor) Apply(plan *Edit) (err error) {
	ctx := fault.Context{Resource: Resource}
	if plan != nil {
		ctx.ResourceID = new(int(plan.textID))
		ctx.Field = plan.fieldID
	}
	defer func() { err = fault.Wrap(err, "game.apply_text", ctx) }()
	if plan == nil {
		return rom.ErrStale
	}
	return plan.commit.Apply(s.context)
}

// Writes require a valid primary table pointer. Recovery is only a read
// fallback and does not prove that the game will consume a replacement.
func (f Format) entry(
	data []byte,
	id uint16,
	references References,
) (int, error) {
	table, ok := gba.Pointer(data, f.Pointer)
	entry := table + int(id)*4
	if id == 0 || id >= 0x7FFF || !ok || table%4 != 0 || table < 0xC0 ||
		!gba.Within(data, entry, 4) {
		return 0, ErrText
	}
	for _, region := range references.Ranges {
		if entry < region.End && region.Start < entry+4 {
			return 0, edit.ErrLayout
		}
	}
	protected := []int{f.Pointer, f.TreePointer, f.RootPointer}
	protected = append(protected, references.Sites...)
	for _, at := range protected {
		if entry < at+4 && at < entry+4 {
			return 0, ErrText
		}
	}
	return entry, nil
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

func (s *Editor) PlanID(
	id uint16,
	content string,
) (result *Edit, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.plan_text_id",
			fault.Context{Resource: Resource, ResourceID: new(int(id))},
		)
	}()
	value, err := s.Read(id)
	if err != nil {
		return nil, err
	}
	if !value.Editable {
		return nil, value.ReadOnlyReason
	}
	return s.Plan(id, "content", content, Description)
}
