package chapter

import (
	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func (g Config) Deployments(
	data []byte,
	chapter, address int,
) (event.Deployments, error) {
	for _, field := range g.Table.fields {
		if field.ID == event.HeaderField &&
			gba.Within(data, address+field.Offset, 1) {
			return g.Events.ReadDeployments(
				data,
				chapter,
				data[address+field.Offset],
			)
		}
	}
	return event.Deployments{}, edit.ErrLayout
}

func (g Config) Conditions(
	data []byte,
	chapter, address int,
	table event.Table,
) (event.Conditions, error) {
	if table > event.LocationEvents {
		return event.Conditions{}, event.ErrEventConditions
	}
	for _, field := range g.Table.fields {
		if field.ID == event.HeaderField &&
			gba.Within(data, address+field.Offset, 1) {
			return g.Events.ReadConditions(
				data,
				chapter,
				data[address+field.Offset],
				table,
			)
		}
	}
	return event.Conditions{}, edit.ErrLayout
}

// Ranges protects typed chapter resources and external pointer sites from
// reuse or from a damaged unrelated table targeting their live bytes.
func (g Config) Ranges(data []byte) []space.Range {
	var result []space.Range
	if g.External != nil {
		result = append(result, g.External(data)...)
	}
	add := func(start, length int) {
		if start >= 0xC0 && length > 0 && gba.Within(data, start, length) {
			result = append(
				result,
				space.Range{Start: start, End: start + length},
			)
		}
	}
	add(g.Events.Root(), 4)
	list, valid := gba.Pointer(data, g.Events.Root())
	if !valid {
		return result
	}
	seen := make(map[int]bool)
	textTable, _ := g.Text.Table(data)
	rule := g.Table.Rule(data, textTable.Count)
	table := rule.Read(
		data,
		records.Layout{},
		false,
		func(uint16) (string, error) { return "", nil },
	)
	if table.Err != nil {
		return result
	}
	for _, row := range table.Rows {
		for _, field := range table.Fields {
			if field.ID == event.HeaderField {
				entry := list + int(field.Value(data, row.Address))*4
				add(entry, 4)
				if header, valid := gba.Pointer(data, entry); valid {
					add(header, g.Events.HeaderSize())
				}
			}
		}
		var groups []deployment.Group
		for _, kind := range event.Tables() {
			conditions, err := g.Conditions(
				data,
				row.ID,
				row.Address,
				kind,
			)
			if err != nil {
				continue
			}
			result = append(result, conditions.Regions()...)
			if ending, ok := conditions.Ending(); ok {
				groups = append(groups, ending.Groups...)
			}
			for _, event := range conditions.Events {
				groups = append(groups, event.Groups...)
			}
		}
		deployment, err := g.Deployments(data, row.ID, row.Address)
		if err == nil {
			result = append(result, deployment.Regions()...)
			groups = append(groups, deployment.Groups...)
		}
		for _, group := range groups {
			add(group.Pointer, 4)
			if group.Address == 0 || seen[group.Address] {
				continue
			}
			seen[group.Address] = true
			for _, region := range group.Regions() {
				add(region.Start, region.End-region.Start)
			}
		}
	}
	return result
}

func checkResourcePointer(data []byte, tables []records.Table, site int) error {
	if site < 0xC0 || site%4 != 0 || !gba.Within(data, site, 4) {
		return event.ErrEventConditions
	}
	for _, table := range tables {
		if table.Pointer >= 0 && site < table.Pointer+4 &&
			site+4 > table.Pointer {
			return event.ErrEventConditions
		}
		if table.Err == nil {
			for _, row := range table.Rows {
				if site < row.Address+table.Stride && site+4 > row.Address {
					return event.ErrEventConditions
				}
			}
		}
	}
	return nil
}
