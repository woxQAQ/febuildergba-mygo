package game

import (
	"encoding/binary"
	"maps"
)

func (s *Session) deploymentDraft(
	chapter int,
	group DeploymentGroup,
) *DeploymentDraft {
	if len(group.Units) > 0 {
		group.faction = group.Units[0].Faction
	}
	return &DeploymentDraft{
		document: s.Document,
		revision: s.image.Revision,
		layouts:  maps.Clone(s.layouts),
		chapter:  chapter,
		group:    group,
		records:  cloneDeploymentRecords(group.records),
	}
}

// payload keeps unmodified native paths shared and copies only changed paths.
// address is the aligned destination of this complete deployment payload.
func (d *DeploymentDraft) payload(address int) []byte {
	var content []byte
	for _, record := range d.records {
		content = append(content, record.data...)
	}
	content = append(content, d.group.terminator...)
	for i, record := range d.records {
		if !record.movementEdited || len(record.movement) == 0 {
			continue
		}
		for len(content)%4 != 0 {
			content = append(content, 0)
		}
		binary.LittleEndian.PutUint32(
			content[i*d.group.Stride+8:],
			uint32(0x08000000+address+len(content)),
		) // #nosec G115 -- PlanAppend validates the complete size before application.
		content = append(content, record.movement...)
	}
	return content
}
