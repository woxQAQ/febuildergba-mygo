package game

import (
	"encoding/binary"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// ObjectiveTemplate adds a native condition; existing chapter rules remain in order.
type ObjectiveTemplate uint8

const (
	BossVictory ObjectiveTemplate = iota
	RoutVictory
	FlagFailure
)

type EventScriptKind uint8

const (
	ExistingEventScript EventScriptKind = iota
	ChapterEndingScript
	GameOverScript
	FlagOnlyEvent
)

// EventScriptRef names an existing resource or the native flag-only marker.
// Selecting a reference changes the condition operand, never the script bytes.
type EventScriptRef struct {
	Address int
	Kind    EventScriptKind
}

func (f deploymentFormat) eventResources(
	data []byte,
	chapter int,
	table EventTable,
	event *EventCondition,
	parsed map[int]DeploymentGroup,
) []space.Range {
	if event.Script == 1 {
		return nil
	}
	resources := ChapterDeployments{}
	scanner := deploymentScanner{
		format: f, data: data, result: &resources,
		seen: make(map[int]bool), active: make(map[int]bool), parsed: parsed,
		texts: &event.Texts, remaining: 8192, chapter: chapter,
	}
	if event.Script < 0xC0 || event.Script%4 != 0 {
		scanner.warn(event.Address + 4)
	} else {
		scanner.walk(
			event.Script,
			eventSlotState{},
			0,
			chapter,
			event.source(table),
		)
	}
	event.Groups, event.Warnings = resources.Groups, resources.Warnings
	return resources.ranges
}

func (f deploymentFormat) editableCondition(
	table EventTable,
	event EventCondition,
) bool {
	return event.Kind == f.condition(table) ||
		(table == AlwaysEvents && (event.Kind == 1 || event.Rout))
}

func (d *EventDraft) objectiveFields(index int) []Field {
	if d.original.Table != AlwaysEvents {
		return nil
	}
	event := d.records[index].event
	kind := int(u16(d.records[index].raw, 0))
	if kind != 1 && !event.Rout {
		return nil
	}
	fields := []Field{{ID: EventFlag, Offset: 2, Width: 2, Hex: true}}
	if kind == 1 {
		width := 2
		if d.original.format.kind == eventFE8 {
			width = 4
		}
		fields = append(
			fields,
			Field{ID: EventTrigger, Offset: 8, Width: width, Hex: true},
		)
	}
	return fields
}

func (s *Session) objectiveScripts(
	resource ChapterEvents,
	chapter int,
) []EventScriptRef {
	result := []EventScriptRef{{Address: 1, Kind: FlagOnlyEvent}}
	add := func(address int, kind EventScriptKind) {
		if address < 0xC0 || address%4 != 0 ||
			!within(s.image.Data, address, 4) {
			return
		}
		if !slices.ContainsFunc(
			result,
			func(ref EventScriptRef) bool { return ref.Address == address },
		) {
			result = append(
				result,
				EventScriptRef{Address: address, Kind: kind},
			)
		}
	}
	if resource.ending != nil {
		add(resource.ending.Script, ChapterEndingScript)
	}
	// Prefer a relocated/custom game-over event already used by this chapter.
	for _, event := range resource.Events {
		if event.Kind == 1 && event.TriggerFlag == 0x65 {
			add(event.Script, GameOverScript)
		}
	}
	scanner := deploymentScanner{format: resource.format, data: s.image.Data}
	if at := resource.format.defeat; within(s.image.Data, at, 4) {
		if _, valid := scanner.instruction(at); valid {
			add(at, GameOverScript)
		}
	}
	for _, table := range eventTables {
		conditions := resource
		if table != AlwaysEvents {
			var err error
			conditions, err = s.ChapterEvents(chapter, table)
			if err != nil {
				continue
			}
		}
		for _, event := range conditions.Events {
			add(event.Script, ExistingEventScript)
		}
	}
	return result
}

func (d *EventDraft) Scripts() []EventScriptRef { return slices.Clone(d.scripts) }

// SetEventScript only links a confirmed existing chapter reference or the
// flag-only marker. Resources are resolved again so navigation follows the draft.
func (d *EventDraft) SetEventScript(
	s *Session,
	index, address int,
) (err error) {
	defer func() {
		err = fault.Wrap(err, "game.change_condition_script", fault.Context{
			Resource: eventConditionsResource,
			RecordID: new(index),
			Value:    new(int64(address)),
		})
	}()
	if !d.Current(s) {
		return rom.ErrStale
	}
	if index < 0 || index >= len(d.records) ||
		len(d.objectiveFields(index)) == 0 {
		return ErrField
	}
	if !slices.ContainsFunc(
		d.scripts,
		func(ref EventScriptRef) bool { return ref.Address == address },
	) {
		return ErrValue
	}
	word := uint32(1)
	if address != 1 {
		word = uint32(
			0x08000000 + address,
		) // #nosec G115 -- References are bounded to existing ROM resources.
	}
	record := &d.records[index]
	binary.LittleEndian.PutUint32(record.raw[4:], word)
	record.event.Script = address
	record.event.Texts, record.event.Groups, record.event.Warnings = nil, nil, nil
	d.original.format.eventResources(
		s.image.Data,
		d.chapter,
		AlwaysEvents,
		&record.event,
		make(map[int]DeploymentGroup),
	)
	return nil
}

func (d *EventDraft) AddObjective(
	s *Session,
	template ObjectiveTemplate,
) (err error) {
	defer func() { err = fault.Wrap(err, "game.add_objective", fault.Context{Resource: eventConditionsResource}) }()
	if !d.Current(s) {
		return rom.ErrStale
	}
	if d.original.Table != AlwaysEvents || template > FlagFailure {
		return ErrField
	}
	if len(d.records) >= maxEventConditions {
		return ErrValue
	}
	target := ChapterEndingScript
	if template == FlagFailure {
		target = GameOverScript
	}
	address := 0
	for _, ref := range d.scripts {
		if ref.Kind == target {
			address = ref.Address
			break
		}
	}
	if address == 0 {
		return ErrEventConditions
	}
	raw := make([]byte, 12)
	raw[0], raw[2] = 1, 3
	binary.LittleEndian.PutUint32(
		raw[4:],
		uint32(0x08000000+address),
	) // #nosec G115 -- Script references were bounded against the current ROM.
	trigger := uint32(2)
	if template == FlagFailure {
		raw[2], trigger = 0, 0x65
	}
	if template == RoutVictory {
		if d.original.format.kind == eventFE8 {
			trigger = 6
		} else {
			raw[0] = 14
			if d.original.format.kind == eventFE6 {
				raw[0] = 13
			}
			trigger = uint32(
				d.original.format.rout,
			) // #nosec G115 -- Native Thumb callback constants fit in uint32.
		}
	}
	binary.LittleEndian.PutUint32(raw[8:], trigger)
	event := EventCondition{Script: address}
	event.decode(raw, d.original.format)
	d.original.format.eventResources(
		s.image.Data,
		d.chapter,
		AlwaysEvents,
		&event,
		make(map[int]DeploymentGroup),
	)
	d.records = append(d.records, eventRecord{event: event, raw: raw})
	return nil
}
