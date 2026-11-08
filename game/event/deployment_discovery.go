package event

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

var ErrDeploymentScript = fault.Code{
	ID:   "game.incomplete_deployment_script",
	Kind: fault.Unsupported,
}

type Deployments struct {
	binding  edit.Stamp
	Groups   []deployment.Group
	Warnings []error
	scripts  []romwrite.Claim
}

func (d Deployments) Current(
	editor *Editor,
) bool {
	return d.binding.Current(editor.context)
}

func (s *Editor) Deployments(
	id int,
) (result Deployments, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.chapter_deployments",
			fault.Context{Table: string(records.Chapters), RecordID: new(id)},
		)
	}()
	s.Refresh()
	defer func() { result.binding = s.image.Stamp() }()
	if s.context.Validate(s.image.Data) != nil {
		return result, edit.ErrVersion
	}
	_, row, field, err := s.records.Resolve(
		records.Chapters,
		id,
		HeaderField,
	)
	if err != nil {
		return result, err
	}
	return s.format.ReadDeployments(
		s.image.Data,
		id,
		s.image.Data[row.Address+field.Offset],
	)
}

func (f Format) ReadDeployments(
	data []byte,
	chapter int,
	eventID byte,
) (result Deployments, err error) {
	ctx := fault.Context{
		Resource:   HeaderField,
		ResourceID: new(int(eventID)),
	}
	defer func() { err = fault.Wrap(err, "game.read_deployment_entries", ctx) }()
	base, err := f.eventHeader(data, eventID)
	if err != nil {
		return result, err
	}
	parsed := deployment.NewReader(data, f.units)
	for _, slot := range f.slots {
		result.Groups = append(
			result.Groups,
			parsed.Group(base+slot.offset, slot.label, slot.faction),
		)
	}
	scanner := deploymentScanner{
		format:    f,
		data:      data,
		result:    &result,
		seen:      make(map[int]bool),
		active:    make(map[int]bool),
		parsed:    parsed,
		remaining: 8192,
		chapter:   chapter,
	}
	scanner.entries(base)
	return result, nil
}

// ScriptClaims reports decoded instructions and their call sites, separately
// from deployment payloads returned by each group's Regions method.
func (d Deployments) ScriptClaims() []romwrite.Claim { return slices.Clone(d.scripts) }

// Discovery follows typed operands with bounded recursion. It does not execute
// branches, ASM, or dynamic calls, and reports incomplete discovery explicitly.
type deploymentScanner struct {
	format             Format
	data               []byte
	result             *Deployments
	seen, active       map[int]bool
	parsed             *deployment.Reader
	remaining, chapter int
	texts              *[]uint16
}

func (s *deploymentScanner) warn(at int) {
	for _, warning := range s.result.Warnings {
		for frame := range fault.Frames(warning) {
			if frame.Context.Range != nil && frame.Context.Range.Offset == at {
				return
			}
		}
	}
	if len(s.result.Warnings) < 8 {
		s.result.Warnings = append(
			s.result.Warnings,
			fault.Wrap(
				ErrDeploymentScript,
				"game.scan_initial_deployments",
				fault.Context{
					Resource: deployment.Resource,
					Range: &fault.Range{
						Offset: at,
						Length: 4,
						Size:   len(s.data),
					},
				},
			),
		)
	}
}

func (s *deploymentScanner) add(site int, label string) {
	if s.seen[site] {
		return
	}
	if len(s.result.Groups) >= 128 {
		s.warn(site)
		return
	}
	s.seen[site] = true
	s.result.Groups = append(
		s.result.Groups,
		s.parsed.Group(site, label, 0),
	)
}

func (s *deploymentScanner) entries(base int) {
	if s.format.opening >= 0 {
		at := base + s.format.opening
		if gba.Within(s.data, at, 4) && gba.Uint32(s.data, at) != 0 {
			if start, valid := gba.Pointer(s.data, at); valid {
				s.walk(start, eventSlotState{}, 0, s.chapter, "deploy_opening")
			} else {
				s.warn(at)
			}
		}
	}
	for _, table := range eventTables {
		s.conditions(base, table)
	}
}

func (s *deploymentScanner) conditions(base int, table Table) {
	site := base + table.offset()
	conditions, err := s.format.readEvents(s.data, site, table)
	if err != nil {
		s.warn(site)
	}
	for _, event := range conditions.Events {
		if !s.format.editableCondition(table, event) {
			continue
		}
		if event.Script == 1 {
			continue
		}
		if event.Script < 0xC0 || event.Script%4 != 0 {
			s.warn(event.Address + 4)
			continue
		}
		source := event.source(table)
		if table == TurnEvents && event.Start <= 1 && event.Phase == 0 {
			source = "deploy_turn_one"
		}
		s.walk(event.Script, eventSlotState{}, 0, s.chapter, source)
	}
}

type deploymentInstruction struct {
	length               int
	end                  bool
	call                 int
	loads                []int
	label, branch, mapID int
	chapterEnd, jump     bool
}

func (s *deploymentScanner) instruction(
	at int,
) (command deploymentInstruction, ok bool) {
	command.call, command.label, command.branch, command.mapID = -1, -1, -1, -1
	if !gba.Within(s.data, at, 4) {
		return command, false
	}
	return s.format.scripts.decode(s, at, command)
}

func (s *deploymentScanner) sizedInstruction(
	at int,
	command deploymentInstruction,
) (deploymentInstruction, bool) {
	header, op := s.data[at], s.data[at+1]
	command.length = int(header>>4) * 2
	if command.length < 4 || command.length%4 != 0 ||
		!gba.Within(s.data, at, command.length) ||
		op > 0xD0 {
		return command, false
	}
	switch {
	case op == 0x01 && command.length == 4:
		command.end = true
	case op == 0x08 && command.length == 4:
		command.label = int(gba.Uint16(s.data, at+2))
	case (op == 0x09 && command.length == 4) || (op == 0x0C && command.length == 8):
		command.branch = int(gba.Uint16(s.data, at+2))
	case op == 0x0A && header == 0x40 && command.length == 8:
		command.call = at + 4
	case op == 0x2C && command.length == 8 && header >= 0x40 && header <= 0x42:
		command.loads = []int{at + 4}
	case op == 0x25 && command.length == 4:
		value := gba.Uint16(s.data, at+2)
		if value == 0xFFFF {
			return command, false
		}
		command.mapID = int(value)
	case op == 0x2A:
		command.chapterEnd = true
	}
	return command, true
}

func (s *deploymentScanner) wordInstruction(
	at int,
	command deploymentInstruction,
) (deploymentInstruction, bool) {
	if s.data[at+1] != 0 {
		return command, false
	}
	op := s.data[at]
	command.length = int(s.format.scripts.words[op]) * 4
	if command.length == 0 || !gba.Within(s.data, at, command.length) {
		return command, false
	}
	rule := s.format.scripts.flow[op]
	command.end, command.chapterEnd, command.jump = rule.end, rule.chapterEnd, rule.jump
	if rule.call != 0 {
		command.call = at + rule.call
	}
	if rule.label != 0 {
		command.label = int(gba.Uint32(s.data, at+rule.label))
	}
	if rule.branch != 0 {
		command.branch = int(gba.Uint32(s.data, at+rule.branch))
	}
	if rule.mapID != 0 {
		command.mapID = int(gba.Uint32(s.data, at+rule.mapID))
	}
	for _, offset := range rule.loads {
		command.loads = append(command.loads, at+offset)
	}
	return command, true
}

func (s *deploymentScanner) walk(
	at int,
	slots eventSlotState,
	depth, mapID int,
	source string,
) int {
	if depth >= 16 || s.active[at] || at < 0xC0 || at%4 != 0 {
		s.warn(at)
		return mapID
	}
	start := at
	s.active[start] = true
	defer delete(s.active, start)
	pending, labels := make(map[int]bool), make(map[int]bool)
	collect := true
	for s.remaining > 0 {
		s.remaining--
		command, valid := s.instruction(at)
		if !valid {
			s.warn(at)
			return mapID
		}
		s.result.scripts = append(s.result.scripts, romwrite.Claim{
			Start: at, End: at + command.length,
			Owner: ScriptCodeOwner(),
		})
		if command.call >= 0 {
			s.result.scripts = append(s.result.scripts, romwrite.Claim{
				Start: command.call, End: command.call + 4,
				Owner: ScriptOwner(command.call),
				Host:  ScriptCodeOwner(),
				Site:  true,
			})
		}
		if command.label >= 0 {
			labels[command.label] = true
			if pending[command.label] {
				collect = true
				delete(pending, command.label)
			}
		}
		if collect && command.branch >= 0 && !labels[command.branch] {
			pending[command.branch] = true
		}
		if command.end || command.chapterEnd {
			if len(pending) == 0 ||
				s.format.labelSearchEnd(s.data[at:at+command.length]) {
				return mapID
			}
			collect = false
		}
		if collect {
			mapID = s.follow(at, command, &slots, depth, mapID, source)
		}
		if command.jump {
			if len(pending) == 0 {
				return mapID
			}
			collect = false
		}
		at += command.length
	}
	s.warn(at)
	return mapID
}

func (s *deploymentScanner) follow(
	at int,
	command deploymentInstruction,
	slots *eventSlotState,
	depth, mapID int,
	source string,
) int {
	s.collectTexts(at, slots)
	if command.mapID >= 0 {
		mapID = command.mapID
	}
	for _, site := range command.loads {
		if mapID != s.chapter {
			continue
		}
		if gba.Uint32(s.data, site)&0x80000000 != 0 &&
			s.format.scripts.trackSlots {
			site = slots.slots[2]
		}
		if site > 0 && gba.Within(s.data, site, 4) &&
			gba.Uint32(s.data, site) != 0 {
			s.add(site, source)
		} else if site == 0 {
			s.warn(at)
		}
	}
	if command.call < 0 {
		if s.format.scripts.trackSlots {
			slots.observe(s.data[at:at+command.length], at+4)
		}
		return mapID
	}
	if target, valid := gba.Pointer(s.data, command.call); valid {
		if !s.loadHelper(target, *slots, mapID, source) {
			mapID = s.walk(target, *slots, depth+1, mapID, source)
		}
	} else {
		s.warn(at)
	}
	*slots = eventSlotState{}
	return mapID
}

func (s *deploymentScanner) loadHelper(
	target int,
	slots eventSlotState,
	mapID int,
	source string,
) bool {
	root := s.format.LoadHelper
	if root == 0 ||
		(target != root && target != root+0x2C && target != root+0x54) {
		return false
	}
	// These native LOAD helpers receive pointers in slots 2 and 3. The value
	// 1 means to reuse an existing group and is not a pointer site to rewrite.
	if mapID == s.chapter {
		for _, slot := range []int{2, 3} {
			if site := slots.slots[slot]; site != 0 {
				if _, valid := gba.Pointer(s.data, site); valid {
					s.add(site, source)
				}
			}
		}
	}
	return true
}
