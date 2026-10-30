package deployment

import (
	"encoding/binary"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

type Use struct {
	Chapter int
	Label   string
}

type Edit struct {
	binding                         edit.Stamp
	chapter, pointer, before, after int
	plan                            space.Plan
	uses                            []Use
}

func (e *Edit) Destination() int { return e.plan.Destination.Start }

func (e *Edit) Length() int { return e.plan.Destination.End - e.plan.Destination.Start }

func (e *Edit) Before() int { return e.before }

func (e *Edit) After() int { return e.after }

func (e *Edit) Uses() []Use { return slices.Clone(e.uses) }

func (s *Editor) Plan(
	draft *Draft,
) (result *Edit, err error) {
	defer func() { err = fault.Wrap(err, "game.plan_deployment", fault.Context{Resource: Resource}) }()
	s.Refresh()
	if draft == nil || !draft.Current(s) {
		return nil, rom.ErrStale
	}
	if !draft.Modified() {
		return nil, edit.ErrValue
	}
	if err = draft.Validate(); err != nil {
		return nil, err
	}
	// The selected site is the only existing range changed. Reject a damaged
	// event graph that aliases the header, known record tables, or their roots.
	site := draft.group.Pointer
	for _, table := range s.records.Tables() {
		if table.Pointer >= 0 && site < table.Pointer+4 &&
			site+4 > table.Pointer {
			return nil, ErrDeployment
		}
		if table.Err == nil {
			for _, row := range table.Rows {
				if site < row.Address+table.Stride && site+4 > row.Address {
					return nil, ErrDeployment
				}
			}
		}
	}
	content := draft.Payload((len(s.image.Data) + 3) &^ 3)
	plan, err := space.PlanAppend(len(s.image.Data), content)
	if err != nil {
		return nil, err
	}
	plan.Edit.Changes = append(
		plan.Edit.Changes,
		rom.Change{
			Offset: site,
			Data: binary.LittleEndian.AppendUint32(
				nil,
				uint32(0x08000000+plan.Destination.Start),
			),
		},
	) // #nosec G115 -- PlanAppend bounds addresses by MaxSize.
	result = &Edit{
		binding: s.context.Bind(s.image.Revision),
		chapter: draft.chapter,
		pointer: site,
		before:  len(draft.group.records),
		after:   len(draft.records),
		plan:    plan,
	}
	result.uses = s.source.Uses(s.image.Data, site)
	return result, nil
}

func (s *Editor) Apply(edit *Edit) (err error) {
	ctx := fault.Context{Resource: Resource}
	if edit != nil {
		ctx.Table, ctx.RecordID = string(records.Chapters), new(edit.chapter)
		ctx.Range = &fault.Range{
			Offset: edit.pointer,
			Length: 4,
			Size:   len(s.image.Data),
		}
	}
	defer func() { err = fault.Wrap(err, "game.apply_deployment", ctx) }()
	if edit == nil {
		return rom.ErrStale
	}
	if err := edit.binding.Check(s.context); err != nil {
		return err
	}
	if err := s.context.Document().ApplyEdit(
		edit.binding.Revision(),
		Resource,
		edit.plan.Edit,
	); err != nil {
		return err
	}
	s.Refresh()
	return nil
}
