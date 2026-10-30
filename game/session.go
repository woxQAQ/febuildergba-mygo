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
	"github.com/woxQAQ/febuildergba-mygo/game/records"
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
	Images      *graphics.Editor
	Chapters    *chapter.Editor
	Events      *event.Editor
	Deployments *deployment.Editor
	Maps        *maps.Editor
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
			func(data []byte, id uint16) text.References { return s.Records.TextReferences(data, id) },
		)
		s.Records = records.New(context, format.recordFormat, s.Texts)
		s.Chapters = chapter.New(context, format.chapterConfig(), s.Records)
		s.Events, s.Deployments, s.Maps = s.Chapters.Events, s.Chapters.Deployments, s.Chapters.Maps
		s.Portraits = portrait.New(
			context,
			format.portraits,
			format.genericPortraitFormat(),
			s.Records,
			format.chapterConfig().Ranges,
		)
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
