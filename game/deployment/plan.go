package deployment

import (
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

type Plan struct {
	commit                          *edit.Commit
	chapter, pointer, before, after int
	plan                            space.WritePlan
	uses                            []Use
}

func (e *Plan) Destination() int { return e.plan.Destination.Start }

func (e *Plan) Length() int { return e.plan.Destination.End - e.plan.Destination.Start }

func (e *Plan) Before() int { return e.before }

func (e *Plan) After() int { return e.after }

func (e *Plan) Uses() []Use { return slices.Clone(e.uses) }

func (s *Editor) Plan(
	draft *Draft,
) (result *Plan, err error) {
	defer func() { err = fault.Wrap(err, "game.plan_deployment", fault.Context{Resource: Resource}) }()
	s.Refresh()
	if draft == nil {
		return nil, rom.ErrStale
	}
	if err = draft.binding.Check(s.context); err != nil {
		return nil, err
	}
	if !draft.Modified() {
		return nil, edit.ErrValue
	}
	if err = draft.Validate(); err != nil {
		return nil, err
	}
	// The selected site is the only existing range changed. The claim check
	// rejects a damaged event graph aliasing any confirmed resource.
	site := draft.group.Pointer
	if s.source.ProtectPointer == nil {
		return nil, edit.ErrLayout
	}
	builder := space.NewBuilder(
		s.image.Data,
		space.Protection{Pointer: s.source.ProtectPointer},
	)
	expected := uint32(0)
	if draft.group.Address != 0 {
		expected = uint32(0x08000000 + draft.group.Address)
	}
	if _, err = builder.ReplaceEncoded(
		func(at int) ([]byte, error) { return draft.Payload(at), nil },
		space.PointerSite{
			Owner:        Owner(site),
			Address:      site,
			ExpectedWord: expected,
		},
	); err != nil {
		return nil, err
	}
	plan := builder.Plan()
	result = &Plan{
		commit:  edit.NewCommit(draft.binding, Resource, plan.Edit),
		chapter: draft.chapter,
		pointer: site,
		before:  len(draft.group.records),
		after:   len(draft.records),
		plan:    plan,
	}
	result.uses = s.source.Uses(s.image.Data, site)
	return result, nil
}

func (s *Editor) Apply(plan *Plan) (err error) {
	ctx := fault.Context{Resource: Resource}
	if plan != nil {
		ctx.Table, ctx.RecordID = string(records.Chapters), new(plan.chapter)
		ctx.Range = &fault.Range{
			Offset: plan.pointer,
			Length: 4,
			Size:   len(s.image.Data),
		}
	}
	defer func() { err = fault.Wrap(err, "game.apply_deployment", ctx) }()
	if plan == nil {
		return rom.ErrStale
	}
	if err := plan.commit.Apply(s.context); err != nil {
		return err
	}
	s.Refresh()
	return nil
}
