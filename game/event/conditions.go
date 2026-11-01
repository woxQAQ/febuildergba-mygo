package event

import (
	"bytes"
	"image"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

var ErrEventConditions = fault.Code{
	ID:   "game.invalid_event_conditions",
	Kind: fault.InvalidData,
}

const (
	eventConditionsResource = "event_conditions"
	maxEventConditions      = 256
)

// EventTable selects a native condition list in the chapter event header.
// AlwaysEvents includes area, flag and ASM conditions; unrelated rows are retained.
type Table uint8

const (
	TurnEvents Table = iota
	AlwaysEvents
	TalkEvents
	LocationEvents
)

var eventTables = [...]Table{
	TurnEvents,
	TalkEvents,
	LocationEvents,
	AlwaysEvents,
}

func (table Table) offset() int {
	switch table {
	case AlwaysEvents:
		return 12
	case TalkEvents:
		return 4
	case LocationEvents:
		return 8
	default:
		return 0
	}
}

// conditionLayout describes the fields consumed by the native condition tables.
// Required talk operands are byte offsets, independent of the visible fields.
type conditionLayout struct {
	talkKind, talkStride, turnStride int
	talkModeWidth, triggerWidth      int
	routKind                         byte
	routFlag                         uint32
	reversedArea                     bool
	talkRequired                     []int
}

func (f Format) condition(table Table) int {
	switch table {
	case AlwaysEvents:
		return 0x0B
	case TalkEvents:
		return f.conditions.talkKind
	default:
		return 2
	}
}

func (table Table) source() string {
	switch table {
	case AlwaysEvents:
		return "deploy_area_event"
	case TalkEvents:
		return "deploy_talk_event"
	case LocationEvents:
		return "location_events"
	default:
		return "deploy_turn_event"
	}
}

func (event Condition) source(table Table) string {
	if table == AlwaysEvents && event.Kind != 11 {
		return "deploy_objective_event"
	}
	return table.source()
}

// EventCondition retains its native bytes independently of its script. Typed
// parameters are decoded views, and only confirmed fields are offered for edits.
type Condition struct {
	Address, Kind, Flag, Script           int
	OperandKind                           ConditionTarget
	Operand                               uint32
	Location                              Location
	Start, End, Phase                     int
	Route                                 uint32
	X1, Y1, X2, Y2                        int
	Initiator, Target, TalkMode, TalkFlag int
	TriggerFlag                           uint32
	Rout                                  bool
	Texts                                 []uint16
	Raw                                   []byte
	Groups                                []deployment.Group
	Warnings                              []error
	reversedArea                          bool
}

// Area returns map cell bounds with an exclusive maximum. The native endpoints
// are inclusive. FE8 accepts reversed endpoints; FE6/7 require ordered endpoints.
func (event Condition) Area() image.Rectangle {
	if event.Kind != 0x0B {
		return image.Rectangle{}
	}
	x1, y1, x2, y2 := event.X1, event.Y1, event.X2, event.Y2
	if event.reversedArea {
		x1, x2 = min(x1, x2), max(x1, x2)
		y1, y2 = min(y1, y2), max(y1, y2)
	}
	if x1 > x2 || y1 > y2 {
		return image.Rectangle{}
	}
	return image.Rect(x1, y1, x2+1, y2+1)
}

func (event *Condition) decode(raw []byte, f Format, table Table) {
	event.Raw = bytes.Clone(raw)
	event.Operand = gba.Uint32(raw, 4)
	if table == LocationEvents {
		event.Location, event.OperandKind = f.location(raw)
		if event.OperandKind != ConditionScript {
			event.Script = 0
		}
	}
	event.reversedArea = f.conditions.reversedArea
	event.TriggerFlag = gba.Uint32(raw, 8)
	if f.conditions.triggerWidth == 2 {
		event.TriggerFlag &= 0xFFFF
	}
	event.Rout = f.Rout != 0 && int(gba.Uint32(raw, 8)) == f.Rout &&
		gba.Uint16(raw, 0) == uint16(f.conditions.routKind)
	event.Kind, event.Flag = int(gba.Uint16(raw, 0)), int(gba.Uint16(raw, 2))
	event.Start, event.End, event.Phase = int(
		raw[8],
	), int(
		raw[9],
	), int(
		gba.Uint16(raw, 10),
	)
	event.Initiator, event.Target = int(raw[8]), int(raw[9])
	event.TalkMode, event.TalkFlag = 0, 0
	if len(raw) == 16 {
		event.Route = gba.Uint32(raw, 12)
		event.TalkMode, event.TalkFlag = int(
			gba.Uint16(raw, 12),
		), int(
			gba.Uint16(raw, 14),
		)
		if f.conditions.talkModeWidth == 1 {
			event.TalkMode &= 0xFF
		}
	}
	// Native area coordinates use signed bytes, including off-map bounds.
	event.X1, event.Y1, event.X2, event.Y2 = int(
		int8(raw[8]),
	), int(
		int8(raw[9]),
	), int(
		int8(raw[10]),
	), int(
		int8(raw[11]),
	) // #nosec G115 -- Explicit native signed-byte interpretation.
}

type Conditions struct {
	Pointer, Address int
	Table            Table
	Events           []Condition
	payload          space.Range
	scripts          []space.Claim
	resources        []space.Range
	format           Format
	ending           *Condition
}

// PayloadRegion is the condition list's own extent, without the resources its
// events reference.
func (c Conditions) PayloadRegion() (space.Range, bool) {
	if c.payload == (space.Range{}) {
		return space.Range{}, false
	}
	return c.payload, true
}

func (f Format) eventStride(table Table, kind int) int {
	if table == TalkEvents {
		return f.conditions.talkStride
	}
	if table == TurnEvents && kind != 1 {
		return f.conditions.turnStride
	}
	return 12
}

func (f Format) eventHeader(data []byte, eventID byte) (int, error) {
	list, ok := gba.Pointer(data, f.Root)
	if !ok || list < 0xC0 || list%4 != 0 {
		return 0, deployment.ErrDeployment
	}
	header, ok := gba.Pointer(data, list+int(eventID)*4)
	if !ok || header < 0xC0 || header%4 != 0 {
		return 0, deployment.ErrDeployment
	}
	return header, nil
}

func (f Format) readEvents(
	data []byte,
	site int,
	table Table,
) (result Conditions, err error) {
	result.Pointer, result.Table, result.format = site, table, f
	ctx := fault.Context{
		Resource: eventConditionsResource,
		Range:    &fault.Range{Offset: site, Length: 4, Size: len(data)},
	}
	defer func() { err = fault.Wrap(err, "game.read_event_conditions", ctx) }()
	if table > LocationEvents || site < 0xC0 || site%4 != 0 ||
		!gba.Within(data, site, 4) {
		return result, ErrEventConditions
	}
	if gba.Uint32(data, site) == 0 {
		return result, nil
	}
	at, valid := gba.Pointer(data, site)
	if !valid || at < 0xC0 || at%4 != 0 {
		return result, ErrEventConditions
	}
	result.Address = at
	for i := 0; i <= maxEventConditions; i++ {
		if !gba.Within(data, at, 4) {
			return result, ErrEventConditions
		}
		if gba.Uint32(data, at) == 0 {
			result.payload = space.Range{Start: result.Address, End: at + 4}
			return result, nil
		}
		kind := int(gba.Uint16(data, at))
		length := f.eventStride(table, kind)
		if kind == 0 || i == maxEventConditions ||
			!gba.Within(data, at, length) {
			return result, ErrEventConditions
		}
		event := Condition{Address: at}
		event.decode(data[at:at+length], f, table)
		if event.OperandKind == ConditionScript {
			event.Script, _ = gba.Pointer(data, at+4)
			if event.Operand == 1 {
				event.Script = 1 // Native marker: no user script.
			}
		}
		result.Events = append(result.Events, event)
		at += length
	}
	return result, ErrEventConditions
}

func (s *Editor) Conditions(
	chapter int,
	table Table,
) (result Conditions, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.chapter_events",
			fault.Context{
				Table:    string(records.Chapters),
				RecordID: new(chapter),
			},
		)
	}()
	s.Refresh()
	if s.context.Validate(s.image.Data) != nil {
		return result, edit.ErrVersion
	}
	_, row, field, err := s.records.Resolve(
		records.Chapters,
		chapter,
		HeaderField,
	)
	if err != nil {
		return result, err
	}
	return s.format.ReadConditions(
		s.image.Data,
		chapter,
		s.image.Data[row.Address+field.Offset],
		table,
	)
}

func (f Format) ReadConditions(
	data []byte,
	chapter int,
	eventID byte,
	table Table,
) (Conditions, error) {
	if table > LocationEvents {
		return Conditions{}, ErrEventConditions
	}
	header, err := f.eventHeader(
		data,
		eventID,
	)
	if err != nil {
		return Conditions{}, err
	}
	result, err := f.readEvents(
		data,
		header+table.offset(),
		table,
	)
	if err != nil {
		return result, err
	}
	parsed := deployment.NewReader(data, f.units)
	for i := range result.Events {
		scripts, resources := f.eventResources(
			data,
			chapter,
			table,
			&result.Events[i],
			parsed,
		)
		result.scripts = append(result.scripts, scripts...)
		result.resources = append(result.resources, resources...)
	}
	if table == AlwaysEvents {
		site := header + f.ending
		if gba.Within(data, site, 4) {
			if script, valid := gba.Pointer(
				data,
				site,
			); valid && script >= 0xC0 &&
				script%4 == 0 {
				result.ending = &Condition{
					Address: site,
					Script:  script,
				}
				scripts, _ := f.eventResources(
					data,
					chapter,
					table,
					result.ending,
					parsed,
				)
				result.scripts = append(result.scripts, scripts...)
			}
		}
	}
	return result, nil
}

func Tables() [4]Table { return eventTables }

// ScriptClaims reports code reached from conditions and the ending entry.
func (c Conditions) ScriptClaims() []space.Claim { return slices.Clone(c.scripts) }

// ResourceRegions reports referenced location payloads, excluding the condition
// table, script code and deployment groups (which expose their own regions).
func (c Conditions) ResourceRegions() []space.Range { return slices.Clone(c.resources) }

func (c Conditions) Ending() (Condition, bool) {
	if c.ending == nil {
		return Condition{}, false
	}
	result := *c.ending
	result.Raw = bytes.Clone(result.Raw)
	result.Texts = slices.Clone(result.Texts)
	result.Groups = slices.Clone(result.Groups)
	result.Warnings = slices.Clone(result.Warnings)
	return result, true
}
