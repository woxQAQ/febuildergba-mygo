package itemicon

import (
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/graphics"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

// Reader binds the native icon table to one document interpretation.
type Reader struct {
	context *edit.Context
	format  Format
	image   edit.Input
	table   Table
}

func New(context *edit.Context, format Format) *Reader {
	return &Reader{context: context, format: format}
}

// Table describes the declared icon window, including blank slots. FF is
// reserved; original tables use the version's inclusive maximum instead.
type Table struct {
	Count   int
	Err     error
	binding edit.Stamp
}

func (table Table) Current(reader *Reader) bool {
	return reader != nil && table.binding.Current(reader.context)
}

func (reader *Reader) Table() Table {
	if reader.table.Current(reader) {
		return reader.table
	}
	reader.image = reader.context.Capture()
	reader.table = Table{binding: reader.image.Stamp()}
	if err := reader.context.Validate(reader.image.Data); err != nil {
		reader.table.Err = err
		return reader.table
	}
	data := reader.image.Data
	base, ok := gba.Pointer(data, reader.format.Pointer)
	palette, paletteOK := gba.Pointer(data, reader.format.Palette)
	count := 0xFF
	if base == reader.format.Original {
		count = min(count, reader.format.Maximum+1)
	}
	if !ok || !paletteOK || base < 0xC0 || palette < 0xC0 || count < 1 ||
		!gba.Within(data, palette, 32) || !gba.Within(data, base, count*128) {
		reader.table.Err = graphics.ErrImage
		return reader.table
	}
	reader.table.Count = count
	return reader.table
}

func (reader *Reader) Preview(id int) (graphics.Preview, error) {
	table := reader.Table()
	if table.Err != nil {
		return graphics.Preview{}, table.Err
	}
	return reader.format.Preview(reader.image.Data, id)
}
