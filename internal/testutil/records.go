package testutil

// NewRecordFixture fixes character modifiers, signed class data, partial weapon
// EXP, unknown attribute bits and independent external pointers for five games.
func NewRecordFixture(version string) *FixtureBuilder {
	var f *FixtureBuilder
	switch version {
	case versionFE8U:
		f = NewMinimalFE8U()
	case versionFE8J:
		f = NewMinimalFE8J()
	case versionFE7U:
		f = NewMinimalFE7U()
	case versionFE7J:
		f = NewMinimalFE7J()
	case versionFE6:
		f = NewMinimalFE6()
	default:
		panic("unsupported record fixture")
	}
	unit := f.GetPosition("units")
	stride, ranks, flags := 84, 44, 40
	if version == versionFE6 {
		stride, ranks, flags = 72, 40, 36
	}
	class := f.GetPosition("classes") + stride
	f.MarkPosition("record_unit", unit)
	f.MarkPosition("record_class", class)
	f.MarkPosition("record_class_stride", stride)
	f.WriteU8(unit+5, 1)
	f.WriteU8(unit+11, 3)
	f.WriteU8(unit+12, 0xFD)
	f.WriteU8(unit+13, 6)
	f.WriteU8(unit+18, 7)
	f.WriteU8(unit+19, 0xFF)
	f.WriteU8(unit+20, 0)
	f.WriteU8(unit+28, 75)
	f.WriteU32(unit+40, 0x80800080)
	f.WriteU8(class+4, 1)
	f.WriteU8(class+11, 20)
	f.WriteU8(class+12, 0xFE)
	f.WriteU8(class+17, 5)
	f.WriteU8(class+18, 6)
	f.WriteU8(class+19, 60)
	f.WriteU8(class+26, 0xFB)
	f.WriteU8(class+27, 0xFB)
	f.WriteU8(class+ranks, 95)
	f.WriteU32(class+flags, 0x00800001)
	if version != versionFE6 {
		f.WriteU8(class+34, 200)
	}
	second := class + stride
	f.WriteU8(second+4, 2)
	f.WriteU8(second+11, 30)
	f.WriteU8(second+17, 8)
	f.WriteU8(second+18, 7)
	f.WriteU8(second+ranks, 181)
	f.WritePointer(unit+44, 0x260000)
	f.WritePointer(class+ranks+8, 0x261000)
	f.WritePointer(class+ranks+12, 0x262000)
	return f
}
