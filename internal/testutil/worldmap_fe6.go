package testutil

import "testing"

// NewFE6WorldMapFixture uses chapter-selected PLIST entries, shared sites and
// independent aliases. Unreferenced entries remain outside script discovery.
func NewFE6WorldMapFixture(t *testing.T) *FixtureBuilder {
	t.Helper()
	f := NewScriptFixture("FE6")
	images := ImageFixture(t, 4).Bytes()
	f.WriteAt(ImageTable, images[ImageTable:ImageIcons])
	f.WritePointer(ImageLayouts[4].PortraitPointer, ImageTable)
	const list, scene, child, alias = 0x289000, 0x28A000, 0x28B000, 0x28D000
	f.WritePointer(0x9310C, list)
	f.WriteAt(list, make([]byte, 256*4))
	f.WritePointer(list+7*4, scene)
	f.WritePointer(list+9*4, scene)
	f.WritePointer(alias, scene)
	f.WritePointer(list+8*4, child)
	chapters := f.GetPosition("chapters")
	first := f.Bytes()[chapters : chapters+68]
	for i, id := range []byte{7, 7, 9, 0, 255} {
		f.WriteAt(chapters+i*68, first)
		f.WriteU8(chapters+i*68+59, id)
	}
	f.WriteAt(chapters+5*68, make([]byte, 68))
	at := scene
	emit := func(name string, words ...uint32) {
		if name != "" {
			f.MarkPosition("fe6_wm_"+name, at)
		}
		for _, word := range words {
			f.WriteU32(at, word)
			at += 4
		}
	}
	emit("init", 0xAABB004D, 0xDEADBEEF)
	emit("zoom", 0xCCDD004F, 0x003C0064)
	emit("face", 0x11220051, 0xAABB0003, 0x00300034, 0xCCDD0001)
	emit("text", 0x33440056, 1)
	emit("move_face", 0x55660053, 0xEEFF0003, 0x123400BC)
	emit("arrow", 0x7788005A, 0xA55A0002)
	emit("highlight", 0x8899005B, 0x00060001)
	emit("dot", 0x99AA005E, 1, 0x00C80064, 0xAABBCCDB)
	emit("flag", 0xAABB0060, 3, 0x00640096, 0xDEADBEEC)
	emit("map_label", 0xBBCC0062, 1, 0x00320064, 0xAABB0022, 0x00200003)
	emit("clear_face", 0xCCDD0052, 3)
	emit("", 0x5C, 1, 0x5D, 0x5F, 1, 0x61, 3, 0x63, 1, 0x50, 0x59)
	emit("end_scene", 0x4E)
	emit("wait", 2, 5)
	emit("end", 6, 0)
	f.WriteAt(
		child,
		[]byte{0x58, 0, 0, 0, 0x57, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0},
	)
	for name, pos := range map[string]int{"root": 0x9310C, "list": list, "site": list + 7*4, "alias_site": list + 9*4, "alias": alias, "scene": scene, "scene_end": at, "child": child, "empty": list, "last": list + 255*4} {
		f.MarkPosition("fe6_wm_"+name, pos)
	}
	return f
}
