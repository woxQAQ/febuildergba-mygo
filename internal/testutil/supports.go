package testutil

import "bytes"

// NewSupportFixture includes reciprocal relationships, an independent shared
// pointer, opaque inactive slots and a terminator whose other bytes are nonzero.
func NewSupportFixture(version string) *FixtureBuilder {
	f := NewQuoteFixture(version)
	n, size, stride, talkSize := 7, 24, 52, 16
	var pointers []int
	switch version {
	case "FE6":
		n, size, stride = 10, 32, 48
		pointers = []int{0x6AFE0}
	case "FE7U":
		talkSize = 20
		pointers = []int{
			0x78A94,
			0x78B04,
			0x78B64,
			0x9B164,
			0x9B18C,
			0x9EC2C,
			0x9ECF4,
			0x9EE00,
			0x9EE70,
		}
	case "FE7J":
		talkSize = 20
		pointers = []int{
			0x79264,
			0x792D4,
			0x79334,
			0x9BB38,
			0x9BB60,
			0x9F668,
			0x9F730,
			0x9F83C,
			0x9F8AC,
		}
	case "FE8U":
		pointers = []int{0x84784, 0x847FC}
	case "FE8J":
		pointers = []int{0x86A50, 0x86AC8}
	}
	base := f.GetPosition("units")
	for i := range 4 {
		f.WriteU8(base+i*stride+4, byte(i+1))
		f.MarkPosition("support_pointer_"+string(rune('1'+i)), base+i*stride+44)
	}
	for i := range 2 {
		raw := bytes.Repeat([]byte{0xA7}, size)
		raw[0], raw[n], raw[2*n], raw[3*n] = byte(2-i), 20, 3, 1
		at := 0x280000 + i*0x40
		f.WriteAt(at, raw)
		f.WritePointer(base+i*stride+44, at)
		f.MarkPosition("support_"+string(rune('1'+i)), at)
	}
	f.WritePointer(base+2*stride+44, 0x280000)
	f.WriteU32(base+3*stride+44, 0)
	at := 0x281000
	f.MarkPosition("support_talks", at)
	f.tablePointers["support_talks"] = pointers
	for _, site := range pointers {
		f.WritePointer(site, at)
	}
	row := bytes.Repeat([]byte{0xB9}, talkSize)
	row[0], row[1] = 1, 2
	for i := range 3 {
		if version == "FE8U" || version == "FE8J" {
			row[0], row[1], row[2], row[3] = 1, 0, 2, 0
			row[4+i*2], row[5+i*2] = 1, 0
		} else {
			copy(row[4+i*4:8+i*4], []byte{1, 0, 0, 0})
		}
	}
	f.WriteAt(at, row)
	// A repeated pair is retained in its original position.
	f.WriteAt(at+talkSize, row)
	end := bytes.Repeat([]byte{0xC8}, talkSize)
	end[0] = 0
	if version == "FE8U" || version == "FE8J" {
		end[0], end[1] = 0xFF, 0xFF
	}
	f.WriteAt(at+2*talkSize, end)
	return f
}
