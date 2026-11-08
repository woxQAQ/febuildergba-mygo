package event

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

// WorldMapScripts selects the independent native event directory. A world-map
// entry's associated chapter is navigation metadata, never a battle-map context.
const (
	WorldMapScripts    = -1
	WorldMapEventField = "world_map_events_id"
)

var ErrWorldMapEvents = fault.Code{
	ID:   "game.invalid_worldmap_events",
	Kind: fault.InvalidData,
}

// WorldMapEventsFormat declares native directories and independent endings.
// Pointer values are root sites, never the current table or script addresses.
type WorldMapEventsFormat struct {
	// ChapterPLIST resolves only entries referenced by the effective chapter table.
	ChapterPLIST int
	Tables       []WorldMapEventTable
	Endings      []WorldMapEventEnding
}

type WorldMapEventTable struct {
	Pointer, Count int
	Source         string
}

type WorldMapEventEnding struct {
	Pointer int
	Source  string
}

type WorldMapEvent struct {
	Entry    ScriptEntry
	Chapters []int
}

func (s *Editor) WorldMapEventsAvailable() bool {
	return s.format.WorldMapEvents.ChapterPLIST != 0 ||
		len(s.format.WorldMapEvents.Tables) != 0 ||
		len(s.format.WorldMapEvents.Endings) != 0
}

func (f Format) worldMapEntries(
	data []byte,
	chapters *records.Table,
) []ScriptEntry {
	entries := f.chapterWorldMapEntries(data, chapters)
	for _, table := range f.WorldMapEvents.Tables {
		base, ok := gba.Pointer(data, table.Pointer)
		if !ok || base < 0xC0 || base%4 != 0 ||
			!gba.Within(data, base, table.Count*4) {
			entries = append(
				entries,
				ScriptEntry{
					Chapter:  WorldMapScripts,
					Pointer:  table.Pointer,
					Source:   table.Source,
					WorldMap: true,
					EventID:  -1,
					Err:      ErrWorldMapEvents,
				},
			)
			continue
		}
		for id := range table.Count {
			entry := scriptEntry(data, WorldMapScripts, base+id*4, table.Source)
			entry.WorldMap, entry.EventID = true, id
			entries = append(entries, entry)
		}
	}
	for _, ending := range f.WorldMapEvents.Endings {
		entry := scriptEntry(
			data,
			WorldMapScripts,
			ending.Pointer,
			ending.Source,
		)
		entry.EventID = -1
		entries = append(entries, entry)
	}
	return entries
}

// A chapter PLIST also contains unrelated graphics and maps. Only explicit
// chapter references establish script sites; even ID zero may contain a script.
func (f Format) chapterWorldMapEntries(
	data []byte,
	chapters *records.Table,
) []ScriptEntry {
	root := f.WorldMapEvents.ChapterPLIST
	if root == 0 {
		return nil
	}
	invalid := []ScriptEntry{
		{
			Chapter:  WorldMapScripts,
			Pointer:  root,
			Source:   "script_worldmap_intro",
			WorldMap: true,
			EventID:  -1,
			Err:      ErrWorldMapEvents,
		},
	}
	if chapters == nil || chapters.Err != nil {
		return invalid
	}
	base, ok := gba.Pointer(data, root)
	if !ok || base < 0xC0 || base%4 != 0 {
		return invalid
	}
	for _, field := range chapters.Fields {
		if field.ID != WorldMapEventField {
			continue
		}
		var entries []ScriptEntry
		seen := make(map[int]bool)
		for _, row := range chapters.Rows {
			id := int(field.Value(data, row.Address))
			if id < 0 || id > 255 {
				return invalid
			}
			if seen[id] {
				continue
			}
			seen[id] = true
			entry := scriptEntry(
				data,
				WorldMapScripts,
				base+id*4,
				"script_worldmap_intro",
			)
			entry.WorldMap, entry.EventID = true, id
			entries = append(entries, entry)
		}
		return entries
	}
	return invalid
}

func (s *Editor) WorldMapEvents() ([]WorldMapEvent, error) {
	format, err := s.scriptFormat()
	if err != nil {
		return nil, err
	}
	if !s.WorldMapEventsAvailable() {
		return nil, edit.ErrField
	}
	entries := format.worldMapEntries(
		s.image.Data,
		s.records.Table(records.Chapters),
	)
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
func (f Format) WorldMapClaims(
	data []byte,
	chapters *records.Table,
) []romwrite.Claim {
	var claims []romwrite.Claim
	if site := f.WorldMapEvents.ChapterPLIST; site >= 0xC0 &&
		gba.Within(data, site, 4) {
		claims = append(
			claims,
			romwrite.Claim{
				Start: site,
				End:   site + 4,
				Owner: romwrite.Owner{
					Resource: "event.worldmap_root",
					ID:       site,
				},
				Site: true,
			},
		)
	}
	for _, table := range f.WorldMapEvents.Tables {
		site := table.Pointer
		if site >= 0xC0 && gba.Within(data, site, 4) {
			owner := romwrite.Owner{Resource: "event.worldmap_root", ID: site}
			claims = append(
				claims,
				romwrite.Claim{
					Start: site,
					End:   site + 4,
					Owner: owner,
					Site:  true,
				},
			)
			base, ok := gba.Pointer(data, site)
			if ok && base >= 0xC0 &&
				(base%4 != 0 || !gba.Within(data, base, table.Count*4)) &&
				base < len(data) {
				claims = append(
					claims,
					romwrite.Claim{
						Start: base,
						End:   min(base+table.Count*4, len(data)),
						Owner: owner,
					},
				)
			}
		}
	}
	entries := f.worldMapEntries(data, chapters)
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
				romwrite.Claim{
					Start: entry.Address,
					End:   entry.Address + 4,
					Owner: ScriptCodeOwner(),
				},
			)
		}
		host := romwrite.Owner{}
		if entry.Source == "script_call_entry" {
			host = ScriptCodeOwner()
		}
		claims = append(
			claims,
			romwrite.Claim{
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
				romwrite.Claim{
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
