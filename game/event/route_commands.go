package event

const (
	ScriptLoadUnitsAlive    ScriptOp = "script_load_units_alive"
	ScriptLoadUnitsParty    ScriptOp = "script_load_units_party"
	ScriptLoadUnitsFiltered ScriptOp = "script_load_units_filtered"
	ScriptLoadUnitsByRoute  ScriptOp = "script_load_units_by_route"
	ScriptLoadPartyFiltered ScriptOp = "script_load_party_filtered"
	ScriptLoadPartyByRoute  ScriptOp = "script_load_party_by_route"
	ScriptTextByRoute       ScriptOp = "script_text_by_route"
	ScriptMoreTextByRoute   ScriptOp = "script_more_text_by_route"
	ScriptTextOpaque        ScriptOp = "script_text_opaque"
)

const (
	ScriptRoute              = "script_route"
	ScriptHardOnly           = "script_hard_only"
	ScriptUnitsPrimaryNormal = "script_units_primary_normal"
	ScriptUnitsPrimaryHard   = "script_units_primary_hard"
	ScriptUnitsHectorNormal  = "script_units_hector_normal"
	ScriptUnitsHectorHard    = "script_units_hector_hard"
	ScriptTextPrimary        = "script_text_primary"
	ScriptTextHector         = "script_text_hector"
)

func (f Format) wordVariantDefinitions() []scriptDefinition {
	if f.kind == eventFE8 {
		return nil
	}
	if f.kind == eventFE6 {
		return []scriptDefinition{
			{
				op:  ScriptLoadUnitsParty,
				raw: scriptWords(0x13, 0),
				fields: []ScriptField{
					scriptOperand(ScriptUnitPointer, 4, 4, 0xFFFFFFFF),
				},
				help: "script_load_party_help",
			},
		}
	}
	shift := uint32(0)
	if f.kind == eventFE7U {
		shift = 2
	}
	var result []scriptDefinition
	for _, entry := range []struct {
		op   ScriptOp
		code uint32
		help string
	}{
		{ScriptLoadUnitsAlive, 0x31, "script_load_alive_help"},
		{ScriptLoadUnitsParty, 0x34, "script_load_party_help"},
	} {
		result = append(
			result,
			scriptDefinition{
				op:  entry.op,
				raw: scriptWords(entry.code+shift, 0),
				fields: []ScriptField{
					scriptOperand(ScriptUnitPointer, 4, 4, 0xFFFFFFFF),
				},
				help: entry.help,
			},
		)
	}
	for _, entry := range []struct {
		op   ScriptOp
		code uint32
	}{
		{ScriptLoadUnitsFiltered, 0x32}, {ScriptLoadPartyFiltered, 0x35},
	} {
		fields := []ScriptField{
			scriptOperand(ScriptRoute, 4, 1, 3),
			scriptOperand(ScriptUnitPointer, 8, 4, 0xFFFFFFFF),
		}
		if entry.op == ScriptLoadUnitsFiltered {
			fields = append(fields, scriptOperand(ScriptHardOnly, 6, 2, 1))
		}
		result = append(
			result,
			scriptDefinition{
				op:     entry.op,
				raw:    scriptWords(entry.code+shift, 1, 0),
				fields: fields,
				help:   "script_load_route_filter_help",
			},
		)
	}
	for _, entry := range []struct {
		op   ScriptOp
		code uint32
	}{
		{ScriptLoadUnitsByRoute, 0x33}, {ScriptLoadPartyByRoute, 0x36},
	} {
		var fields []ScriptField
		for index, id := range []string{ScriptUnitsPrimaryNormal, ScriptUnitsPrimaryHard, ScriptUnitsHectorNormal, ScriptUnitsHectorHard} {
			field := scriptOperand(ScriptUnitPointer, 4+index*4, 4, 0xFFFFFFFF)
			field.ID = id
			field.Optional = entry.op == ScriptLoadUnitsByRoute
			fields = append(fields, field)
		}
		result = append(
			result,
			scriptDefinition{
				op:     entry.op,
				raw:    scriptWords(entry.code+shift, 0, 0, 0, 0),
				fields: fields,
				help:   "script_load_route_choices_help",
			},
		)
	}
	for _, entry := range []struct {
		op   ScriptOp
		code uint32
	}{
		{ScriptTextByRoute, 0x0F}, {ScriptMoreTextByRoute, 0x12},
	} {
		primary, hector := scriptOperand(
			ScriptTextID,
			4,
			2,
			0xFFFF,
		), scriptOperand(
			ScriptTextID,
			8,
			2,
			0xFFFF,
		)
		primary.ID, hector.ID = ScriptTextPrimary, ScriptTextHector
		result = append(
			result,
			scriptDefinition{
				op:     entry.op,
				raw:    scriptWords(entry.code, 1, 1),
				fields: []ScriptField{primary, hector},
				help:   "script_text_route_help",
			},
		)
	}
	return append(
		result,
		scriptDefinition{
			op:     ScriptTextOpaque,
			raw:    scriptWords(0x0E, 1),
			fields: []ScriptField{scriptOperand(ScriptTextID, 4, 2, 0xFFFF)},
			help:   "script_text_opaque_help",
		},
	)
}
