package terrain

import (
	"encoding/binary"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

type Source struct {
	Classes        func([]byte) ([]Class, error)
	ProtectPointer func([]byte, Reference) error
}

type Editor struct {
	context *edit.Context
	format  Format
	source  Source
}

func New(context *edit.Context, format Format, source Source) *Editor {
	return &Editor{context: context, format: format, source: source}
}

func (e *Editor) Format() Format { return e.format }

type Draft struct {
	binding          edit.Stamp
	class            Class
	tables, original []Table
}

func cloneTables(tables []Table) []Table {
	result := slices.Clone(tables)
	for i := range result {
		result[i].Values = slices.Clone(result[i].Values)
	}
	return result
}

func (e *Editor) NewDraft(id int) (result *Draft, err error) {
	defer func() { err = fault.Wrap(err, "terrain.draft", fault.Context{Resource: "terrain", RecordID: &id}) }()
	image := e.context.Document().Snapshot()
	if err = e.context.Validate(image.Data); err != nil {
		return nil, err
	}
	if e.format.count == 0 || e.source.Classes == nil {
		return nil, edit.ErrLayout
	}
	classes, err := e.source.Classes(image.Data)
	if err != nil {
		return nil, err
	}
	for _, class := range classes {
		if class.ID != id {
			continue
		}
		tables := e.format.Read(image.Data, class)
		return &Draft{
			binding:  e.context.Bind(image.Revision),
			class:    class,
			tables:   tables,
			original: cloneTables(tables),
		}, nil
	}
	return nil, edit.ErrValue
}

func (d *Draft) Current(
	e *Editor,
) bool {
	return d != nil && d.binding.Current(e.context)
}
func (d *Draft) Class() Class    { return d.class }
func (d *Draft) Tables() []Table { return cloneTables(d.tables) }
func (d *Draft) Modified() bool {
	for i, table := range d.tables {
		if !slices.Equal(table.Values, d.original[i].Values) {
			return true
		}
	}
	return false
}

func (d *Draft) Set(kind Kind, id, value int) error {
	if value < -128 || value > 127 {
		return edit.ErrValue
	}
	for i := range d.tables {
		table := &d.tables[i]
		if table.Kind != kind {
			continue
		}
		if table.Err != nil {
			return table.Err
		}
		if table.Empty {
			return ErrTable
		}
		if id < 0 || id >= len(table.Values) {
			return edit.ErrValue
		}
		table.Values[id] = value
		return nil
	}
	return edit.ErrField
}

type Change struct{ Terrain, Before, After int }

type Move struct {
	Reference
	Destination int
	Changes     []Change
	// Shared lists known overlapping class tables that retain their old data.
	Shared []Reference
}

type Plan struct {
	plan  *edit.Plan
	moves []Move
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
		result[i].Changes = slices.Clone(result[i].Changes)
		result[i].Shared = slices.Clone(result[i].Shared)
	}
	return result
}

func (e *Editor) Plan(d *Draft) (result *Plan, err error) {
	defer func() { err = fault.Wrap(err, "terrain.plan", fault.Context{Resource: "terrain"}) }()
	if d == nil {
		return nil, rom.ErrStale
	}
	if err = d.binding.Check(e.context); err != nil {
		return nil, err
	}
	image := e.context.Document().Snapshot()
	if err = e.context.Validate(image.Data); err != nil {
		return nil, err
	}
	if !d.Modified() || e.source.ProtectPointer == nil {
		return nil, edit.ErrValue
	}
	classes, err := e.source.Classes(image.Data)
	if err != nil {
		return nil, err
	}
	var tables []Table
	for _, class := range classes {
		tables = append(tables, e.format.Read(image.Data, class)...)
	}
	pending := make(map[int]bool)
	for i, table := range d.tables {
		if !slices.Equal(table.Values, d.original[i].Values) {
			pending[table.Pointer] = true
		}
	}
	result = &Plan{}
	change := rom.Edit{Size: len(image.Data)}
	for i, table := range d.tables {
		if !pending[table.Pointer] {
			continue
		}
		move := Move{Reference: table.Reference}
		for _, other := range tables {
			if other.Pointer == table.Pointer &&
				(other.Class.ID != table.Class.ID || other.Kind != table.Kind) {
				return nil, ErrReferences
			}
			// A pointer write must not alter any table payload, even an invalid
			// table's readable prefix. Payload overlap alone is safe: sources stay.
			if other.Address >= 0xC0 && other.Address < len(image.Data) &&
				table.Pointer < min(
					other.Address+e.format.count,
					len(image.Data),
				) && other.Address < table.Pointer+4 {
				return nil, ErrReferences
			}
			if other.Err == nil && !other.Empty && !pending[other.Pointer] &&
				other.Address < table.Address+e.format.count && table.Address < other.Address+e.format.count {
				move.Shared = append(move.Shared, other.Reference)
			}
		}
		if err = e.source.ProtectPointer(
			image.Data,
			table.Reference,
		); err != nil {
			return nil, err
		}
		payload := make([]byte, len(table.Values))
		for id, value := range table.Values {
			payload[id] = byte(
				value & 0xFF,
			) // #nosec G115 -- signed-byte encoding, range checked by Set.
			if value != d.original[i].Values[id] {
				move.Changes = append(
					move.Changes,
					Change{id, d.original[i].Values[id], value},
				)
			}
		}
		allocation, err := space.PlanAppend(change.Size, payload)
		if err != nil {
			return nil, err
		}
		move.Destination = allocation.Destination.Start
		change.Size = allocation.Edit.Size
		change.Changes = append(change.Changes, allocation.Edit.Changes...)
		pointer := uint32(
			0x08000000 + move.Destination,
		) // #nosec G115 -- PlanAppend bounds the address.
		change.Changes = append(
			change.Changes,
			rom.Change{
				Offset: table.Pointer,
				Data:   binary.LittleEndian.AppendUint32(nil, pointer),
			},
		)
		result.moves = append(result.moves, move)
	}
	result.plan = edit.NewPlan(d.binding, "Edit class terrain tables", change)
	return result, nil
}

func (e *Editor) Apply(plan *Plan) error {
	if plan == nil {
		return rom.ErrStale
	}
	return plan.plan.Apply(e.context)
}
