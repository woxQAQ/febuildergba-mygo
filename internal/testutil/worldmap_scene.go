package testutil

import "testing"

// NewWorldMapSceneFixture combines independent portrait resources with a shared
// narration scene, split text IDs, signed delays and unused configuration bits.
func NewWorldMapSceneFixture(t *testing.T, version string) *FixtureBuilder {
	t.Helper()
	f := NewWorldMapEventFixture(version)
	index := 0
	if version == "FE8J" {
		index = 1
	}
	images := ImageFixture(t, index).Bytes()
	f.WriteAt(ImageTable, images[ImageTable:ImageIcons])
	f.WritePointer(ImageLayouts[index].PortraitPointer, ImageTable)
	f.WriteU16(f.GetPosition("units")+6, 1)
	f.WriteU16(f.GetPosition("units")+52+6, 2)
	// ID 3 is readable here, so 1 | 2 is a valid compound narration reference.
	f.WriteU32(f.GetPosition("talk_text_table")+12, 0x88313000)
	const scene = 0x28E000
	for _, site := range []int{f.GetPosition("wm_site"), f.GetPosition("wm_alias_site"), f.GetPosition("wm_alias"), f.GetPosition("wm_intro") + 7*4} {
		f.WritePointer(site, scene)
	}
	f.WriteAt(scene, []byte{
		0x20, 0xC0, 0x11, 0x22,
		0x40, 0xAD, 0x33, 0x44, 0xFE, 0xFF, 0xFF, 0xFF,
		0x20, 0xAE, 0x55, 0x66,
		0x40, 0xAF, 0x77, 0x88, 60, 0, 0xFF, 0xFF,
		0x20, 0xB1, 0x99, 0xAA,
		0x40, 0xB2, 0xBB, 0xCC, 2, 0xAA, 0xBB, 0xCC,
		0x20, 0xB3, 0xDD, 0xEE,
		0x20, 0xC7, 0x12, 0x34,
		0x60, 0xC4, 0x56, 0x78, 0, 0, 1, 0, 52, 0xA5, 0xFE, 0xFF,
		0x40, 0xC6, 0x9A, 0xBC, 1, 0, 2, 0,
		0x20, 0x1D, 0, 0,
		0x40, 0xC5, 1, 0, 0x5B, 0xD1, 0xFF, 0xFF,
		0x20, 0xC9, 0xDE, 0xF0,
		0x20, 0xC1, 0x11, 0x22,
		0x20, 0xC2, 0x33, 0x44,
		0x20, 0x81, 0x55, 0x66,
		0x20, 0x82, 0x77, 0x88,
		0x20, 0x0E, 1, 0,
		0x20, 1, 0, 0,
	})
	f.MarkPosition("wm_scene", scene)
	f.MarkPosition("wm_portrait_root", ImageLayouts[index].PortraitPointer)
	return f
}
