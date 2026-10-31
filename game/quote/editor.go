package quote

import (
	"bytes"
	"encoding/binary"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

type Source struct {
	ProtectPointer func([]byte, int) error
	ValidateText   func([]byte, uint16) error
}
type Editor struct {
	context *edit.Context
	format  Format
	source  Source
	image   rom.Snapshot
	binding edit.Stamp
	tables  []Table
}

func New(context *edit.Context, format Format, source Source) *Editor {
	return &Editor{context: context, format: format, source: source}
}

func (s *Editor) refresh() {
	if s.binding.Current(s.context) {
		return
	}
	s.image = s.context.Document().Snapshot()
	s.binding = s.context.Bind(s.image.Revision)
	s.tables = s.format.Tables(s.image.Data)
}

func (s *Editor) Tables() []Table {
	s.refresh()
	result := slices.Clone(s.tables)
	for i := range result {
		result[i].Pointers = slices.Clone(result[i].Pointers)
		result[i].Entries = slices.Clone(result[i].Entries)
		for j := range result[i].Entries {
			result[i].Entries[j].Raw = bytes.Clone(result[i].Entries[j].Raw)
		}
	}
	return result
}
func (s *Editor) Version() int       { return s.format.version }
func (s *Editor) AnyChapter() uint32 { return s.format.anyChapter }

func (s *Editor) TriangleChapter(kind Kind) (uint32, bool) {
	return 0xFE, s.format.version == 8 && kind == Battle
}

func (s *Editor) SpecialChapter(kind Kind, value uint32) bool {
	triangle, available := s.TriangleChapter(kind)
	return value == s.AnyChapter() || available && value == triangle
}

type (
	row struct {
		Entry
		original []byte
	}
	Draft struct {
		binding    edit.Stamp
		table      Table
		rows       []row
		nextID     uint64
		anyChapter uint32
	}
)

func (s *Editor) NewDraft(kind Kind) (*Draft, error) {
	image := s.context.Document().Snapshot()
	if err := s.context.Validate(image.Data); err != nil {
		return nil, err
	}
	for _, table := range s.format.Tables(image.Data) {
		if table.Kind != kind {
			continue
		}
		if table.Err != nil {
			return nil, table.Err
		}
		draft := &Draft{
			binding:    s.context.Bind(image.Revision),
			table:      table,
			anyChapter: s.format.anyChapter,
			nextID:     uint64(len(table.Entries) + 1),
		}
		for _, entry := range table.Entries {
			original := entry.Raw
			entry.Raw = bytes.Clone(original)
			draft.rows = append(draft.rows, row{entry, original})
		}
		return draft, nil
	}
	return nil, edit.ErrField
}

func (d *Draft) Current(
	s *Editor,
) bool {
	return d != nil && d.binding.Current(s.context)
}
func (d *Draft) Kind() Kind { return d.table.Kind }

func (d *Draft) Fields() []Field { return slices.Clone(d.table.rule.fields) }
func (d *Draft) Entries() []Entry {
	result := make([]Entry, len(d.rows))
	for i, r := range d.rows {
		result[i] = r.Entry
		result[i].Raw = bytes.Clone(r.Raw)
	}
	return result
}

func (d *Draft) Value(index int, id string) (uint32, error) {
	if index < 0 || index >= len(d.rows) {
		return 0, edit.ErrValue
	}
	for _, field := range d.table.rule.fields {
		if field.ID == id {
			return field.Value(d.rows[index].Raw), nil
		}
	}
	return 0, edit.ErrField
}

func (d *Draft) Set(index int, id string, value uint32) error {
	if index < 0 || index >= len(d.rows) {
		return edit.ErrValue
	}
	for _, field := range d.table.rule.fields {
		if field.ID == id {
			if !field.Editable(d.rows[index].Raw) {
				return edit.ErrLayout
			}
			if value < field.Min || value > field.Max {
				return edit.ErrValue
			}
			field.put(d.rows[index].Raw, value)
			return nil
		}
	}
	return edit.ErrField
}

func (d *Draft) Add() error {
	if len(d.rows) >= MaxEntries {
		return space.ErrCapacity
	}
	raw := make([]byte, d.table.Stride)
	for _, field := range d.table.rule.fields {
		value := field.Min
		switch field.ID {
		case Unit, Opponent:
			value = 1
		case Chapter:
			value = d.anyChapter
		case Route:
			value = 0xFF
		}
		field.put(raw, value)
	}
	d.rows = append(
		d.rows,
		row{Entry: Entry{ID: d.nextID, Address: -1, Raw: raw}},
	)
	d.nextID++
	return nil
}

func (d *Draft) Duplicate(index int) error {
	if index < 0 || index >= len(d.rows) {
		return edit.ErrValue
	}
	if len(d.rows) >= MaxEntries {
		return space.ErrCapacity
	}
	copied := d.rows[index]
	copied.ID = d.nextID
	copied.Address = -1
	copied.Raw = bytes.Clone(copied.Raw)
	d.nextID++
	d.rows = slices.Insert(d.rows, index+1, copied)
	return nil
}

func (d *Draft) Remove(index int) error {
	if index < 0 || index >= len(d.rows) {
		return edit.ErrValue
	}
	d.rows = slices.Delete(d.rows, index, index+1)
	return nil
}

func (d *Draft) Move(from, to int) error {
	if from < 0 || to < 0 || from >= len(d.rows) || to >= len(d.rows) {
		return edit.ErrValue
	}
	moved := d.rows[from]
	d.rows = slices.Delete(d.rows, from, from+1)
	d.rows = slices.Insert(d.rows, to, moved)
	return nil
}

func (d *Draft) Modified() bool {
	if len(d.rows) != len(d.table.Entries) {
		return true
	}
	for i, r := range d.rows {
		if !bytes.Equal(r.Raw, d.table.Entries[i].Raw) {
			return true
		}
	}
	return false
}

type Plan struct {
	plan                 *edit.Plan
	kind                 Kind
	before, after, count int
	pointers             []int
	shared               []Kind
}

func (p *Plan) Kind() Kind      { return p.kind }
func (p *Plan) Before() int     { return p.before }
func (p *Plan) After() int      { return p.after }
func (p *Plan) Count() int      { return p.count }
func (p *Plan) Size() int       { return p.plan.Size() }
func (p *Plan) Pointers() []int { return slices.Clone(p.pointers) }
func (p *Plan) Shared() []Kind  { return slices.Clone(p.shared) }

func (p *Plan) Current(
	s *Editor,
) bool {
	return p != nil && p.plan.Current(s.context)
}

func (s *Editor) Plan(d *Draft) (result *Plan, err error) {
	defer func() { err = fault.Wrap(err, "quote.plan", fault.Context{Resource: "quotes"}) }()
	if d == nil {
		return nil, rom.ErrStale
	}
	if err = d.binding.Check(s.context); err != nil {
		return nil, err
	}
	image := s.context.Document().Snapshot()
	if err = s.context.Validate(image.Data); err != nil {
		return nil, err
	}
	if !d.Modified() {
		return nil, edit.ErrValue
	}
	if s.source.ProtectPointer == nil {
		return nil, edit.ErrLayout
	}
	resources := s.format.resourceRanges(image.Data)
	for _, site := range d.table.Pointers {
		for _, region := range resources {
			if site < region.End && region.Start < site+4 {
				return nil, edit.ErrLayout
			}
		}
		if err = s.source.ProtectPointer(image.Data, site); err != nil {
			return nil, err
		}
	}
	payload := make([]byte, 0, (len(d.rows)+1)*d.table.Stride)
	for _, row := range d.rows {
		if d.table.rule.ended(row.Raw) {
			return nil, edit.ErrValue
		}
		for _, field := range d.table.rule.fields {
			value := field.Value(row.Raw)
			if row.original != nil && value == field.Value(row.original) {
				continue
			}
			if value < field.Min || value > field.Max {
				return nil, edit.ErrValue
			}
			switch field.ID {
			case Text, FinalText:
				if value != 0 {
					if s.source.ValidateText == nil {
						return nil, edit.ErrLayout
					}
					if err = s.source.ValidateText(
						image.Data,
						uint16(value),
					); err != nil {
						return nil, err
					} // #nosec G115 -- Text field bounds are checked above.
				}
			case Script:
				if value != 0 {
					at := int(value) - 0x08000000
					if value < 0x08000000 || at < 0xC0 || at%4 != 0 ||
						!gba.Within(image.Data, at, 4) {
						return nil, edit.ErrValue
					}
				}
			}
		}
		payload = append(payload, row.Raw...)
	}
	payload = append(payload, d.table.terminator...)
	allocation, err := space.PlanAppend(len(image.Data), payload)
	if err != nil {
		return nil, err
	}
	pointer := uint32(
		0x08000000 + allocation.Destination.Start,
	) // #nosec G115 -- PlanAppend bounds the destination by ROM MaxSize.
	for _, site := range d.table.Pointers {
		allocation.Edit.Changes = append(
			allocation.Edit.Changes,
			rom.Change{
				Offset: site,
				Data:   binary.LittleEndian.AppendUint32(nil, pointer),
			},
		)
	}
	result = &Plan{
		plan: edit.NewPlan(
			d.binding,
			"Edit battle and defeat dialog",
			allocation.Edit,
		),
		kind:     d.Kind(),
		before:   d.table.Address,
		after:    allocation.Destination.Start,
		count:    len(d.rows),
		pointers: slices.Clone(d.table.Pointers),
	}
	for _, table := range s.format.Tables(image.Data) {
		if table.Kind != d.Kind() && table.Address == d.table.Address {
			result.shared = append(result.shared, table.Kind)
		}
	}
	return result, nil
}

func (s *Editor) Apply(plan *Plan) error {
	if plan == nil {
		return rom.ErrStale
	}
	return plan.plan.Apply(s.context)
}
