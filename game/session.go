// Package game selects native versions and assembles editors for an open ROM.
package game

import (
	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/arena"
	"github.com/woxQAQ/febuildergba-mygo/game/chapter"
	"github.com/woxQAQ/febuildergba-mygo/game/deployment"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/event"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
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
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// Session assembles the editors for one document and the detected game version.
// Each editor owns its interpretation and cache; all edits share document history.
type Session struct {
	Document    *rom.Document
	Bytes       *edit.Bytes
	Records     *records.Editor
	Texts       *text.Editor
	Portraits   *portrait.Editor
	Sprites     *sprite.Reader
	Images      *graphics.Editor
	Chapters    *chapter.Editor
	Events      *event.Editor
	Deployments *deployment.Editor
	Maps        *maps.Editor
	Shops       *shop.Editor
	Quotes      *quote.Editor
	Supports    *support.Editor
	Arena       *arena.Editor
	Terrain     *terrain.Editor
	WorldMap    *worldmap.Editor
	format      format
	context     *edit.Context
	claimsStamp edit.Stamp
	claimsCache []space.Claim
}

func Open(path string) (session *Session, err error) {
	defer func() { err = fault.Wrap(err, "game.open", fault.Context{Path: path}) }()
	doc, err := rom.Open(path)
	if err != nil {
		return nil, err
	}
	image := doc.Snapshot()
	for _, format := range []format{fe8uFormat(), fe8jFormat(), fe7uFormat(), fe7jFormat(), fe6Format()} {
		if !format.Match(image.Data) {
			continue
		}
		context := edit.New(doc, format.Match)
		s := &Session{Document: doc, format: format, context: context}
		s.Bytes = edit.NewBytes(context)
		s.Texts = text.New(
			context,
			format.text,
			func(data []byte, id uint16) text.References {
				references := s.Records.TextReferences(data, id)
				references.Uses = append(
					references.Uses,
					format.quotes.TextUses(data, id)...)
				references.Uses = append(
					references.Uses,
					format.supports.TextUses(data, id)...)
				references.Uses = append(
					references.Uses,
					format.worldmap.TextUses(data, id)...)
				// A text entry lives inside the text table, so the table's own
				// claim cannot guard its entries against other domains.
				var external []space.Range
				for _, claim := range s.claims(data) {
					if claim.Owner.Resource != "text" {
						external = append(external, claim.Range)
					}
				}
				references.Ranges = external
				return references
			},
		)
		s.Records = records.New(
			context,
			func(data []byte) records.Format {
				resolved := format.recordFormat(data)
				resolved.Protected = func(data []byte) []space.Range {
					return space.ClaimRanges(s.claims(data))
				}
				return resolved
			},
			s.Texts,
		)
		s.Portraits = portrait.New(
			context,
			format.portraits,
			format.genericPortraits,
			s.Records,
			func(data []byte) []space.Range {
				return space.ClaimRanges(s.claims(data))
			},
		)
		s.WorldMap = worldmap.New(context, format.worldmap, s.worldmapSource())
		chapterConfig := format.chapterConfig()
		chapterConfig.WorldMap = s.WorldMap
		chapterConfig.Portraits = s.Portraits
		chapterConfig.InventoryUses = s.inventoryUses
		chapterConfig.ProtectPointer = s.protectPointer
		chapterConfig.ProtectRange = s.protectRange
		s.Chapters = chapter.New(context, chapterConfig, s.Records)
		s.Events, s.Deployments, s.Maps = s.Chapters.Events, s.Chapters.Deployments, s.Chapters.Maps
		s.Shops = shop.New(context, format.shops, s.shopSource())
		s.Quotes = quote.New(context, format.quotes, s.quoteSource())
		s.Supports = support.New(context, format.supports, s.supportSource())
		s.Arena = arena.New(context, format.arena, s.arenaSource())
		s.Terrain = terrain.New(
			context,
			format.terrain,
			s.terrainSource(),
		)
		s.Sprites = sprite.New(context, format.sprites)
		s.Images = graphics.New(context, map[graphics.Kind]graphics.Backend{
			graphics.Portrait: format.portraits,
			graphics.ItemIcon: format.icons,
		}, s.imageReferences)
		s.Refresh()
		return s, nil
	}
	return nil, edit.ErrVersion
}

func (s *Session) Version() string { return s.format.id }

func (s *Session) Refresh() { s.Records.Refresh() }
