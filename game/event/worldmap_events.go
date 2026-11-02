package event

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// WorldMapScripts selects the independent native event directory. A world-map
// entry's associated chapter is navigation metadata, never a battle-map context.
const (
	WorldMapScripts    = -1
	WorldMapEventField = "world_map_events_id"
	WorldMapEventCount = 59
)

var ErrWorldMapEvents = fault.Code{
	ID:   "game.invalid_worldmap_events",
	Kind: fault.InvalidData,
}

// WorldMapEventsFormat contains pointer sites, not table addresses. The opening
// scene is beginning entry 1; the two endings have independent native callers.
type WorldMapEventsFormat struct {
	Beginning, Intro            int
	EirikaEnding, EphraimEnding int
}

type WorldMapEvent struct {
	Entry    ScriptEntry
	Chapters []int
}

func (s *Editor) WorldMapEventsAvailable() bool {
	return s.format.WorldMapEvents.Beginning != 0 &&
		s.format.WorldMapEvents.Intro != 0
}

func (f Format) worldMapEntries(data []byte) []ScriptEntry {
	if f.WorldMapEvents.Beginning == 0 || f.WorldMapEvents.Intro == 0 {
		return nil
	}
	var entries []ScriptEntry
	for _, table := range []struct {
		site   int
		source string
	}{
		{f.WorldMapEvents.Beginning, "script_worldmap_beginning"},
		{f.WorldMapEvents.Intro, "script_worldmap_intro"},
	} {
		base, ok := gba.Pointer(data, table.site)
		if !ok || base < 0xC0 || base%4 != 0 ||
			!gba.Within(data, base, WorldMapEventCount*4) {
			entries = append(
				entries,
				ScriptEntry{
					Chapter:  WorldMapScripts,
					Pointer:  table.site,
					Source:   table.source,
					WorldMap: true,
					EventID:  -1,
					Err:      ErrWorldMapEvents,
				},
			)
			continue
		}
		for id := range WorldMapEventCount {
			entry := scriptEntry(data, WorldMapScripts, base+id*4, table.source)
			entry.WorldMap, entry.EventID = true, id
			entries = append(entries, entry)
		}
	}
	for _, ending := range []struct {
		site   int
		source string
	}{
		{f.WorldMapEvents.EirikaEnding, "script_worldmap_ending_eirika"},
		{f.WorldMapEvents.EphraimEnding, "script_worldmap_ending_ephraim"},
	} {
		if ending.site == 0 {
			continue
		}
		entry := scriptEntry(data, WorldMapScripts, ending.site, ending.source)
		entry.EventID = -1
		entries = append(entries, entry)
	}
	return entries
}

func (s *Editor) WorldMapEvents() ([]WorldMapEvent, error) {
	format, err := s.scriptFormat()
	if err != nil {
		return nil, err
	}
	if !s.WorldMapEventsAvailable() {
		return nil, edit.ErrField
	}
	entries := format.worldMapEntries(s.image.Data)
	var result []WorldMapEvent
	table := s.records.Table(records.Chapters)
	for _, entry := range entries {
		item := WorldMapEvent{Entry: entry}
		if entry.WorldMap && entry.EventID >= 0 && table != nil &&
			table.Err == nil {
			for _, field := range table.Fields {
				if field.ID == WorldMapEventField {
					for _, row := range table.Rows {
						if int(
							field.Value(s.image.Data, row.Address),
						) == entry.EventID {
							item.Chapters = append(item.Chapters, row.ID)
						}
					}
				}
			}
		}
		result = append(result, item)
	}
	return result, nil
}

// WorldMapClaims protects native directories even without a readable chapter
// table. Calls and decoded payloads share the ordinary event-script ownership.
func (f Format) WorldMapClaims(data []byte) []space.Claim {
	if f.WorldMapEvents.Beginning == 0 || f.WorldMapEvents.Intro == 0 {
		return nil
	}
	var claims []space.Claim
	for _, site := range []int{f.WorldMapEvents.Beginning, f.WorldMapEvents.Intro} {
		if site >= 0xC0 && gba.Within(data, site, 4) {
			owner := space.Owner{Resource: "event.worldmap_root", ID: site}
			claims = append(
				claims,
				space.Claim{
					Start: site,
					End:   site + 4,
					Owner: owner,
					Site:  true,
				},
			)
			base, ok := gba.Pointer(data, site)
			if ok && base >= 0xC0 &&
				(base%4 != 0 || !gba.Within(data, base, WorldMapEventCount*4)) &&
				base < len(data) {
				claims = append(
					claims,
					space.Claim{
						Start: base,
						End:   min(base+WorldMapEventCount*4, len(data)),
						Owner: owner,
					},
				)
			}
		}
	}
	entries := f.worldMapEntries(data)
	index := f.indexScripts(data, entries)
	for _, entry := range index.entries {
		if entry.Pointer < 0xC0 || !gba.Within(data, entry.Pointer, 4) {
			continue
		}
		if entry.EventID < 0 && entry.WorldMap && entry.Err != nil {
			continue
		}
		if entry.Address >= 0xC0 && gba.Within(data, entry.Address, 4) {
			claims = append(
				claims,
				space.Claim{
					Start: entry.Address,
					End:   entry.Address + 4,
					Owner: ScriptCodeOwner(),
				},
			)
		}
		host := space.Owner{}
		if entry.Source == "script_call_entry" {
			host = ScriptCodeOwner()
		}
		claims = append(
			claims,
			space.Claim{
				Start: entry.Pointer,
				End:   entry.Pointer + 4,
				Owner: ScriptOwner(entry.Pointer),
				Host:  host,
				Site:  true,
			},
		)
	}
	for _, program := range index.programs {
		for _, row := range program.rows {
			claims = append(
				claims,
				space.Claim{
					Start: row.instruction.Address,
					End:   row.instruction.Address + len(row.instruction.Raw),
					Owner: ScriptCodeOwner(),
				},
			)
		}
	}
	return claims
}

// WorldMapUses lists declared directory entries reaching a confirmed node or
// route operand. Unknown instructions and dynamic calls make the search partial;
// absence from the result is not a proof that a resource is unused.
func (s *Editor) WorldMapUses(
	target ScriptTarget,
	id int,
) ([]WorldMapEvent, bool, error) {
	entries, err := s.WorldMapEvents()
	if err != nil {
		return nil, false, err
	}
	index, err := s.scriptIndex(WorldMapScripts)
	if err != nil {
		return nil, false, err
	}
	var result []WorldMapEvent
	complete := !index.partial
	for _, entry := range entries {
		pending := []int{entry.Entry.Address}
		seen := map[int]bool{}
		matched := false
		if entry.Entry.Err != nil {
			complete = false
			continue
		}
		for len(pending) > 0 && len(seen) < 512 {
			at := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			if at == 0 || seen[at] {
				continue
			}
			seen[at] = true
			program, ok := index.programs[at]
			if !ok {
				complete = false
				continue
			}
			if !program.Complete {
				complete = false
			}
			for _, row := range program.rows {
				if row.definition == nil {
					complete = false
					continue
				}
				for _, field := range row.definition.fields {
					value := field.value(row.instruction.Raw)
					if field.Target == target && value == int64(id) {
						matched = true
					}
					if field.Target == ScriptEventTarget {
						if value < 0x080000C0 || value >= 0x0A000000 {
							complete = false
						} else {
							pending = append(pending, int(value)-0x08000000)
						}
					}
				}
			}
		}
		if len(pending) > 0 {
			complete = false
		}
		if matched {
			entry.Chapters = slices.Clone(entry.Chapters)
			result = append(result, entry)
		}
	}
	return result, complete, nil
}
