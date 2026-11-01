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
	commit                          *edit.Plan
	chapter, pointer, before, after int
	plan                            space.Plan
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
	if err = s.source.ProtectPointer(
		s.image.Data,
		Owner(site),
		site,
	); err != nil {
		return nil, err
	}
	appender := space.NewAppender(len(s.image.Data))
	content := draft.Payload(appender.Next())
	if _, err = appender.Add(content, site); err != nil {
		return nil, err
	}
	plan := appender.Plan()
	result = &Plan{
		commit:  edit.NewPlan(draft.binding, Resource, plan.Edit),
		chapter: draft.chapter,
		pointer: site,
		before:  len(draft.group.records),
		after:   len(draft.records),
		plan:    plan,
	}
	result.uses = s.source.Uses(s.image.Data, site)
	return result, nil
}

func (s *Editor) Apply(edit *Plan) (err error) {
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
	if err := edit.commit.Apply(s.context); err != nil {
		return err
	}
	s.Refresh()
	return nil
}
