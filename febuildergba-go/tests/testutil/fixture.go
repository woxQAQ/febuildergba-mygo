// Package testutil provides test infrastructure and fixture generation.
package testutil

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
)

// FixtureBuilder constructs minimal test ROMs.
type FixtureBuilder struct {
	data      []byte
	size      int
	pointers  map[int]int // offset -> target
	positions map[string]int
}

// NewFixture creates a builder with standard GBA header.
func NewFixture(gameID string, size int) *FixtureBuilder {
	if size < 0x1000 {
		size = 0x1000 // minimum reasonable size
	}
	f := &FixtureBuilder{
		data:      make([]byte, size),
		size:      size,
		pointers:  make(map[int]int),
		positions: make(map[string]int),
	}

	// Write GBA ROM header (0x00-0xA0)
	copy(f.data[0x104:0x108], []byte{0x24, 0xFF, 0xAE, 0x51}) // Nintendo logo signature
	copy(f.data[0xA0:0xAC], []byte(gameID))

	return f
}

// WriteAt writes data at offset.
func (f *FixtureBuilder) WriteAt(offset int, data []byte) {
	if offset+len(data) > f.size {
		panic("fixture overflow")
	}
	copy(f.data[offset:], data)
}

// WriteU8 writes a single byte.
func (f *FixtureBuilder) WriteU8(offset int, value uint8) {
	f.data[offset] = value
}

// WriteU16 writes a little-endian uint16.
func (f *FixtureBuilder) WriteU16(offset int, value uint16) {
	binary.LittleEndian.PutUint16(f.data[offset:], value)
}

// WriteU32 writes a little-endian uint32.
func (f *FixtureBuilder) WriteU32(offset int, value uint32) {
	binary.LittleEndian.PutUint32(f.data[offset:], value)
}

// WritePointer writes a GBA pointer (address | 0x08000000).
func (f *FixtureBuilder) WritePointer(offset int, target int) {
	f.pointers[offset] = target
	f.WriteU32(offset, uint32(target|0x08000000))
}

// MarkPosition records a named position for later reference.
func (f *FixtureBuilder) MarkPosition(name string, offset int) {
	f.positions[name] = offset
}

// GetPosition retrieves a previously marked position.
func (f *FixtureBuilder) GetPosition(name string) int {
	pos, ok := f.positions[name]
	if !ok {
		panic("unknown position: " + name)
	}
	return pos
}

// Bytes returns the complete fixture data.
func (f *FixtureBuilder) Bytes() []byte {
	return bytes.Clone(f.data)
}

// Save writes the fixture to a file.
func (f *FixtureBuilder) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, f.data, 0644)
}

// FE8U-specific builders

// UnitData represents a FE8U unit record.
type UnitData struct {
	NameID      uint16
	DescID      uint16
	CharID      uint8
	Class       uint8
	Portrait    uint8
	Level       uint8
	HP, Pow     uint8
	Skl, Spd    uint8
	Def, Res    uint8
	Lck         uint8
	SwordRank   uint8
	LanceRank   uint8
	AxeRank     uint8
	BowRank     uint8
	StaffRank   uint8
	AnimaRank   uint8
	LightRank   uint8
	DarkRank    uint8
	HPGrowth    uint8
	PowGrowth   uint8
	SklGrowth   uint8
	SpdGrowth   uint8
	DefGrowth   uint8
	ResGrowth   uint8
	LckGrowth   uint8
}

// WriteFE8UUnit writes a unit record at offset.
func (f *FixtureBuilder) WriteFE8UUnit(offset int, u UnitData) {
	f.WriteU16(offset+0x00, u.NameID)
	f.WriteU16(offset+0x02, u.DescID)
	f.WriteU8(offset+0x04, u.CharID)
	f.WriteU8(offset+0x05, u.Class)
	f.WriteU8(offset+0x06, u.Portrait)
	f.WriteU8(offset+0x08, u.Level)
	f.WriteU8(offset+0x09, u.HP)
	f.WriteU8(offset+0x0A, u.Pow)
	f.WriteU8(offset+0x0B, u.Skl)
	f.WriteU8(offset+0x0C, u.Spd)
	f.WriteU8(offset+0x0D, u.Def)
	f.WriteU8(offset+0x0E, u.Res)
	f.WriteU8(offset+0x0F, u.Lck)
	f.WriteU8(offset+0x10, u.SwordRank)
	f.WriteU8(offset+0x11, u.LanceRank)
	f.WriteU8(offset+0x12, u.AxeRank)
	f.WriteU8(offset+0x13, u.BowRank)
	f.WriteU8(offset+0x14, u.StaffRank)
	f.WriteU8(offset+0x15, u.AnimaRank)
	f.WriteU8(offset+0x16, u.LightRank)
	f.WriteU8(offset+0x17, u.DarkRank)
	f.WriteU8(offset+0x18, u.HPGrowth)
	f.WriteU8(offset+0x19, u.PowGrowth)
	f.WriteU8(offset+0x1A, u.SklGrowth)
	f.WriteU8(offset+0x1B, u.SpdGrowth)
	f.WriteU8(offset+0x1C, u.DefGrowth)
	f.WriteU8(offset+0x1D, u.ResGrowth)
	f.WriteU8(offset+0x1E, u.LckGrowth)
	// Rest filled with zeros
}

// ClassData represents a FE8U class record.
type ClassData struct {
	NameID      uint16
	DescID      uint16
	ClassID     uint8
	HP, Pow     uint8
	Skl, Spd    uint8
	Def, Res    uint8
	Lck         uint8
	HPGrowth    uint8
	PowGrowth   uint8
	SklGrowth   uint8
	SpdGrowth   uint8
	DefGrowth   uint8
	ResGrowth   uint8
	LckGrowth   uint8
	HPMax       uint8
	PowMax      uint8
	SklMax      uint8
	SpdMax      uint8
	DefMax      uint8
	ResMax      uint8
	LckMax      uint8
}

// WriteFE8UClass writes a class record at offset.
func (f *FixtureBuilder) WriteFE8UClass(offset int, c ClassData) {
	f.WriteU16(offset+0x00, c.NameID)
	f.WriteU16(offset+0x02, c.DescID)
	f.WriteU8(offset+0x04, c.ClassID)
	f.WriteU8(offset+0x05, c.HP)
	f.WriteU8(offset+0x06, c.Pow)
	f.WriteU8(offset+0x07, c.Skl)
	f.WriteU8(offset+0x08, c.Spd)
	f.WriteU8(offset+0x09, c.Def)
	f.WriteU8(offset+0x0A, c.Res)
	f.WriteU8(offset+0x0B, c.Lck)
	f.WriteU8(offset+0x1D, c.HPGrowth)
	f.WriteU8(offset+0x1E, c.PowGrowth)
	f.WriteU8(offset+0x1F, c.SklGrowth)
	f.WriteU8(offset+0x20, c.SpdGrowth)
	f.WriteU8(offset+0x21, c.DefGrowth)
	f.WriteU8(offset+0x22, c.ResGrowth)
	f.WriteU8(offset+0x23, c.LckGrowth)
	f.WriteU8(offset+0x24, c.HPMax)
	f.WriteU8(offset+0x25, c.PowMax)
	f.WriteU8(offset+0x26, c.SklMax)
	f.WriteU8(offset+0x27, c.SpdMax)
	f.WriteU8(offset+0x28, c.DefMax)
	f.WriteU8(offset+0x29, c.ResMax)
	f.WriteU8(offset+0x2A, c.LckMax)
}

// ItemData represents a FE8U item record.
type ItemData struct {
	NameID      uint16
	DescID      uint16
	ItemID      uint8
	Type        uint8
	Attributes  uint16
	StatBonuses [7]int8
	Effectiveness uint8
	Durability  uint8
	Power       uint8
	Hit         uint8
	Weight      uint8
	Crit        uint8
	Range       uint8
	CostPerUse  uint16
	WRankBonus  uint8
}

// WriteFE8UItem writes an item record at offset.
func (f *FixtureBuilder) WriteFE8UItem(offset int, i ItemData) {
	f.WriteU16(offset+0x00, i.NameID)
	f.WriteU16(offset+0x02, i.DescID)
	f.WriteU8(offset+0x04, i.ItemID)
	f.WriteU8(offset+0x05, i.Type)
	f.WriteU16(offset+0x06, i.Attributes)
	for j, bonus := range i.StatBonuses {
		f.WriteU8(offset+0x08+j, uint8(bonus))
	}
	f.WriteU8(offset+0x0F, i.Effectiveness)
	f.WriteU8(offset+0x10, i.Durability)
	f.WriteU8(offset+0x11, i.Power)
	f.WriteU8(offset+0x12, i.Hit)
	f.WriteU8(offset+0x13, i.Weight)
	f.WriteU8(offset+0x14, i.Crit)
	f.WriteU8(offset+0x15, i.Range)
	f.WriteU16(offset+0x1C, i.CostPerUse)
	f.WriteU8(offset+0x20, i.WRankBonus)
}

// NewMinimalFE8U creates a minimal FE8U fixture with standard tables.
func NewMinimalFE8U() *FixtureBuilder {
	f := NewFixture("FIREEMBLEM8\x0A\x45\x00\x00", 0x900000)

	// FE8U pointer table locations
	const (
		ptrUnits   = 0x803D90
		ptrClasses = 0x803EB8
		ptrItems   = 0x809B10
	)

	// Data locations
	const (
		dataUnits   = 0x800000
		dataClasses = 0x800200
		dataItems   = 0x800400
	)

	// Write pointer table
	f.WritePointer(ptrUnits, dataUnits)
	f.WritePointer(ptrClasses, dataClasses)
	f.WritePointer(ptrItems, dataItems)

	f.MarkPosition("units", dataUnits)
	f.MarkPosition("classes", dataClasses)
	f.MarkPosition("items", dataItems)

	// Write one unit (Eirika)
	f.WriteFE8UUnit(dataUnits, UnitData{
		NameID:    0x0002,
		CharID:    0x03,
		Class:     0x07,
		Level:     1,
		HP:        16,
		Pow:       4,
		Skl:       9,
		Spd:       12,
		Def:       3,
		Res:       2,
		Lck:       8,
		SwordRank: 11,
		HPGrowth:  75,
		PowGrowth: 40,
		SklGrowth: 60,
		SpdGrowth: 60,
		DefGrowth: 30,
		ResGrowth: 30,
		LckGrowth: 60,
	})
	// Terminator
	for i := 0; i < 0x88; i++ {
		f.data[dataUnits+0x88+i] = 0
	}

	// Write one class (Eirika Lord)
	f.WriteFE8UClass(dataClasses, ClassData{
		NameID:    0x00F5,
		ClassID:   0x07,
		HP:        16,
		Pow:       4,
		Skl:       9,
		Spd:       9,
		Def:       3,
		Res:       1,
		Lck:       5,
		HPGrowth:  75,
		PowGrowth: 40,
		SklGrowth: 60,
		SpdGrowth: 60,
		DefGrowth: 30,
		ResGrowth: 30,
		LckGrowth: 60,
		HPMax:     60,
		PowMax:    20,
		SklMax:    20,
		SpdMax:    20,
		DefMax:    20,
		ResMax:    20,
		LckMax:    30,
	})
	// Terminator
	for i := 0; i < 0x54; i++ {
		f.data[dataClasses+0x54+i] = 0
	}

	// Write one item (Iron Sword)
	f.WriteFE8UItem(dataItems, ItemData{
		NameID:     0x01E9,
		ItemID:     0x01,
		Type:       0x00, // Sword
		Durability: 46,
		Power:      5,
		Hit:        90,
		Weight:     5,
		Crit:       0,
		Range:      0x11, // 1-1
	})
	// Terminator
	for i := 0; i < 0x24; i++ {
		f.data[dataItems+0x24+i] = 0
	}

	return f
}

// NewMinimalFE6 creates a minimal FE6 fixture.
func NewMinimalFE6() *FixtureBuilder {
	f := NewFixture("FIREEMBLEM6\x00", 0x900000)

	// FE6 pointer locations (different from FE8U)
	const (
		ptrUnits   = 0x017FC8
		ptrClasses = 0x017FD0
		ptrItems   = 0x017FE8
	)

	const (
		dataUnits   = 0x800000
		dataClasses = 0x800200
		dataItems   = 0x800400
	)

	f.WritePointer(ptrUnits, dataUnits)
	f.WritePointer(ptrClasses, dataClasses)
	f.WritePointer(ptrItems, dataItems)

	f.MarkPosition("units", dataUnits)
	f.MarkPosition("classes", dataClasses)
	f.MarkPosition("items", dataItems)

	// FE6 uses same unit stride (0x88)
	f.WriteFE8UUnit(dataUnits, UnitData{
		NameID: 0x0001,
		CharID: 0x01,
		Class:  0x01,
		Level:  1,
		HP:     18,
		Pow:    5,
		Skl:    5,
		Spd:    7,
		Def:    7,
		Res:    0,
		Lck:    7,
	})
	for i := 0; i < 0x88; i++ {
		f.data[dataUnits+0x88+i] = 0
	}

	// FE6 class stride is 0x48 (no promotion bonuses)
	// Write minimal class data
	f.WriteU16(dataClasses+0x00, 0x0100) // Name ID
	f.WriteU8(dataClasses+0x04, 0x01)    // Class ID
	f.WriteU8(dataClasses+0x05, 18)      // HP
	for i := 0; i < 0x48; i++ {
		f.data[dataClasses+0x48+i] = 0
	}

	// FE6 item stride is 0x20 (no weapon rank bonus)
	f.WriteU16(dataItems+0x00, 0x0200) // Name ID
	f.WriteU8(dataItems+0x04, 0x01)    // Item ID
	for i := 0; i < 0x20; i++ {
		f.data[dataItems+0x20+i] = 0
	}

	return f
}
