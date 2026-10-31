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

type Edit struct {
	commit                          *edit.Plan
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
	appender := space.NewAppender(len(s.image.Data))
	content := draft.Payload(appender.Next())
	if _, err = appender.Add(content, site); err != nil {
		return nil, err
	}
	plan := appender.Plan()
	result = &Edit{
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
	if err := edit.commit.Apply(s.context); err != nil {
		return err
	}
	s.Refresh()
	return nil
}
