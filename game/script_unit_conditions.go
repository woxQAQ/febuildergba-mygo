package game

const (
	ScriptCheckUnitExists      ScriptOp = "script_check_unit_exists"
	ScriptCheckUnitAlive       ScriptOp = "script_check_unit_alive"
	ScriptCheckUnitDeployed    ScriptOp = "script_check_unit_deployed"
	ScriptCheckUnitFaction     ScriptOp = "script_check_unit_faction"
	ScriptBranchNoAliveAlly    ScriptOp = "script_branch_no_alive_ally"
	ScriptBranchNoDeployedAlly ScriptOp = "script_branch_no_deployed_ally"
)

// FE8 queries write slot C without branching. FE6/FE7 instead branch when the
// named character is absent from the living or deployed player-unit roster.
func (f deploymentFormat) unitConditionDefinitions() []scriptDefinition {
	if f.kind == eventFE8 {
		var result []scriptDefinition
		for _, entry := range []struct {
			op   ScriptOp
			code byte
			help string
		}{
			{ScriptCheckUnitExists, 0x20, "script_check_unit_exists_help"},
			{ScriptCheckUnitAlive, 0x22, "script_check_unit_alive_help"},
			{ScriptCheckUnitDeployed, 0x23, "script_check_unit_deployed_help"},
			{ScriptCheckUnitFaction, 0x25, "script_check_unit_faction_help"},
		} {
			result = append(result, scriptDefinition{
				op: entry.op, raw: []byte{entry.code, 0x33, 1, 0},
				fields: []ScriptField{scriptFE8Character(2)}, help: entry.help,
			})
		}
		return result
	}
	code := uint32(0x44)
	if f.kind == eventFE7U {
		code = 0x46
	}
	if f.kind == eventFE6 {
		code = 0x1D
	}
	fields := []ScriptField{
		scriptOperand(ScriptLabelID, 4, 4, 0xFFFFFFFF),
		sceneOperand(ScriptCharacter, 8, 2, 0, 0xFF),
	}
	return []scriptDefinition{
		{
			op:     ScriptBranchNoAliveAlly,
			raw:    scriptWords(code, 0, 1),
			fields: fields,
			branch: true,
			help:   "script_branch_no_alive_ally_help",
		},
		{
			op:     ScriptBranchNoDeployedAlly,
			raw:    scriptWords(code+1, 0, 1),
			fields: fields,
			branch: true,
			help:   "script_branch_no_deployed_ally_help",
		},
	}
}
