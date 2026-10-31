package support

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
	Units func([]byte) ([]Unit, error)
	// owner is the character whose pointer may change, or zero for dialog.
	ProtectPointer func(data []byte, site, owner int) error
	ValidateText   func([]byte, uint16) error
}

type Editor struct {
	context   *edit.Context
	format    Format
	source    Source
	binding   edit.Stamp
	records   []Record
	recordErr error
}

func New(context *edit.Context, format Format, source Source) *Editor {
	return &Editor{context: context, format: format, source: source}
}

func (e *Editor) Records() ([]Record, error) {
	if !e.binding.Current(e.context) {
		image := e.context.Document().Snapshot()
		units, err := e.source.Units(image.Data)
		e.binding = e.context.Bind(image.Revision)
		e.recordErr = err
		e.records = make([]Record, len(units))
		for i, unit := range units {
			e.records[i] = e.format.Read(image.Data, unit)
		}
	}
	result := slices.Clone(e.records)
	for i := range result {
		result[i].Raw = bytes.Clone(result[i].Raw)
	}
	return result, e.recordErr
}

func (e *Editor) Talks() ([]Talk, error) {
	table := e.format.talks(e.context.Document().Snapshot().Data)
	return table.rows, table.err
}

// TalkDraft edits the native dialog table independently of character
// relationship records. Pair identities and table order remain unchanged.
type TalkDraft struct {
	binding edit.Stamp
	format  Format
	table   talkTable
	rows    []Talk
}

func (e *Editor) talkDraft(image rom.Snapshot) *TalkDraft {
	table := e.format.talks(image.Data)
	return &TalkDraft{
		binding: e.context.Bind(image.Revision),
		format:  e.format,
		table:   table,
		rows:    cloneTalks(table.rows),
	}
}

func (e *Editor) NewTalkDraft() (*TalkDraft, error) {
	image := e.context.Document().Snapshot()
	if err := e.context.Validate(image.Data); err != nil {
		return nil, err
	}
	draft := e.talkDraft(image)
	if draft.table.err != nil {
		return nil, draft.table.err
	}
	return draft, nil
}

func (d *TalkDraft) Current(
	e *Editor,
) bool {
	return d != nil && d.binding.Current(e.context)
}
func (d *TalkDraft) Talks() []Talk { return cloneTalks(d.rows) }
func (d *TalkDraft) Modified() bool {
	for i, row := range d.rows {
		if !bytes.Equal(row.Raw, d.table.rows[i].Raw) {
			return true
		}
	}
	return false
}

func (d *TalkDraft) SetText(index, rank int, id uint16) error {
	if d.table.err != nil {
		return d.table.err
	}
	if index < 0 || index >= len(d.rows) || rank < 0 || rank >= 3 {
		return edit.ErrValue
	}
	row := &d.rows[index]
	row.Texts[rank] = uint32(id)
	at := 4 + rank*d.format.textWidth
	if d.format.textWidth == 2 {
		binary.LittleEndian.PutUint16(row.Raw[at:], id)
	} else {
		binary.LittleEndian.PutUint32(row.Raw[at:], uint32(id))
	}
	return nil
}

type Draft struct {
	binding       edit.Stamp
	format        Format
	record        Record
	original, raw []byte
	talks         *TalkDraft
}

func (e *Editor) NewDraft(id int) (*Draft, error) {
	image := e.context.Document().Snapshot()
	if err := e.context.Validate(image.Data); err != nil {
		return nil, err
	}
	units, err := e.source.Units(image.Data)
	if err != nil {
		return nil, err
	}
	for _, unit := range units {
		if unit.ID != id {
			continue
		}
		r := e.format.Read(image.Data, unit)
		if r.Err != nil {
			return nil, r.Err
		}
		raw := r.Raw
		if r.Empty {
			raw = make([]byte, e.format.size)
		}
		return &Draft{
			binding:  e.context.Bind(image.Revision),
			format:   e.format,
			record:   r,
			original: raw,
			raw:      bytes.Clone(raw),
			talks:    e.talkDraft(image),
		}, nil
	}
	return nil, edit.ErrValue
}

func (d *Draft) Current(
	e *Editor,
) bool {
	return d != nil && d.binding.Current(e.context)
}
func (d *Draft) Unit() Unit                    { return d.record.Unit }
func (d *Draft) Address() int                  { return d.record.Address }
func (d *Draft) Capacity() int                 { return d.format.capacity }
func (d *Draft) TalkError() error              { return d.talks.table.err }
func (d *Draft) Raw() []byte                   { return bytes.Clone(d.raw) }
func (d *Draft) Relationships() []Relationship { return d.relationships(d.raw) }
func (d *Draft) relationships(raw []byte) []Relationship {
	n := d.format.capacity
	result := make([]Relationship, int(raw[3*n]))
	for i := range result {
		result[i] = Relationship{raw[i], raw[n+i], raw[2*n+i]}
	}
	return result
}

func (d *Draft) Set(index int, value Relationship) error {
	if index < 0 || index >= int(d.raw[3*d.Capacity()]) || value.Unit == 0 ||
		int(value.Unit) == d.record.ID {
		return edit.ErrValue
	}
	n := d.Capacity()
	d.raw[index], d.raw[n+index], d.raw[2*n+index] = value.Unit, value.Initial, value.Growth
	return nil
}

func (d *Draft) Add(value Relationship) error {
	n := d.Capacity()
	count := int(d.raw[3*n])
	if count >= n {
		return space.ErrCapacity
	}
	if value.Unit == 0 || int(value.Unit) == d.record.ID {
		return edit.ErrValue
	}
	d.raw[count], d.raw[n+count], d.raw[2*n+count], d.raw[3*n] = value.Unit, value.Initial, value.Growth, byte(
		count+1,
	)
	return nil
}

func (d *Draft) Remove(index int) error {
	n := d.Capacity()
	count := int(d.raw[3*n])
	if index < 0 || index >= count {
		return edit.ErrValue
	}
	for _, base := range []int{0, n, 2 * n} {
		copy(d.raw[base+index:base+count-1], d.raw[base+index+1:base+count])
		d.raw[base+count-1] = 0
	}
	d.raw[3*n]--
	return nil
}

// Talks preserves global order and save-index identity, including duplicate
// pairs. Text replacement does not add, remove or reorder native entries.
func (d *Draft) Talks() []Talk {
	var result []Talk
	for _, row := range d.talks.rows {
		if int(row.Unit) == d.record.ID || int(row.Partner) == d.record.ID {
			result = append(result, row)
		}
	}
	return cloneTalks(result)
}

func (d *Draft) SetText(index, rank int, id uint16) error {
	if d.talks.table.err != nil {
		return d.talks.table.err
	}
	if index < 0 || index >= len(d.talks.rows) {
		return edit.ErrValue
	}
	row := d.talks.rows[index]
	if int(row.Unit) != d.record.ID && int(row.Partner) != d.record.ID {
		return edit.ErrValue
	}
	return d.talks.SetText(index, rank, id)
}

func (d *Draft) Modified() bool { return !bytes.Equal(d.raw, d.original) || d.talks.Modified() }

type Move struct {
	Unit          int // Zero denotes the global dialog table.
	Before, After int
	Pointers      []int
	Shared        []Unit
}

type Plan struct {
	plan    *edit.Plan
	moves   []Move
	missing []Unit
}

func (p *Plan) Current(
	e *Editor,
) bool {
	return p != nil && p.plan.Current(e.context)
}
func (p *Plan) Size() int { return p.plan.Size() }
func (p *Plan) Moves() []Move {
	result := slices.Clone(p.moves)
	for i := range result {
		result[i].Pointers = slices.Clone(result[i].Pointers)
		result[i].Shared = slices.Clone(result[i].Shared)
	}
	return result
}
func (p *Plan) Missing() []Unit { return slices.Clone(p.missing) }

func (e *Editor) Plan(d *Draft, synchronize bool) (*Plan, error) {
	if d == nil {
		return nil, rom.ErrStale
	}
	return e.plan(d.binding, d, d.talks, synchronize)
}

func (e *Editor) PlanTalks(d *TalkDraft) (*Plan, error) {
	if d == nil {
		return nil, rom.ErrStale
	}
	return e.plan(d.binding, nil, d, false)
}

func (e *Editor) plan(
	binding edit.Stamp,
	d *Draft,
	talks *TalkDraft,
	synchronize bool,
) (result *Plan, err error) {
	defer func() { err = fault.Wrap(err, "support.plan", fault.Context{Resource: "support"}) }()
	if err = binding.Check(e.context); err != nil {
		return nil, err
	}
	image := e.context.Document().Snapshot()
	if err = e.context.Validate(image.Data); err != nil {
		return nil, err
	}
	if (d == nil || !d.Modified()) && !talks.Modified() ||
		e.source.ProtectPointer == nil {
		return nil, edit.ErrValue
	}
	units, err := e.source.Units(image.Data)
	if err != nil {
		return nil, err
	}
	byID := make(map[int]Unit, len(units))
	for _, unit := range units {
		byID[unit.ID] = unit
	}
	pending := map[int][]byte{}
	result = &Plan{}
	if d != nil && !bytes.Equal(d.raw, d.original) {
		pending[d.record.ID] = d.raw
		seen := map[uint8]bool{}
		original := d.relationships(d.original)
		for _, relation := range d.Relationships() {
			unit, valid := byID[int(relation.Unit)]
			if !valid || seen[relation.Unit] || relation.Unit == 0 ||
				int(relation.Unit) == d.record.ID {
				return nil, edit.ErrValue
			}
			seen[relation.Unit] = true
			if !synchronize || slices.Contains(original, relation) {
				continue
			}
			r := e.format.Read(image.Data, unit)
			if r.Err != nil {
				return nil, r.Err
			}
			found := false
			if !r.Empty {
				for i, reverse := range d.relationships(r.Raw) {
					if int(reverse.Unit) != d.record.ID {
						continue
					}
					if found {
						return nil, ErrReferences
					}
					found = true
					n := d.Capacity()
					if reverse.Initial != relation.Initial ||
						reverse.Growth != relation.Growth {
						r.Raw[n+i], r.Raw[2*n+i] = relation.Initial, relation.Growth
						pending[unit.ID] = r.Raw
					}
				}
			}
			if !found {
				result.missing = append(result.missing, unit)
			}
		}
	}
	change := rom.Edit{Size: len(image.Data)}
	resources := e.format.Ranges(image.Data, units)
	appendResource := func(payload []byte, move Move) error {
		for _, site := range move.Pointers {
			if move.Unit != 0 && slices.Contains(e.format.pointers, site) {
				return edit.ErrLayout
			}
			if move.Unit == 0 {
				for _, unit := range units {
					if unit.Pointer == site {
						return edit.ErrLayout
					}
				}
			}
			for _, region := range resources {
				// Only the selected character pointer or native consumer owns
				// this four-byte interval. Payload intersections still fail.
				if region.Start == site && region.End == site+4 {
					continue
				}
				if site < region.End && region.Start < site+4 {
					return edit.ErrLayout
				}
			}
			if err := e.source.ProtectPointer(
				image.Data,
				site,
				move.Unit,
			); err != nil {
				return err
			}
		}
		allocation, err := space.PlanAppend(change.Size, payload)
		if err != nil {
			return err
		}
		change.Size = allocation.Edit.Size
		change.Changes = append(change.Changes, allocation.Edit.Changes...)
		move.After = allocation.Destination.Start
		pointer := uint32(
			0x08000000 + move.After,
		) // #nosec G115 -- PlanAppend bounds the destination.
		for _, site := range move.Pointers {
			change.Changes = append(
				change.Changes,
				rom.Change{
					Offset: site,
					Data:   binary.LittleEndian.AppendUint32(nil, pointer),
				},
			)
		}
		result.moves = append(result.moves, move)
		return nil
	}
	for _, unit := range units {
		raw, changed := pending[unit.ID]
		if !changed {
			continue
		}
		r := e.format.Read(image.Data, unit)
		move := Move{
			Unit:     unit.ID,
			Before:   r.Address,
			Pointers: []int{unit.Pointer},
		}
		for _, other := range units {
			if _, changed := pending[other.ID]; changed {
				continue
			}
			at, valid := gba.Pointer(image.Data, other.Pointer)
			if valid && at == r.Address && other.ID != unit.ID {
				move.Shared = append(move.Shared, other)
			}
		}
		if err = appendResource(raw, move); err != nil {
			return nil, err
		}
	}
	if talks.Modified() {
		if talks.table.err != nil {
			return nil, talks.table.err
		}
		var payload []byte
		for i, row := range talks.rows {
			for rank, id := range row.Texts {
				if id == talks.table.rows[i].Texts[rank] || id == 0 {
					continue
				}
				if id > 65535 || e.source.ValidateText == nil {
					return nil, edit.ErrValue
				}
				if err = e.source.ValidateText(
					image.Data,
					uint16(id),
				); err != nil {
					return nil, err
				}
			}
			payload = append(payload, row.Raw...)
		}
		payload = append(payload, talks.table.terminator...)
		if err = appendResource(
			payload,
			Move{
				Before:   talks.table.address,
				Pointers: slices.Clone(e.format.pointers),
			},
		); err != nil {
			return nil, err
		}
	}
	result.plan = edit.NewPlan(binding, "Edit supports", change)
	return result, nil
}

func (e *Editor) Apply(plan *Plan) error {
	if plan == nil {
		return rom.ErrStale
	}
	return plan.plan.Apply(e.context)
}
