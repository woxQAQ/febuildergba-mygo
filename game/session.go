// Package game selects native versions and assembles editors for an open ROM.
package game

import (
	"github.com/woxQAQ/febuildergba-mygo/fault"
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
	"github.com/woxQAQ/febuildergba-mygo/rom"
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
	Terrain     *terrain.Editor
	format      format
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
		s := &Session{Document: doc, format: format}
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
				return references
			},
		)
		s.Records = records.New(context, format.recordFormat, s.Texts)
		chapterConfig := format.chapterConfig()
		chapterConfig.InventoryUses = s.inventoryUses
		s.Chapters = chapter.New(context, chapterConfig, s.Records)
		s.Events, s.Deployments, s.Maps = s.Chapters.Events, s.Chapters.Deployments, s.Chapters.Maps
		s.Shops = shop.NewEditor(context, format.shops, s.shopSource())
		s.Quotes = quote.New(context, format.quotes, s.quoteSource())
		s.Supports = support.New(context, format.supports, s.supportSource())
		s.Terrain = terrain.New(
			context,
			terrain.ForVersion(format.id),
			s.terrainSource(),
		)
		s.Portraits = portrait.New(
			context,
			format.portraits,
			format.genericPortraitFormat(),
			s.Records,
			format.chapterConfig().Ranges,
		)
		s.Sprites = sprite.New(context, format.id)
		s.Images = graphics.New(context, map[graphics.Kind]graphics.Backend{
			graphics.Portrait: format.portraits,
			graphics.ItemIcon: format.icons,
		}, format.imageReferences(s.Records))
		s.Refresh()
		return s, nil
	}
	return nil, edit.ErrVersion
}

func (s *Session) Version() string { return s.format.id }

func (s *Session) Refresh() { s.Records.Refresh() }
