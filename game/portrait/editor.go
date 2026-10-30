// Package portrait decodes and edits native portrait resources.
package portrait

import (
	"image"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

// Editor owns portrait discovery, decoded frames and metadata edits. Record
// references remain inputs to portrait-table discovery, including damaged rows.
type Editor struct {
	context         *edit.Context
	format          Format
	generic         GenericFormat
	records         *records.Editor
	protected       func([]byte) []space.Range
	image           rom.Snapshot
	binding         edit.Stamp
	portraitEntries []Entry
}

func New(
	context *edit.Context,
	format Format,
	generic GenericFormat,
	records *records.Editor,
	protected func([]byte) []space.Range,
) *Editor {
	return &Editor{
		context:   context,
		format:    format,
		generic:   generic,
		records:   records,
		protected: protected,
	}
}

func (s *Editor) Refresh() {
	if !s.binding.Current(s.context) {
		s.image = s.context.Document().Snapshot()
		s.binding = s.context.Bind(s.image.Revision)
		s.portraitEntries = nil
	}
}

func (f Format) Image(data []byte, id int) (*image.RGBA, error) {
	sheet, err := f.sheet(data, id)
	return sheet.image, err
}

func (f Format) Encode(
	data []byte,
	id int,
	bitmap image.Image,
) (graphics.Encoding, error) {
	sheet, err := f.sheet(data, id)
	if err != nil {
		return graphics.Encoding{}, err
	}
	parts, err := sheet.encode(bitmap)
	return graphics.Encoding{
		Parts:    parts,
		Expected: gba.Quantise4BPP(bitmap),
	}, err
}
