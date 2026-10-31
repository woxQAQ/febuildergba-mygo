package testutil

import "bytes"

// NewQuoteFixture declares records and consumers from the native formats.
// Padding and terminator tails deliberately contain nonzero opaque bytes.
func NewQuoteFixture(version string) *FixtureBuilder {
	f := NewNativeShopFixture(version)
	type table struct {
		name     string
		pointers []int
		raw      []byte
		end      int
		wide     bool
	}
	anyChapter := byte(0x43)
	if version == "FE7J" {
		anyChapter = 0x45
	}
	pair := []byte{1, 2, anyChapter, 0xA3, 1, 0, 0, 0, 0, 0, 0, 0, 3, 0, 0, 0}
	single := []byte{1, anyChapter, 0xB2, 0xC3, 1, 0, 0, 0, 3, 0, 0, 0}
	death := []byte{
		1,
		anyChapter,
		0xB2,
		0xC3,
		1,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		3,
		0,
		0,
		0,
	}
	fallback := []byte{1, anyChapter, 0xB2, 0xC3, 0, 0, 0, 0, 0x65, 0, 0, 0}
	var tables []table
	switch version {
	case "FE6":
		pair = []byte{1, 2, 0x2D, 0xA3, 1, 0, 0, 0, 3, 0, 0, 0}
		single = []byte{
			1,
			0x2D,
			0xB2,
			0xC3,
			1,
			0,
			0,
			0,
			3,
			0,
			0,
			0,
			0xD4,
			0xE5,
			0xF6,
			0xA7,
		}
		death = []byte{
			1,
			0x2D,
			0xB2,
			0xC3,
			1,
			0,
			0,
			0,
			0x65,
			0,
			0,
			0,
			2,
			0,
			0,
			0,
		}
		tables = []table{
			{"battle", []int{0x6B660}, pair, 1, false},
			{"general", []int{0x6B744, 0x6B7A4}, single, 0, false},
			{"triangle", []int{0x6B748, 0x6B7DC}, single, 0, false},
			{"death", []int{0x6B7FC, 0x6B88C, 0x6D2BC}, death, 0, false},
		}
	case "FE7U", "FE7J":
		tables = []table{
			{"battle", []int{0x792E0}, pair, 1, false},
			{"general", []int{0x79454, 0x794DC}, single, 0, false},
			{"triangle", []int{0x79458, 0x79510}, single, 0, false},
			{"death", []int{0x79550, 0x796B4}, death, 0, false},
			{"death_lyn", []int{0x7955C, 0x7967C}, fallback, 0, false},
			{"death_other", []int{0x79558, 0x79678}, fallback, 0, false},
		}
		if version == "FE7J" {
			for i := range tables {
				for j := range tables[i].pointers {
					tables[i].pointers[j] += 0x7D0
				}
			}
		}
	case "FE8U", "FE8J":
		battlePointer, deathPointer := 0x846AC, 0x8472C
		if version == "FE8J" {
			battlePointer, deathPointer = 0x86978, 0x869F8
		}
		pair = []byte{1, 0, 2, 0, 0xFF, 0, 3, 0, 1, 0, 0xD4, 0xE5, 0, 0, 0, 0}
		death = []byte{1, 0, 0xFF, 0xFF, 0x65, 0, 1, 0, 0, 0, 0, 0}
		tables = []table{
			{"battle", []int{battlePointer}, pair, 0, true},
			{"death", []int{deathPointer}, death, 0, true},
		}
	}
	for i, table := range tables {
		at := 0x260000 + i*0x400
		f.MarkPosition("quote_"+table.name, at)
		f.tablePointers["quote_"+table.name] = table.pointers
		for _, site := range table.pointers {
			f.WritePointer(site, at)
		}
		f.WriteAt(at, table.raw)
		second := bytes.Clone(table.raw)
		second[0] = 2
		f.WriteAt(at+len(second), second)
		terminator := bytes.Repeat([]byte{0xA9}, len(second))
		terminator[table.end] = 0
		if table.wide {
			terminator[0], terminator[1] = 0xFF, 0xFF
		}
		f.WriteAt(at+len(second)*2, terminator)
	}
	return f
}
