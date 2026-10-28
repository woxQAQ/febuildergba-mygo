package game

func (f deploymentFormat) scriptASM(raw []byte) bool {
	switch f.kind {
	case eventFE8:
		return len(raw) == 8 && raw[1] == 0x0D
	case eventFE7U:
		return len(raw) == 8 && raw[0] == 0x3E
	case eventFE7J:
		return len(raw) == 8 && raw[0] == 0x3C
	case eventFE6:
		return len(raw) == 8 && raw[0] == 0x17
	}
	return false
}

func (row *scriptRow) describe(definitions []scriptDefinition, at int) {
	for index := range definitions {
		if definitions[index].matches(row.instruction.Raw) {
			row.definition = &definitions[index]
			row.instruction.Op = row.definition.op
			row.instruction.Editable = true
			break
		}
	}
	if row.definition == nil {
		return
	}
	for _, field := range row.definition.fields {
		if field.ID == ScriptLabelID && row.definition.label {
			row.label = field.value(row.instruction.Raw)
		}
		if field.ID == ScriptLabelID && row.definition.branch {
			row.branch = field.value(row.instruction.Raw)
		}
		if field.ID == ScriptPointer {
			row.call = at + field.offset
		}
	}
}

func scriptDeploymentOverlap(
	groups []DeploymentGroup,
	overlap func(int, int) bool,
) bool {
	for _, group := range groups {
		if overlap(
			group.Address,
			len(group.records)*group.Stride+len(group.terminator),
		) {
			return true
		}
		for _, record := range group.records {
			if len(record.data) == 20 && len(record.movement) > 0 &&
				overlap(
					int(u32(record.data, 8)&0x01FFFFFF),
					len(record.movement),
				) {
				return true
			}
		}
	}
	return false
}
