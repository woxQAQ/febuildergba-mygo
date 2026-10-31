package testutil

import "bytes"

// NewTerrainFixture gives class 1 shared, independent, empty and damaged
// terrain entries. A partially overlapping class 0 table and opaque suffixes
// distinguish copying a native table from overwriting its neighbors.
func NewTerrainFixture(version string) *FixtureBuilder {
	var f *FixtureBuilder
	count, stride, normal, avoid := 65, 84, 56, 68
	switch version {
	case "FE6":
		f = NewMinimalFE6()
		count, stride, normal, avoid = 51, 72, 52, 56
	case "FE7U":
		f = NewMinimalFE7U()
	case "FE7J":
		f = NewMinimalFE7J()
	case "FE8U":
		f = NewMinimalFE8U()
	case "FE8J":
		f = NewMinimalFE8J()
	default:
		panic("unsupported fixture version")
	}
	base := f.GetPosition("classes")
	f.WriteU8(base+2*stride+4, 2)
	f.MarkPosition("terrain_class", base+stride)
	f.MarkPosition("terrain_normal_pointer", base+stride+normal)
	f.MarkPosition("terrain_avoid_pointer", base+stride+avoid)
	const movement, bonus = 0x320001, 0x321001
	f.MarkPosition("terrain_movement", movement)
	f.MarkPosition("terrain_bonus", bonus)
	raw := bytes.Repeat([]byte{7}, count+4)
	raw[0], raw[1], raw[2], raw[3], raw[4], raw[12] = 0xFF, 1, 0, 0x80, 0xFE, 2
	copy(raw[count:], []byte{0xA1, 0xB2, 0xC3, 0xD4})
	f.WriteAt(movement, raw)
	raw = bytes.Repeat([]byte{0}, count+4)
	raw[0], raw[12], raw[31] = 0xFE, 20, 30
	copy(raw[count:], []byte{0xE1, 0xF2, 0xA3, 0xB4})
	f.WriteAt(bonus, raw)
	f.WritePointer(base+normal, movement+1)
	for _, id := range []int{1, 2} {
		class := base + id*stride
		f.WritePointer(class+normal, movement)
		f.WritePointer(class+avoid, bonus)
		f.WritePointer(class+avoid+4, bonus)
		f.WritePointer(class+avoid+8, bonus)
		if version != "FE6" {
			f.WritePointer(class+normal+4, movement)
			f.WritePointer(class+normal+8, movement)
		}
	}
	// Other columns remain editable when these references are unavailable.
	f.WriteU32(base+stride+avoid+8, 0x03000000)
	if version != "FE6" {
		f.WriteU32(base+stride+normal+8, 0)
	}
	return f
}
