package game

import (
	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

type deploymentEventKind uint8

const (
	eventFE8 deploymentEventKind = iota
	eventFE7J
	eventFE7U
	eventFE6
)

// Discovery follows typed operands with bounded recursion. It does not execute
// branches, ASM, or dynamic calls, and reports incomplete discovery explicitly.
type deploymentScanner struct {
	format             deploymentFormat
	data               []byte
	result             *ChapterDeployments
	seen, active       map[int]bool
	parsed             map[int]DeploymentGroup
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
					Resource: deploymentResource,
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
		s.format.group(s.data, site, label, 0, s.parsed),
	)
}

func (s *deploymentScanner) entries(base int) {
	if s.format.opening >= 0 {
		at := base + s.format.opening
		if within(s.data, at, 4) && u32(s.data, at) != 0 {
			if start, valid := pointer(s.data, at); valid {
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

func (s *deploymentScanner) conditions(base int, table EventTable) {
	site := base + table.offset()
	conditions, err := s.format.readEvents(s.data, site, table)
	if err != nil {
		s.warn(site)
	}
	s.result.ranges = append(s.result.ranges, conditions.ranges...)
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
	if !within(s.data, at, 4) {
		return command, false
	}
	switch s.format.kind {
	case eventFE8:
		return s.fe8Instruction(at, command)
	default:
		return s.wordInstruction(at, command)
	}
}

func (s *deploymentScanner) fe8Instruction(
	at int,
	command deploymentInstruction,
) (deploymentInstruction, bool) {
	header, op := s.data[at], s.data[at+1]
	command.length = int(header>>4) * 2
	if command.length < 4 || command.length%4 != 0 ||
		!within(s.data, at, command.length) ||
		op > 0xD0 {
		return command, false
	}
	switch {
	case op == 0x01 && command.length == 4:
		command.end = true
	case op == 0x08 && command.length == 4:
		command.label = int(u16(s.data, at+2))
	case (op == 0x09 && command.length == 4) || (op == 0x0C && command.length == 8):
		command.branch = int(u16(s.data, at+2))
	case op == 0x0A && header == 0x40 && command.length == 8:
		command.call = at + 4
	case op == 0x2C && command.length == 8 && header >= 0x40 && header <= 0x42:
		command.loads = []int{at + 4}
	case op == 0x25 && command.length == 4:
		value := u16(s.data, at+2)
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
	command.length = int(deploymentEventWords[s.format.kind][op]) * 4
	if command.length == 0 || !within(s.data, at, command.length) {
		return command, false
	}
	if s.format.kind == eventFE6 {
		return s.fe6Instruction(at, command)
	}
	return s.fe7Instruction(at, command)
}

func (s *deploymentScanner) fe6Instruction(
	at int,
	command deploymentInstruction,
) (deploymentInstruction, bool) {
	op := s.data[at]
	if s.format.kind == eventFE6 {
		switch op {
		case 0x00, 0x01:
			command.end = true
		case 0x12, 0x13:
			command.loads = []int{at + 4}
		case 0x1B:
			command.label = int(u32(s.data, at+4))
		case 0x1C, 0x1D, 0x1E, 0x1F, 0x20, 0x21, 0x22, 0x23, 0x24:
			command.branch = int(u32(s.data, at+4))
		case 0x3F:
			command.mapID = int(u32(s.data, at+4))
		case 0x3D:
			command.chapterEnd = true
		case 0x25:
			command.call, command.jump = at+4, true
		}
		return command, true
	}
	return command, false
}

func (s *deploymentScanner) fe7Instruction(
	at int,
	command deploymentInstruction,
) (deploymentInstruction, bool) {
	op := s.data[at]
	shift := byte(0)
	call := byte(0xCC)
	if s.format.kind == eventFE7U {
		shift, call = 2, 0xCB
	}
	switch op {
	case 0x00, 0x01:
		command.end = true
	case call:
		command.call = at + 4
	case 0x30 + shift, 0x31 + shift, 0x34 + shift:
		command.loads = []int{at + 4}
	case 0x32 + shift:
		command.loads = []int{at + 8}
	case 0x35 + shift:
		command.loads = []int{at + 8}
	case 0x33 + shift, 0x36 + shift:
		command.loads = []int{at + 4, at + 8, at + 12, at + 16}
	case 0x42 + shift:
		command.label = int(u32(s.data, at+4))
	case 0x43 + shift,
		0x44 + shift,
		0x45 + shift,
		0x46 + shift,
		0x47 + shift,
		0x48 + shift,
		0x49 + shift,
		0x4A + shift,
		0x4B + shift,
		0x4C + shift,
		0x4D + shift,
		0x4E + shift,
		0x4F + shift,
		0x50 + shift,
		0x51 + shift,
		0x52 + shift,
		0x53 + shift,
		0x54 + shift,
		0x55 + shift:
		command.branch = int(u32(s.data, at+4))
	case 0x1B + shift:
		command.mapID = int(u32(s.data, at+4))
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
		s.result.ranges = append(
			s.result.ranges,
			space.Range{Start: at, End: at + command.length},
		)
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
		if u32(s.data, site)&0x80000000 != 0 && s.format.kind == eventFE8 {
			site = slots.slots[2]
		}
		if site > 0 && within(s.data, site, 4) && u32(s.data, site) != 0 {
			s.add(site, source)
		} else if site == 0 {
			s.warn(at)
		}
	}
	if command.call < 0 {
		if s.format.kind == eventFE8 {
			slots.observe(s.data[at:at+command.length], at+4)
		}
		return mapID
	}
	if target, valid := pointer(s.data, command.call); valid {
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
	root := s.format.loadHelper
	if root == 0 ||
		(target != root && target != root+0x2C && target != root+0x54) {
		return false
	}
	// These native LOAD helpers receive pointers in slots 2 and 3. The value
	// 1 means to reuse an existing group and is not a pointer site to rewrite.
	if mapID == s.chapter {
		for _, slot := range []int{2, 3} {
			if site := slots.slots[slot]; site != 0 {
				if _, valid := pointer(s.data, site); valid {
					s.add(site, source)
				}
			}
		}
	}
	return true
}
