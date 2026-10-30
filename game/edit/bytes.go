package edit

import (
	"bytes"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/rom"
)

// RawRegion records the document and revision whose bytes were shown for
// editing.
// Its original bytes cannot be changed through the UI's draft buffer.
type Region struct {
	binding  Stamp
	address  int
	original []byte
}

func (r *Region) Address() int { return r.address }

func (r *Region) Revision() uint64 { return r.binding.Revision() }

func (r *Region) Bytes() []byte { return bytes.Clone(r.original) }

func (s *Bytes) Read(
	address, length int,
) (result *Region, err error) {
	defer func() {
		err = fault.Wrap(
			err,
			"game.read_region",
			fault.Context{
				Range: &fault.Range{
					Offset: address,
					Length: length,
					Size:   len(s.image.Data),
				},
			},
		)
	}()
	s.Refresh()
	if length < 1 || !gba.Within(s.image.Data, address, length) {
		return nil, rom.ErrRange
	}
	return &Region{
		binding:  s.context.BindBytes(s.image.Revision),
		address:  address,
		original: bytes.Clone(s.image.Data[address : address+length]),
	}, nil
}

// ApplyRegion is a fixed-length replacement. A stale draft must be read again
// explicitly.
func (s *Bytes) Apply(
	region *Region,
	replacement []byte,
) (err error) {
	ctx := fault.Context{}
	if region != nil {
		ctx.Range = &fault.Range{
			Offset: region.address,
			Length: len(replacement),
			Size:   len(region.original),
		}
	}
	defer func() { err = fault.Wrap(err, "game.apply_region", ctx) }()
	if region == nil || !region.binding.SameDocument(s.context) {
		return rom.ErrStale
	}
	if len(replacement) != len(region.original) {
		return rom.ErrRange
	}
	if err := s.context.Document().Apply(
		region.binding.Revision(),
		"raw",
		[]rom.Change{{Offset: region.address, Data: replacement}},
	); err != nil {
		return err
	}
	s.Refresh()
	return nil
}

// Bytes provides bounded raw access without depending on game interpretation.
type Bytes struct {
	context *Context
	image   rom.Snapshot
}

func NewBytes(context *Context) *Bytes { return &Bytes{context: context} }

func (s *Bytes) Refresh() { s.image = s.context.Document().Snapshot() }

func (s *Bytes) Slice(address, length int) ([]byte, error) {
	s.Refresh()
	if !gba.Within(s.image.Data, address, length) {
		return nil, fault.Wrap(
			rom.ErrRange,
			"game.raw",
			fault.Context{
				Range: &fault.Range{
					Offset: address,
					Length: length,
					Size:   len(s.image.Data),
				},
			},
		)
	}
	return bytes.Clone(s.image.Data[address : address+length]), nil
}
