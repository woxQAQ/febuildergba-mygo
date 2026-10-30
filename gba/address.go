package gba

import (
	"encoding/binary"
)

func Within(data []byte, offset, size int) bool {
	return offset >= 0 && size >= 0 && offset <= len(data) &&
		size <= len(data)-offset
}

func Uint16(
	data []byte,
	offset int,
) uint16 {
	return binary.LittleEndian.Uint16(data[offset:])
}

func Uint32(
	data []byte,
	offset int,
) uint32 {
	return binary.LittleEndian.Uint32(data[offset:])
}

func Pointer(data []byte, at int) (int, bool) {
	if !Within(data, at, 4) {
		return 0, false
	}
	p := Uint32(data, at)
	if p < 0x08000000 || p >= 0x0A000000 {
		return 0, false
	}
	offset := int(p - 0x08000000)
	return offset, offset < len(data)
}
