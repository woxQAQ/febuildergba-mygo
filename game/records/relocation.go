package records

import (
	"bytes"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// MovePlan is an immutable preview tied to one document revision.
type MovePlan struct {
	commit *edit.Commit
	kind   Kind
	source space.Range
	plan   space.WritePlan
	refs   int
}

func (m *MovePlan) Source() int { return m.source.Start }

func (m *MovePlan) Destination() int { return m.plan.Destination.Start }

func (m *MovePlan) Length() int { return m.source.End - m.source.Start }

func (m *MovePlan) Size() int { return m.plan.Edit.Size }

func (m *MovePlan) ReferenceCount() int { return m.refs }

func (m *MovePlan) Revision() uint64 { return m.commit.Revision() }

// PlanTableMove appends by default. Reusing existing bytes requires declared
// free ranges.
func (s *Editor) PlanTableMove(
	kind Kind,
	available []space.FreeRange,
) (result *MovePlan, err error) {
	defer func() { err = fault.Wrap(err, "game.plan_table_move", fault.Context{Table: string(kind)}) }()
	s.Refresh()
	table := s.Table(kind)
	if table == nil || table.Err != nil || table.Manual {
		return nil, edit.ErrLayout
	}
	input := s.context.CaptureBytes()
	if input.Stamp().Revision() != s.binding.Revision() {
		return nil, rom.ErrStale
	}
	move, err := s.definition.TableMove(input.Data, *table)
	if err != nil {
		return nil, err
	}
	move.Available = available
	for _, other := range s.Tables() {
		if other.Manual && other.Err == nil {
			move.Reserved = append(
				move.Reserved,
				space.Range{
					Start: other.Address,
					End:   other.Address + other.Stride*other.Count,
				},
			)
		}
	}
	builder := space.NewBuilder(input.Data, s.protection)
	if _, err = builder.Relocate(move); err != nil {
		return nil, err
	}
	plan := builder.Plan()
	// Re-run automatic interpretation against the prospective bytes before
	// offering the move.
	data := bytes.Repeat([]byte{0xFF}, plan.Edit.Size)
	copy(data, input.Data)
	for _, change := range plan.Edit.Changes {
		copy(data[change.Offset:], change.Data)
	}
	inspection := s.source(data).Inspect(data, nil)
	valid := false
	for _, candidate := range inspection.Tables {
		if candidate.Kind == kind {
			valid = candidate.Err == nil &&
				candidate.Address == plan.Destination.Start+table.Address-move.Source.Start &&
				candidate.Count == table.Count
		}
	}
	if !valid {
		return nil, edit.ErrLayout
	}
	return &MovePlan{
		commit: edit.NewCommit(
			input.Stamp(),
			"move."+string(kind),
			plan.Edit,
		),
		kind:   kind,
		source: move.Source,
		plan:   plan,
		refs:   len(move.References),
	}, nil
}

func (s *Editor) ApplyTableMove(move *MovePlan) (err error) {
	ctx := fault.Context{}
	if move != nil {
		ctx.Table = string(move.kind)
	}
	defer func() { err = fault.Wrap(err, "game.apply_table_move", ctx) }()
	if move == nil {
		return rom.ErrStale
	}
	// Byte-bound moves keep their own precondition: a table under manual
	// layout cannot be relocated by an automatic plan.
	if _, manual := s.layouts[move.kind]; manual {
		return edit.ErrLayout
	}
	if err := move.commit.Apply(s.context); err != nil {
		return err
	}
	s.Refresh()
	return nil
}

func (g Format) TableMove(data []byte, table Table) (space.Move, error) {
	move := space.Move{Owner: space.Owner{Resource: string(table.Kind), ID: -1}}
	for _, rule := range g.Rules {
		if rule.Kind != table.Kind {
			continue
		}
		source, err := rule.Extent(data, table)
		if err != nil {
			return space.Move{}, err
		}
		move.Source = source
		for _, address := range rule.Pointers {
			if target, ok := gba.Pointer(
				data,
				address,
			); ok &&
				target == source.Start {
				move.References = append(
					move.References,
					space.Reference{Owner: move.Owner, Address: address},
				)
			}
		}
		// These fields are known pointers, rather than arbitrary pointer-shaped
		// bytes.
		for _, row := range table.Rows {
			for _, offset := range rule.PointerFields {
				if target, ok := gba.Pointer(
					data,
					row.Address+offset,
				); ok && target >= source.Start &&
					target < source.End {
					move.References = append(
						move.References,
						space.Reference{
							Owner: space.Owner{
								Resource: string(table.Kind),
								ID:       row.ID,
							},
							Address: row.Address + offset,
							Offset:  target - source.Start,
						},
					)
				}
			}
		}
	}
	if len(move.References) == 0 {
		return space.Move{}, space.ErrReference
	}
	move.Reserved = append(move.Reserved, space.Range{Start: 0, End: 0xC0})
	for _, rule := range g.Rules {
		for _, address := range rule.Pointers {
			move.Reserved = append(
				move.Reserved,
				space.Range{Start: address, End: address + 4},
			)
		}
		other := rule.Read(
			data,
			Layout{},
			false,
			func(uint16) (string, error) { return "", nil },
		)
		if other.Err != nil {
			continue
		}
		region, err := rule.Extent(data, other)
		if err != nil {
			return space.Move{}, err
		}
		move.Reserved = append(move.Reserved, region)
	}
	if g.Protected != nil {
		move.Reserved = append(move.Reserved, g.Protected(data)...)
	}
	return move, nil
}
