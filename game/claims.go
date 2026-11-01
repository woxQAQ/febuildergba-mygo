package game

import (
	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/shop"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// recordOwner names a record table row in resource claims; ID -1 marks the
// table's declared root pointers.
func recordOwner(kind records.Kind, id int) space.Owner {
	return space.Owner{Resource: string(kind), ID: id}
}

// claims aggregates every domain's confirmed resource claims, cached by the
// bound document stamp. Callers pass the current snapshot; domain editors
// refresh before planning, so the cache cannot outlive the data it describes.
func (s *Session) claims(data []byte) []space.Claim {
	if s.claimsStamp.Current(s.context) {
		return s.claimsCache
	}
	claims := s.collectClaims(data)
	s.claimsStamp = s.context.Bind(s.context.Document().State().Revision)
	s.claimsCache = claims
	return claims
}

// protectPointer guards one pointer write against the session's claims. The
// result carries the caller-neutral conflict; domain sources translate it to
// their own stable code.
func (s *Session) protectPointer(
	data []byte,
	owner space.Owner,
	site int,
) error {
	if site < 0xC0 || site%4 != 0 || !gba.Within(data, site, 4) {
		return edit.ErrLayout
	}
	return space.CheckWrite(s.claims(data), owner, site)
}

// protectRange guards an in-place write with the same session claims as
// pointer updates. Payload bounds are checked before overlap authorization.
func (s *Session) protectRange(
	data []byte,
	owner space.Owner,
	region space.Range,
) error {
	if region.Start < 0xC0 || region.End <= region.Start ||
		!gba.Within(data, region.Start, region.End-region.Start) {
		return edit.ErrLayout
	}
	return space.CheckRangeWrite(s.claims(data), owner, region)
}

// guardPointer maps a rejected pointer write to the domain's code.
func (s *Session) guardPointer(
	data []byte,
	owner space.Owner,
	site int,
	code fault.Code,
) error {
	if err := s.protectPointer(data, owner, site); err != nil {
		return fault.New(
			code,
			"game.protect_pointer",
			fault.Context{
				Resource: owner.Resource,
				Range: &fault.Range{
					Offset: site,
					Length: 4,
					Size:   len(data),
				},
			},
			err,
		)
	}
	return nil
}

func (s *Session) collectClaims(data []byte) []space.Claim {
	g := s.format
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
	addSite := func(site int, owner space.Owner) {
		if site >= 0xC0 && gba.Within(data, site, 4) {
			claims = append(
				claims,
				space.Claim{
					Start: site, End: site + 4,
					Owner: owner,
					Site:  true,
				},
			)
		}
	}

	// Declared table root pointers stay claimed even when inspection fails.
	for _, rule := range g.tableRules(data) {
		for _, site := range rule.Pointers {
			addSite(site, recordOwner(rule.Kind, -1))
		}
	}
	// Record tables and their rows, under the effective layout.
	inspection := s.Records.Inspect(data)
	var chapters *records.Table
	for i := range inspection.Tables {
		table := &inspection.Tables[i]
		addSite(table.Pointer, recordOwner(table.Kind, -1))
		if table.Err != nil {
			continue
		}
		for _, row := range table.Rows {
			add(row.Address, table.Stride, recordOwner(table.Kind, row.ID))
		}
		if table.Kind == records.Chapters {
			chapters = table
		}
	}

	// Text and native image roots.
	textOwner := space.Owner{Resource: "text"}
	addSite(g.text.Pointer, textOwner)
	addSite(g.text.TreePointer, textOwner)
	addSite(g.text.RootPointer, textOwner)
	addSite(g.icons.Pointer, space.Owner{Resource: "itemicon"})
	addSite(g.icons.Palette, space.Owner{Resource: "itemicon"})
	addSite(g.portraits.Pointer, space.Owner{Resource: "portrait"})
	addSite(g.worldmap.Pointer, space.Owner{Resource: "worldmap", ID: -1})
	if table, err := g.text.Table(data); err == nil {
		add(table.Address, table.Count*4, textOwner)
	}

	// Loader shops and world-map node slots.
	claims = append(claims, g.shops.Claims(data)...)
	if nodes, err := g.worldmap.Nodes(data); err == nil {
		for _, node := range nodes {
			host := space.Owner{Resource: "worldmap", ID: node.ID}
			add(node.Address, worldmap.NodeSize, host)
			for slot, site := range node.Shops {
				owner := shop.Key{
					Origin: shop.WorldMap,
					ID:     node.ID,
					Slot:   slot,
				}.Owner()
				if gba.Within(data, site, 4) {
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
				if at, valid := gba.Pointer(
					data,
					site,
				); valid && at >= 0xC0 && at%2 == 0 &&
					gba.Within(data, at, shop.PayloadSize) {
					add(at, shop.PayloadSize, owner)
				}
			}
		}
	}

	// Battle and defeat dialog, supports and class terrain.
	claims = append(claims, g.quotes.Claims(data)...)
	claims = append(claims, g.arena.Claims(data)...)
	units, _ := s.supportUnits(data)
	claims = append(claims, g.supports.Claims(data, units)...)
	if classes, err := s.terrainClasses(data); err == nil {
		claims = append(claims, g.terrain.Claims(data, classes)...)
	}

	// Chapter resources: event headers, condition lists, script code and
	// sites, location payloads and deployment groups.
	if chapters != nil {
		claims = append(claims, g.chapterConfig().Claims(data, chapters)...)
	}
	return claims
}
