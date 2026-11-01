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

// Claims declares the typed chapter resources a damaged unrelated table must
// not overwrite: event list roots and headers, condition tables and their
// script operand sites, decoded script code, referenced location payloads and
// deployment groups. The chapter table comes from
// the session's effective layout so manual layouts protect their real rows.
func (g Config) Claims(data []byte, chapters *records.Table) []space.Claim {
	var claims []space.Claim
	add := func(start, length int, owner space.Owner) {
		if start >= 0xC0 && length > 0 && gba.Within(data, start, length) {
			claims = append(
				claims,
				space.Claim{
					Start: start, End: start + length,
					Owner: owner,
				},
			)
		}
	}
	addSite := func(site int, owner, host space.Owner) {
		if site >= 0xC0 && gba.Within(data, site, 4) {
			claims = append(
				claims,
				space.Claim{
					Start: site, End: site + 4,
					Owner: owner,
					Host:  host,
					Site:  true,
				},
			)
		}
	}
	addSite(g.Events.Root, event.RootOwner(), space.Owner{})
	list, valid := gba.Pointer(data, g.Events.Root)
	if !valid || chapters == nil || chapters.Err != nil {
		return claims
	}
	seen := make(map[int]bool)
	for _, row := range chapters.Rows {
		header := -1
		headerOwner := space.Owner{}
		for _, field := range chapters.Fields {
			if field.ID != event.HeaderField {
				continue
			}
			entry := list + int(field.Value(data, row.Address))*4
			if at, ok := gba.Pointer(data, entry); ok {
				// Chapters whose entries point at one header declare identical
				// claims; the header address is the shared owner's identity.
				header = at
				headerOwner = event.HeaderOwner(header)
				add(header, g.Events.HeaderSize(), headerOwner)
				if opening := g.Events.Opening(); opening >= 0 {
					addSite(
						header+opening,
						event.ScriptOwner(header+opening),
						headerOwner,
					)
				}
				addSite(
					header+g.Events.EndingOffset(),
					event.ScriptOwner(header+g.Events.EndingOffset()),
					headerOwner,
				)
			}
			addSite(entry, event.HeaderOwner(entry), space.Owner{})
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
			owner := event.ConditionsOwner(conditions.Pointer, kind)
			addSite(conditions.Pointer, owner, headerOwner)
			if region, ok := conditions.PayloadRegion(); ok {
				add(region.Start, region.End-region.Start, owner)
			}
			claims = append(claims, conditions.ScriptClaims()...)
			for _, region := range conditions.ResourceRegions() {
				add(region.Start, region.End-region.Start, space.Owner{
					Resource: "event.location_resource", ID: region.Start,
				})
			}
			for _, condition := range conditions.Events {
				// Script operand sites are declared so a script relocation may
				// rewrite them; the condition list payload hosts them.
				if condition.OperandKind == event.ConditionScript {
					site := condition.Address + 4
					addSite(site, event.ScriptOwner(site), owner)
				}
				groups = append(groups, condition.Groups...)
			}
			if ending, ok := conditions.Ending(); ok {
				groups = append(groups, ending.Groups...)
			}
		}
		deployments, err := g.Deployments(data, row.ID, row.Address)
		if err == nil {
			claims = append(claims, deployments.ScriptClaims()...)
			groups = append(groups, deployments.Groups...)
		}
		for _, group := range groups {
			host := event.ScriptCodeOwner()
			if header >= 0 && group.Pointer >= header &&
				group.Pointer+4 <= header+g.Events.HeaderSize() {
				host = headerOwner
			}
			addSite(group.Pointer, deployment.Owner(group.Pointer), host)
			if group.Address == 0 || seen[group.Address] {
				continue
			}
			seen[group.Address] = true
			for _, region := range group.Regions() {
				add(
					region.Start,
					region.End-region.Start,
					deployment.Owner(group.Pointer),
				)
			}
		}
	}
	return claims
}

// checkResourcePointer is the minimal record-table guard used when a config
// has no session claim check, such as in isolated coordinator tests.
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
