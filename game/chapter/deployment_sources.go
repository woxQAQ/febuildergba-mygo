package chapter

import (
	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
)

func (s *Editor) deploymentSource() deployment.Source {
	return deployment.Source{
		Candidates: s.deploymentCandidates,
		Uses:       s.deploymentUses,
	}
}

func (s *Editor) deploymentCandidates(
	data []byte,
	chapter int,
) ([]deployment.Group, error) {
	_, row, _, err := s.records.ResolveSnapshot(
		data,
		records.Chapters,
		chapter,
		event.HeaderField,
	)
	if err != nil {
		return nil, err
	}
	discovered, err := s.format.Deployments(data, chapter, row.Address)
	if err != nil {
		return nil, err
	}
	groups := discovered.Groups
	for _, kind := range event.Tables() {
		conditions, err := s.format.Conditions(data, chapter, row.Address, kind)
		if err != nil {
			continue
		}
		for _, event := range conditions.Events {
			groups = append(groups, event.Groups...)
		}
	}
	return groups, nil
}

func (s *Editor) deploymentUses(data []byte, site int) []deployment.Use {
	var uses []deployment.Use
	for _, table := range s.records.Inspect(data).Tables {
		if table.Kind != records.Chapters || table.Err != nil {
			continue
		}
		for _, row := range table.Rows {
			groups, err := s.format.Deployments(data, row.ID, row.Address)
			if err != nil {
				continue
			}
			for _, group := range groups.Groups {
				if group.Pointer == site {
					uses = append(
						uses,
						deployment.Use{Chapter: row.ID, Label: group.Label},
					)
				}
			}
		}
	}
	return uses
}
