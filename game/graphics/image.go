package graphics

import (
	"image"

	"github.com/woxQAQ/febuildergba-mygo/fault"
)

type Kind string

const (
	Portrait Kind = "portrait"
	ItemIcon Kind = "item_icon"
)

type Ref struct {
	Kind Kind
	ID   int
}

type Region struct {
	Label           string
	Address, Length int
}

type Preview struct {
	Image   *image.RGBA
	Regions []Region
}

// Encoding includes the format's canonical pixels for prospective validation.
// Palette quantization belongs to the native encoder that chooses the palette.
type Encoding struct {
	Parts    []Replacement
	Expected image.Image
}

// Backend implements one native image format. PNG handling and document
// transactions do not need to know its table layout or resource composition.
type Backend interface {
	Preview([]byte, int) (Preview, error)
	Image([]byte, int) (*image.RGBA, error)
	Encode([]byte, int, image.Image) (Encoding, error)
}

var ErrImage = fault.Code{
	ID:   "game.invalid_image",
	Kind: fault.InvalidData,
}
