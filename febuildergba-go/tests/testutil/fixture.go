// Package testutil builds synthetic, non-bootable ROMs for domain tests.
package testutil

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
)

type FixtureBuilder struct {
	data      []byte
	positions map[string]int
}

// NewFixture writes the six-byte version identifier consumed by the adapters.
func NewFixture(gameID string, size int) *FixtureBuilder {
	f := &FixtureBuilder{data: make([]byte, max(size, 0x1000)), positions: make(map[string]int)}
	copy(f.data[0xA0:0xAC], "TEST FIXTURE")
	copy(f.data[0xAC:0xB2], gameID)
	return f
}

func (f *FixtureBuilder) WriteAt(offset int, data []byte) {
	if offset < 0 || offset > len(f.data) || len(data) > len(f.data)-offset {
		panic("fixture overflow")
	}
	copy(f.data[offset:], data)
}

func (f *FixtureBuilder) WriteU8(offset int, value uint8) { f.data[offset] = value }
func (f *FixtureBuilder) WriteU16(offset int, value uint16) {
	binary.LittleEndian.PutUint16(f.data[offset:], value)
}

func (f *FixtureBuilder) WriteU32(offset int, value uint32) {
	binary.LittleEndian.PutUint32(f.data[offset:], value)
}

func (f *FixtureBuilder) WritePointer(offset, target int) {
	f.WriteU32(offset, uint32(target)|0x08000000) // #nosec G115 -- fixture offsets fit in a GBA address.
}
func (f *FixtureBuilder) MarkPosition(name string, offset int) { f.positions[name] = offset }
func (f *FixtureBuilder) GetPosition(name string) int {
	pos, ok := f.positions[name]
	if !ok {
		panic("unknown fixture position: " + name)
	}
	return pos
}
func (f *FixtureBuilder) Bytes() []byte { return bytes.Clone(f.data) }
func (f *FixtureBuilder) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, f.data, 0o600)
}

// Addresses and strides mirror the pinned legacy ROMFE* and form initializers.
func NewMinimalFE8U() *FixtureBuilder {
	return gameFixture("BE8E01", 16<<20, 52, 84, 36, 0, [][]int{
		{0x10108, 0x104EC, 0x10538, 0xA0B40, 0xA0BB8, 0xBE954},
		{0x17AB8, 0x17DBC, 0x18DAC, 0x19458},
		{0x177C0, 0x16570, 0x166D0, 0x1671C, 0x167E0, 0x16A10, 0x16B18, 0x16BB4},
	})
}

func NewMinimalFE8J() *FixtureBuilder {
	return gameFixture("BE8J01", 16<<20, 52, 84, 36, 0, [][]int{
		{0x10650, 0x10268, 0x1069C, 0xA3B64, 0xA3CE0, 0xA4928, 0xA49A0, 0xA4C3C, 0xA5458, 0xA56A8, 0xA5860, 0xA595C, 0xA5BB0, 0xA5D28, 0xA61F8},
		{0x17860, 0x17ADC, 0x18AC0, 0x19130},
		{0x17568, 0x17554, 0x1753C, 0x17510, 0x172BC},
	})
}

func NewMinimalFE7U() *FixtureBuilder {
	return gameFixture("AE7E01", 16<<20, 52, 84, 36, 0, [][]int{
		{0x9A274}, {0x178F0}, {0x174D4},
	})
}

func NewMinimalFE7J() *FixtureBuilder {
	return gameFixture("AE7J01", 16<<20, 52, 84, 36, 0, [][]int{
		{0x9AC50}, {0x17CE0}, {0x164DC},
	})
}

func NewMinimalFE6() *FixtureBuilder {
	return gameFixture("AFEJ01", 8<<20, 48, 72, 32, 48, [][]int{
		{0x17680}, {0x176E0}, {0x16410},
	})
}

func gameFixture(id string, size, unitStride, classStride, itemStride, skip int, pointers [][]int) *FixtureBuilder {
	f := NewFixture(id, size)
	const unitBase, classBase, itemBase = 0x200000, 0x204000, 0x208000
	for i, base := range []int{unitBase, classBase, itemBase} {
		for _, address := range pointers[i] {
			f.WritePointer(address, base)
		}
	}
	for name, offset := range map[string]int{
		"units_base": unitBase, "classes_base": classBase, "items_base": itemBase,
		"units": unitBase + skip, "classes": classBase, "items": itemBase,
		"units_pointer": pointers[0][0], "classes_pointer": pointers[1][0], "items_pointer": pointers[2][0],
	} {
		f.MarkPosition(name, offset)
	}
	if skip != 0 {
		f.WriteU8(unitBase, 0xA5) // Distinguishes the hidden FE6 record from zero padding.
	}
	for i := range 2 {
		unit := unitBase + skip + i*unitStride
		f.WriteU8(unit+4, byte(i+1))
		f.WriteU8(unit+11, 1)
		f.WriteU8(unit+12, byte(16+i))
		f.WriteU8(unit+28, 75)
		class := classBase + i*classStride
		f.WriteU8(class+4, byte(i))
		f.WriteU8(class+11, byte(20+i))
		item := itemBase + i*itemStride
		f.WriteU8(item+6, byte(i))
		f.WriteU8(item+20, 46)
		f.WriteU8(item+21, byte(5+i))
	}
	f.WritePointer(unitBase+skip+0x2C, 0x300000)
	if classStride == 84 {
		f.WritePointer(classBase+0x88, 0x300000)
	}
	// Class ID 0 ends the class table after its first record; invalid pointers end items.
	f.WriteU32(itemBase+2*itemStride+12, 0xFFFFFFFF)
	return f
}
