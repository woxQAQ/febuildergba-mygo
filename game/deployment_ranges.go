package game

import "github.com/woxQAQ/febuildergba-mygo/space"

// deploymentRanges protects typed deployment resources and pointer sites from
// reuse or from a damaged unrelated table targeting their live bytes.
func (g gameFormat) deploymentRanges(data []byte) []space.Range {
	var result []space.Range
	add := func(start, length int) {
		if start >= 0xC0 && length > 0 && within(data, start, length) {
			result = append(
				result,
				space.Range{Start: start, End: start + length},
			)
		}
	}
	add(g.deployments.root, 4)
	list, valid := pointer(data, g.deployments.root)
	if !valid {
		return result
	}
	seen := make(map[int]bool)
	for _, rule := range g.tableRules(data) {
		if rule.kind != Chapters {
			continue
		}
		table := rule.read(
			data,
			Layout{},
			false,
			func(uint16) (string, error) { return "", nil },
		)
		if table.Err != nil {
			continue
		}
		for _, row := range table.Rows {
			for _, field := range table.Fields {
				if field.ID == chapterEventsField {
					entry := list + int(field.Value(data, row.Address))*4
					add(entry, 4)
					if header, valid := pointer(data, entry); valid {
						last := g.deployments.slots[len(g.deployments.slots)-1].offset + 4
						if g.deployments.opening >= 0 {
							last = g.deployments.opening + 8
						}
						add(header, max(last, g.deployments.ending+4))
					}
				}
			}
			var groups []DeploymentGroup
			for _, kind := range eventTables {
				conditions, err := g.ChapterEvents(
					data,
					row.ID,
					row.Address,
					kind,
				)
				if err != nil {
					continue
				}
				result = append(result, conditions.ranges...)
				if conditions.ending != nil {
					groups = append(groups, conditions.ending.Groups...)
				}
				for _, event := range conditions.Events {
					groups = append(groups, event.Groups...)
				}
			}
			deployment, err := g.ChapterDeployments(data, row.ID, row.Address)
			if err == nil {
				result = append(result, deployment.ranges...)
				groups = append(groups, deployment.Groups...)
			}
			for _, group := range groups {
				add(group.Pointer, 4)
				if group.Address == 0 || seen[group.Address] {
					continue
				}
				seen[group.Address] = true
				add(
					group.Address,
					len(group.records)*group.Stride+len(group.terminator),
				)
				for _, record := range group.records {
					if len(record.data) == 20 && len(record.movement) > 0 {
						at := int(u32(record.data, 8) & 0x01FFFFFF)
						add(at, len(record.movement))
					}
				}
			}
		}
	}
	return result
}
