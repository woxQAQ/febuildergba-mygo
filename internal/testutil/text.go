package testutil

const (
	DescriptionStream   = NameStream + 0x100
	OriginalDescription = "A\n[ToggleRed][LoadFace][0x0141]B[.]"
	EditedDescription   = "AB\n[ToggleRed][LoadFace][0x0141]A[.]"
)

func DescriptionFixture() *FixtureBuilder { return DescriptionFixtureFor(TextVersions[0]) }

func DescriptionFixtureFor(version TextVersion) *FixtureBuilder {
	f := NameFixtureFor(version)
	// Complete four-level tree. The fixed stream uses leaf indices
	// 1,4,6,7,8,9,2,5,0. In particular 0x0141 is one portrait argument,
	// not the visible character A followed by a newline.
	for i := range 15 {
		f.WriteU16(NameTree+i*4, uint16(i*2+1))
		f.WriteU16(NameTree+i*4+2, uint16(i*2+2))
	}
	for i, word := range []uint16{0, 'A', 'B', 0x4241, 1, 0x1F, 0x80, 0x21, 0x10, 0x0141, '[', ']', '\\', 0x1E, 0x26, ' '} {
		f.WriteU32(NameTree+(i+15)*4, 0x80000000|uint32(word))
	}
	f.WriteAt(DescriptionStream, []byte{0x28, 0xE6, 0x91, 0xA4, 0, 0, 0, 0})
	f.WritePointer(NameTable+16, DescriptionStream)
	f.WritePointer(
		NameTable+20,
		DescriptionStream,
	) // Independent ID aliases the original resource.
	f.WriteAt(NameRaw+16, []byte{'B', 1, 0x1F, 0})
	f.WriteU32(NameTable+24, 0x88000000|uint32(NameRaw+16))
	f.WriteAt(NameRaw+32, []byte{'A', 'B', 0})
	f.WriteU32(NameTable+4, 0x88000000|uint32(NameRaw+32))
	f.WriteU32(NameTable+8, 0x88000000|uint32(NameRaw+32))
	f.WriteU16(f.GetPosition("units")+2, 4)
	f.WriteU16(
		f.GetPosition("units")+version.UnitStride,
		4,
	) // A name also shares the description's ID.
	f.WriteU16(f.GetPosition("units")+version.UnitStride+2, 4)
	f.WriteU16(f.GetPosition("classes")+2, 4)
	f.WriteU16(f.GetPosition("classes")+version.ClassStride+2, 5)
	f.WriteU16(f.GetPosition("items")+2, 4)
	f.WriteU16(f.GetPosition("items")+4, 4)
	f.WriteU16(f.GetPosition("items")+version.ItemStride+4, 6)
	return f
}

const NameTree, NameRoot, NameStream, NameTable, NameRaw = 0x310000, 0x310100, 0x311000, 0x312000, 0x313000

type TextVersion struct {
	ID                                  string
	Build                               func() *FixtureBuilder
	Text, Tree, Root                    int
	UnitStride, ClassStride, ItemStride int
	Japanese                            bool
}

var TextVersions = []TextVersion{
	{"FE8U", NewMinimalFE8U, 0xA2A0, 0x6E0, 0x6DC, 52, 84, 36, false},
	{"FE8J", NewMinimalFE8J, 0xA000, 0x6E0, 0x6DC, 52, 84, 36, true},
	{"FE7U", NewMinimalFE7U, 0x12CB8, 0x6BC, 0x6B8, 52, 84, 36, false},
	{"FE7J", NewMinimalFE7J, 0x13370, 0x6E0, 0x6DC, 52, 84, 36, true},
	{"FE6", NewMinimalFE6, 0x13B10, 0x6E0, 0x6DC, 48, 72, 32, true},
}

func NameFixture() *FixtureBuilder { return NameFixtureFor(TextVersions[0]) }

func NameFixtureFor(version TextVersion) *FixtureBuilder {
	f := version.Build()
	// Fixed codes: A=0, B=10, AB=110, BC=1110, end=1111.
	// ABC requires A+BC; taking AB greedily leaves an unencodable C.
	for i, word := range []uint32{0x00020001, 0x80000041, 0x00040003, 0x80000042, 0x00060005, 0x80004241, 0x00080007, 0x80004342, 0x80000000} {
		f.WriteU32(NameTree+i*4, word)
	}
	f.WritePointer(version.Tree, NameTree)
	f.WritePointer(version.Root, NameRoot)
	f.WritePointer(NameRoot, NameTree)
	f.WritePointer(version.Text, NameTable)
	f.WriteAt(
		NameStream,
		[]byte{0x7B, 0},
	) // AB followed by the padded end symbol.
	f.WritePointer(NameTable+4, NameStream)
	f.WritePointer(
		NameTable+8,
		NameStream,
	) // Another ID aliases the original bytes.
	f.WriteU32(NameTable+12, 0x88000000|NameRaw)
	f.WriteAt(NameRaw, []byte{'B', 0})
	f.WriteU16(f.GetPosition("units"), 1)
	f.WriteU16(f.GetPosition("units")+version.UnitStride, 1)
	f.WriteU16(f.GetPosition("classes"), 1)
	f.WriteU16(f.GetPosition("classes")+version.ClassStride, 2)
	f.WriteU16(f.GetPosition("items"), 1)
	f.WriteU16(f.GetPosition("items")+version.ItemStride, 3)
	return f
}

func TextFixtureFor(version TextVersion) *FixtureBuilder {
	f := DescriptionFixtureFor(version)
	f.WritePointer(
		NameTable,
		NameRaw,
	) // ID 0 participates in the native table extent.
	return f
}
