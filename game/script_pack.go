package game

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
	for index, row := range d.rows {
		if row.definition == nil {
			return nil, nil, ErrScriptStructure
		}
		for _, field := range row.definition.fields {
			value := field.value(row.instruction.Raw)
			if (field.ID == ScriptPointer || field.ID == ScriptUnitPointer) &&
				(value < 0x080000C0 || value >= 0x0A000000) {
				return nil, nil, ErrScriptStructure
			}
			if field.ID == ScriptSlotValue &&
				value >= int64(0x08000000+entry.Address) &&
				value < int64(0x08000000+end) {
				return nil, nil, ErrScriptStructure
			}
			if field.ID != ScriptPointer {
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
