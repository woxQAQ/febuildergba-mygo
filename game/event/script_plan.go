package event

import (
	"bytes"
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

type ScriptUse struct {
	Chapter, Pointer, Address int
	Source                    string
}

// ScriptPlan is one immutable parameter patch or append-and-pointer operation.
type ScriptPlan struct {
	commit                      *edit.Plan
	source, destination, length int
	relocated                   bool
	before, after               int
	uses                        []ScriptUse
	movements                   int
}

func (plan *ScriptPlan) MovementCount() int { return plan.movements }

func (plan *ScriptPlan) Source() int { return plan.source }

func (plan *ScriptPlan) Destination() int { return plan.destination }

func (plan *ScriptPlan) Length() int { return plan.length }

func (plan *ScriptPlan) Relocated() bool { return plan.relocated }

func (plan *ScriptPlan) Before() int { return plan.before }

func (plan *ScriptPlan) After() int { return plan.after }

func (plan *ScriptPlan) Uses() []ScriptUse { return slices.Clone(plan.uses) }

func (s *Editor) PlanScriptEdit(
	draft *ScriptDraft,
) (result *ScriptPlan, err error) {
	defer func() { err = fault.Wrap(err, "game.plan_script", fault.Context{Resource: scriptResource}) }()
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
	if !draft.Complete() {
		return nil, ErrScriptStructure
	}
	entry := draft.Entry()
	current, err := s.Script(entry.Chapter, entry.Pointer)
	if err != nil {
		return nil, err
	}
	if current.Entry.Address != entry.Address {
		return nil, rom.ErrStale
	}
	appender := space.NewAppender(len(s.image.Data))
	draft, movements, err := draft.prepareMovements(appender)
	if err != nil {
		return nil, err
	}
	structural := draft.structural()
	if structural && !draft.Relocatable() {
		return nil, ErrScriptStructure
	}
	if draft.labelsChanged(structural) {
		if err = draft.validateLabels(); err != nil {
			return nil, err
		}
	}
	destination := entry.Address
	if structural {
		destination = appender.Next()
	}
	content, positions, err := draft.scriptContent(destination, structural)
	if err != nil {
		return nil, err
	}
	write, err := s.scriptWritePlan(
		draft,
		appender,
		destination,
		content,
		structural,
	)
	if err != nil {
		return nil, err
	}
	prospective := bytes.Repeat([]byte{0xFF}, write.Edit.Size)
	copy(prospective, s.image.Data)
	for _, change := range write.Edit.Changes {
		copy(prospective[change.Offset:], change.Data)
	}
	if err = s.validateScriptEdit(
		draft,
		prospective,
		positions,
		destination,
	); err != nil {
		return nil, err
	}
	result = &ScriptPlan{
		commit:      edit.NewPlan(draft.binding, scriptResource, write.Edit),
		source:      entry.Address,
		destination: destination,
		length:      len(content),
		movements:   movements,
		relocated:   structural,
		before:      len(draft.original.rows),
		after:       len(draft.rows),
	}
	result.uses = s.scriptUses(entry, structural, draft.originalLength())
	if err = s.validateSharedMapChangeScript(
		draft,
		result.uses,
		structural,
	); err != nil {
		return nil, err
	}
	if err = draft.binding.Check(s.context); err != nil {
		return nil, err
	}
	return result, nil
}

func (d *ScriptDraft) labelsChanged(structural bool) bool {
	if structural {
		return true
	}
	for index, row := range d.rows {
		if row.definition != nil &&
			(row.definition.label || row.definition.branch) &&
			!bytes.Equal(
				row.instruction.Raw,
				d.original.rows[index].instruction.Raw,
			) {
			return true
		}
	}
	return false
}

func (s *Editor) scriptWritePlan(
	draft *ScriptDraft,
	appender *space.Appender,
	destination int,
	content []byte,
	structural bool,
) (space.Plan, error) {
	entry := draft.Entry()
	if structural {
		if err := s.resourcePointer(
			ScriptOwner(entry.Pointer),
			entry.Pointer,
		); err != nil {
			return space.Plan{}, err
		}
		if _, err := appender.Add(content, entry.Pointer); err != nil {
			return space.Plan{}, err
		}
		return appender.Plan(), nil
	}
	if err := s.scriptWriteRange(
		entry.Address,
		entry.Address+len(content),
	); err != nil {
		return space.Plan{}, err
	}
	plan := appender.Plan()
	for index, row := range draft.rows {
		if !bytes.Equal(
			row.instruction.Raw,
			draft.original.rows[index].instruction.Raw,
		) {
			plan.Edit.Changes = append(
				plan.Edit.Changes,
				rom.Change{
					Offset: row.instruction.Address,
					Data:   bytes.Clone(row.instruction.Raw),
				},
			)
		}
	}
	return plan, nil
}

func (s *Editor) validateScriptEdit(
	draft *ScriptDraft,
	data []byte,
	positions []int,
	destination int,
) error {
	decoded := draft.original.format.readScript(
		data,
		ScriptEntry{Address: destination},
	)
	if !decoded.Complete || len(decoded.rows) != len(draft.rows) {
		return ErrScript
	}
	structural := draft.structural()
	originals := make(map[uint64]scriptRow)
	for _, row := range draft.original.rows {
		originals[row.instruction.ID] = row
	}
	for index, row := range draft.rows {
		if row.definition == nil {
			continue
		}
		for _, field := range row.definition.fields {
			old, found := originals[row.instruction.ID]
			if found && (!structural || field.Target != ScriptMovementTarget) &&
				field.value(
					old.instruction.Raw,
				) == field.value(
					row.instruction.Raw,
				) {
				continue
			}
			if err := s.validateScriptOperand(
				data,
				positions[index],
				field,
				draft.original.format,
			); err != nil {
				return err
			}
		}
	}
	if err := s.validateSlotUnitLoads(draft, data, positions); err != nil {
		return err
	}
	return nil
}

func (s *Editor) validateScriptOperand(
	data []byte,
	address int,
	field ScriptField,
	format Format,
) error {
	value := field.value(data[address:])
	if value == 0 && field.Optional {
		return nil
	}
	switch field.Target {
	case ScriptMovementTarget:
		if value < 0x080000C0 || value >= 0x0A000000 {
			return ErrScriptStructure
		}
		_, err := readScriptMovement(data, int(value)-0x08000000)
		return err
	case ScriptTextTarget:
		if format.kind == eventFE8 && (value < 0 || value >= 0x8000) {
			return nil
		}
		_, err := s.DialogueText(
			uint16(value),
		) // #nosec G115 -- Text operands are at most 16 bits.
		return err
	case ScriptEventTarget:
		if format.kind == eventFE8 && value == 0xFFFFFFFF {
			return nil
		}
		target, valid := gba.Pointer(data, address+field.offset)
		if !valid || target < 0xC0 || target%4 != 0 {
			return ErrScript
		}
		if !format.readScript(data, ScriptEntry{Address: target}).Complete {
			return ErrScript
		}
	case ScriptDeploymentTarget:
		if format.kind == eventFE8 && value >= 0x80000000 {
			return nil
		}
		if value == 0 {
			return deployment.ErrDeployment
		}
		group := deployment.NewReader(data, format.units).Group(
			address+field.offset,
			"script_units",
			0,
		)
		return group.Err
	}
	return nil
}

func (s *Editor) scriptUses(
	entry ScriptEntry,
	relocated bool,
	length int,
) []ScriptUse {
	var result []ScriptUse
	table := s.records.Table(records.Chapters)
	if table == nil || table.Err != nil {
		return result
	}
	for _, chapter := range table.Rows {
		index, err := s.scriptIndex(chapter.ID)
		if err != nil {
			continue
		}
		for _, candidate := range index.entries {
			if candidate.Err != nil {
				continue
			}
			affected := candidate.Pointer == entry.Pointer
			if !relocated {
				program := index.programs[candidate.Address]
				programLength := 0
				for _, row := range program.rows {
					programLength += len(row.instruction.Raw)
				}
				affected = candidate.Address < entry.Address+length &&
					entry.Address < candidate.Address+programLength
			}
			if affected {
				result = append(
					result,
					ScriptUse{
						Chapter: chapter.ID,
						Pointer: candidate.Pointer,
						Address: candidate.Address,
						Source:  candidate.Source,
					},
				)
			}
		}
	}
	return result
}

func (s *Editor) ApplyScriptEdit(plan *ScriptPlan) (err error) {
	defer func() { err = fault.Wrap(err, "game.apply_script", fault.Context{Resource: scriptResource}) }()
	if plan == nil {
		return rom.ErrStale
	}
	if err = plan.commit.Apply(s.context); err != nil {
		return err
	}
	s.Refresh()
	return nil
}

// In-place code edits cannot claim bytes already described as record data,
// condition lists, event headers, deployment records or movement paths.
func (s *Editor) scriptWriteRange(start, end int) error {
	format, err := s.scriptFormat()
	if err != nil {
		return err
	}
	overlap := func(at, length int) bool { return length > 0 && start < at+length && at < end }
	for _, table := range s.records.Tables() {
		if overlap(table.Pointer, 4) {
			return edit.ErrLayout
		}
		if table.Err == nil &&
			overlap(table.Address, table.Count*table.Stride) {
			return edit.ErrLayout
		}
	}
	for _, at := range []int{s.text.Pointer, s.text.TreePointer, s.text.RootPointer, format.Root} {
		if overlap(at, 4) {
			return edit.ErrLayout
		}
	}
	if text, readErr := s.text.Table(
		s.image.Data,
	); readErr == nil &&
		overlap(text.Address, text.Count*4) {
		return edit.ErrLayout
	}
	chapters := s.records.Table(records.Chapters)
	if chapters == nil || chapters.Err != nil {
		return edit.ErrLayout
	}
	list, valid := gba.Pointer(s.image.Data, format.Root)
	if !valid {
		return edit.ErrLayout
	}
	for _, chapter := range chapters.Rows {
		value, valueErr := s.records.Value(
			records.Chapters,
			chapter.ID,
			HeaderField,
		)
		if valueErr != nil {
			continue
		}
		site := list + int(value)*4
		if overlap(site, 4) {
			return edit.ErrLayout
		}
		header, valid := gba.Pointer(s.image.Data, site)
		if !valid {
			continue
		}
		if overlap(header, format.ending+4) {
			return edit.ErrLayout
		}
		for _, table := range eventTables {
			conditions, readErr := format.readEvents(
				s.image.Data,
				header+table.offset(),
				table,
			)
			if readErr != nil {
				continue
			}
			for _, region := range conditions.ranges {
				if overlap(region.Start, region.End-region.Start) {
					return edit.ErrLayout
				}
			}
		}
		groups, readErr := s.Deployments(chapter.ID)
		if readErr != nil {
			continue
		}
		if scriptDeploymentOverlap(groups.Groups, overlap) {
			return edit.ErrLayout
		}
	}
	return nil
}

// scriptContent packs known rows and retargets declared calls into the same
// resource. Generic slot values cannot establish an internal pointer contract.
func (d *ScriptDraft) scriptContent(
	destination int,
	structural bool,
) ([]byte, []int, error) {
	var content []byte
	positions := make([]int, len(d.rows))
	addresses := make(map[int]int)
	for index, row := range d.rows {
		positions[index] = destination + len(content)
		if row.instruction.Address > 0 {
			addresses[row.instruction.Address] = positions[index]
		}
		content = append(content, row.instruction.Raw...)
	}
	if len(content) == 0 {
		return nil, nil, ErrScript
	}
	if !structural {
		return content, positions, nil
	}
	entry := d.Entry()
	end := entry.Address + d.originalLength()
	loadSources := d.unitLoadSources()
	for index, row := range d.rows {
		if row.definition == nil {
			return nil, nil, ErrScriptStructure
		}
		for _, field := range row.definition.fields {
			value := field.value(row.instruction.Raw)
			if field.Target == ScriptDeploymentTarget &&
				d.original.format.kind == eventFE8 &&
				slotUnitLoad(row.instruction.Raw) {
				source, ok := loadSources[index]
				if !ok {
					return nil, nil, ErrScriptStructure
				}
				value = int64(gba.Uint32(d.rows[source].instruction.Raw, 4))
			}
			if field.Optional && value == 0 {
				continue
			}
			if (field.Target == ScriptEventTarget || field.Target == ScriptDeploymentTarget || field.Target == ScriptMovementTarget) &&
				(value < 0x080000C0 || value >= 0x0A000000) {
				return nil, nil, ErrScriptStructure
			}
			if field.ID == ScriptSlotValue &&
				value >= int64(0x08000000+entry.Address) &&
				value < int64(0x08000000+end) {
				return nil, nil, ErrScriptStructure
			}
			if field.Target != ScriptEventTarget {
				continue
			}
			target := int(value) - 0x08000000
			if target < entry.Address || target >= end {
				continue
			}
			relocated, exists := addresses[target]
			if !exists {
				return nil, nil, ErrScript
			}
			field.put(
				content[positions[index]-destination:],
				int64(0x08000000+relocated),
			)
		}
	}
	return content, positions, nil
}
