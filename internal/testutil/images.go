package testutil

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/gba"
)

// ImageLayout describes the fixed native locations used by the image fixtures.
// Inputs follow ROMFE* and the portrait/item-icon forms in the pinned baseline;
// they are independent of the production format configuration.
type ImageLayout struct {
	PortraitPointer, PortraitStride, CardOffset int
	IconPointer, IconPalette                    int
	GenericPointer, GenericCount                int
}

// ImageLayouts follows the FE8U, FE8J, FE7U, FE7J, FE6 fixture order.
var ImageLayouts = [...]ImageLayout{
	{0x5524, 28, 16, 0x36B4, 0x35D0, 0x5F94, 8},
	{0x542C, 28, 16, 0x36D4, 0x351C, 0x5E98, 8},
	{0x6B30, 28, 16, 0x4E20, 0x4D40, 0x75C0, 6},
	{0x69C0, 28, 16, 0x4CFC, 0x4C1C, 0x7450, 6},
	{0x7FD8, 16, 0, 0x4CEC, 0x4AC4, 0x8DF0, 7},
}

const ImageTable, ImageFace, ImageMini, ImageMouth, ImagePalette, ImageCard, ImageIcons = 0x340000, 0x341000, 0x344000, 0x345000, 0x346000, 0x347000, 0x348000

func ImageFixture(t *testing.T, index int) *FixtureBuilder {
	t.Helper()
	f := Games()[index].Build()
	format := ImageLayouts[index]
	f.WritePointer(format.PortraitPointer, ImageTable)
	f.WritePointer(format.IconPointer, ImageIcons)
	f.WritePointer(format.IconPalette, ImagePalette)
	f.WriteAt(ImagePalette, []byte{0, 0, 0x1F, 0, 0xE0, 3, 0, 0x7C})
	f.WriteAt(ImageIcons, bytes.Repeat([]byte{0x11}, 128))
	f.WriteAt(ImageIcons+128, bytes.Repeat([]byte{0x22}, 128))
	length := 4096
	if format.PortraitStride == 16 {
		length = 4352
	}
	raw := bytes.Repeat([]byte{0x11}, length)
	if index == 2 || index == 4 {
		compressed, err := gba.CompressLZ77(raw)
		if err != nil {
			t.Fatal(err)
		}
		// A two-literal prefix and distance-two run are valid for BIOS VRAM
		// decoding.
		if !bytes.Equal(compressed[4:9], []byte{0x3F, 0x11, 0x11, 0xF0, 1}) {
			t.Fatalf("noncanonical VRAM packet prefix: % X", compressed[4:9])
		}
		f.WriteAt(ImageFace, compressed)
	} else {
		f.WriteAt(ImageFace, append([]byte{0, 4, 0x10, 0}, raw...))
	}
	mini := bytes.Repeat([]byte{0x22}, 512)
	if index != 4 {
		var err error
		mini, err = gba.CompressLZ77(mini)
		if err != nil {
			t.Fatal(err)
		}
	}
	f.WriteAt(ImageMini, mini)
	f.WriteAt(ImageMouth, bytes.Repeat([]byte{0x33}, 1536))
	card, err := gba.CompressLZ77(bytes.Repeat([]byte{0x22}, 3200))
	if err != nil {
		t.Fatal(err)
	}
	f.WriteAt(ImageCard, card)
	for _, id := range []int{1, 2} {
		at := ImageTable + id*format.PortraitStride
		f.WritePointer(at, ImageFace)
		f.WritePointer(at+4, ImageMini)
		f.WritePointer(at+8, ImagePalette)
		if index != 4 {
			f.WritePointer(at+12, ImageMouth)
		}
	}
	cardAt := ImageTable + 3*format.PortraitStride
	f.WritePointer(cardAt+format.CardOffset, ImageCard)
	f.WritePointer(cardAt+8, ImagePalette)
	unitStride, classStride := 52, 84
	if index == 4 {
		unitStride, classStride = 48, 72
	}
	f.WriteU16(f.GetPosition("units")+6, 1)
	f.WriteU16(f.GetPosition("units")+unitStride+6, 1)
	f.WriteU16(f.GetPosition("classes")+8, 1)
	f.WriteU16(f.GetPosition("classes")+classStride+8, 2)
	return f
}

func ImagePNG(t *testing.T, bitmap image.Image) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := png.Encode(&out, bitmap); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
