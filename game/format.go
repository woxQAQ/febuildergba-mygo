package game

import (
	"slices"

	"github.com/woxQAQ/febuildergba-mygo/game/chapter"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/itemicon"
	"github.com/woxQAQ/febuildergba-mygo/game/maps"
	"github.com/woxQAQ/febuildergba-mygo/game/portrait"
	"github.com/woxQAQ/febuildergba-mygo/game/quote"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/shop"
	"github.com/woxQAQ/febuildergba-mygo/game/sprite"
	"github.com/woxQAQ/febuildergba-mygo/game/support"
	"github.com/woxQAQ/febuildergba-mygo/game/terrain"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/game/worldmap"
)

// format groups the locations and encodings consumed by the shared editors.
type format struct {
	id, header string
	minimum    int
	tables     []records.Rule
	chapters   chapter.Format
	events     event.Format
	maps       maps.Format
	text       text.Format
	portraits  portrait.Format
	// genericPortraits locates the version's generic mini-portrait table.
	genericPortraits portrait.GenericFormat
	icons            itemicon.Format
	supports         support.Format
	quotes           quote.Format
	terrain          terrain.Format
	sprites          sprite.Format
	// classAnimationOffset is the class record's battle-animation pointer
	// position, 48 in FE6 and 52 in the later generations.
	classAnimationOffset int
	traits               []records.RecordTrait
	weaponThresholds     []int64
	shops                shop.Format
	worldmap             worldmap.Format
	features             func([]byte) (map[byte]string, []string)
}

func (g format) Match(data []byte) bool {
	return len(data) >= g.minimum && string(data[0xAC:0xB2]) == g.header
}

func (g format) recordFormat(data []byte) records.Format {
	var aliases map[byte]string
	var patches []string
	if g.features != nil {
		aliases, patches = g.features(data)
	}
	return records.Format{
		TextAliases:          aliases,
		Rules:                g.tableRules(data),
		Patches:              patches,
		Traits:               g.traits,
		WeaponThresholds:     g.weaponThresholds,
		Terrain:              g.terrain,
		ClassAnimationOffset: g.classAnimationOffset,
		NameOf:               func(id uint16) (string, error) { return g.text.ReadName(data, id, aliases) },
	}
}

func (g format) chapterConfig() chapter.Config {
	return chapter.Config{
		Table: g.chapters, Events: g.events, Maps: g.maps, Text: g.text,
		Shops: g.shops,
	}
}

func (g format) tableRules(data []byte) []records.Rule {
	rules := slices.Clone(g.tables)
	textTable, _ := g.text.Table(data)
	return append(rules, g.chapters.Rule(data, textTable.Count))
}
