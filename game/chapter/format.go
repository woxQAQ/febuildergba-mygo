package chapter

import (
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/gba"
)

// chapterFormat captures the MapSetting forms' version-specific layout. Text
// offsets used by the legacy stopping predicate can differ from the title
// shown in the chapter list, notably in FE7U.
type Format struct {
	pointers          []int
	stride, name      int
	boundaryByte      int
	boundaryTexts     []int
	fields            []records.Field
	strideInstruction int
}

func chapterText(id string, offset int, role text.Role) records.Field {
	f := records.NumericField(id, "chapter_text", offset, 2)
	f.Text = role
	return f
}

func chapterCommonFields(weather, background, wall int) []records.Field {
	label := records.NumericField(
		"chapter_label_pointer",
		records.GroupAdvanced,
		0,
		4,
	)
	label.Hex = true
	return []records.Field{
		label,
		records.NumericField("map_object_id", "chapter_map", 4, 1),
		records.NumericField("map_object_alt_id", "chapter_map", 5, 1),
		records.NumericField("map_palette_id", "chapter_map", 6, 1),
		records.NumericField("map_tileset_id", "chapter_map", 7, 1),
		records.NumericField("map_id", "chapter_map", 8, 1),
		records.NumericField("tile_animation_1_id", "chapter_map", 9, 1),
		records.NumericField("tile_animation_2_id", "chapter_map", 10, 1),
		records.NumericField("map_changes_id", "chapter_map", 11, 1),
		records.NumericField("fog_level", records.GroupGeneral, 12, 1),
		records.NumericField(
			"battle_preparations",
			records.GroupGeneral,
			13,
			1,
		),
		records.NumericField(
			"chapter_title_image_id",
			records.GroupGeneral,
			14,
			1,
		),
		records.NumericField("weather", records.GroupGeneral, weather, 1),
		records.NumericField(
			"battle_background_id",
			records.GroupGeneral,
			background,
			1,
		),
		records.NumericField(
			"breakable_wall_hp",
			records.GroupGeneral,
			wall,
			1,
		),
	}
}

func chapterMusicFields(offset, width int) []records.Field {
	return []records.Field{
		records.NumericField("player_music_id", "chapter_music", offset, width),
		records.NumericField(
			"enemy_music_id",
			"chapter_music",
			offset+width,
			width,
		),
		records.NumericField(
			"ally_music_id",
			"chapter_music",
			offset+width*2,
			width,
		),
	}
}

func FE8(pointers ...int) Format {
	fields := chapterCommonFields(18, 19, 44)
	fields = append(fields, chapterMusicFields(22, 2)...)
	fields = append(
		fields,
		records.NumericField(
			"chapter_title_image_alt_id",
			records.GroupGeneral,
			15,
			1,
		),
		records.NumericField("player_music_alt_id", "chapter_music", 28, 2),
		records.NumericField("enemy_music_alt_id", "chapter_music", 30, 2),
		records.NumericField("ally_music_alt_id", "chapter_music", 32, 2),
		chapterText("name_id", 112, text.Name),
		chapterText("chapter_name_alt_id", 114, text.Description),
		records.NumericField("chapter_events_id", "chapter_map", 116, 1),
		records.NumericField("world_map_events_id", "chapter_map", 117, 1),
		records.NumericField("chapter_number", records.GroupGeneral, 128, 1),
		records.NumericField(
			"victory_music_threshold",
			records.GroupGeneral,
			134,
			1,
		),
		records.NumericField(
			"fade_before_events",
			records.GroupGeneral,
			135,
			1,
		),
		chapterText("objective_text_id", 136, text.Description),
		chapterText("objective_detail_text_id", 138, text.Description),
	)
	return Format{
		pointers:      pointers,
		stride:        148,
		name:          112,
		boundaryByte:  12,
		boundaryTexts: []int{112, 114, 136, 138},
		fields:        fields,
	}
}

func FE7(pointer int, english bool) Format {
	format := FE8(pointer)
	// FE7U adds two title-lettering words before the actual chapter names.
	if english {
		format.stride, format.name = 152, 116
		format.boundaryTexts = []int{112, 114, 140, 142}
		for i := range format.fields {
			f := &format.fields[i]
			if f.Offset >= 112 {
				f.Offset += 4
			}
		}
		format.fields = append(format.fields,
			chapterText("chapter_lettering_id", 112, text.Description),
			chapterText("chapter_lettering_alt_id", 114, text.Description),
		)
	}
	// These bytes identify the preparation-screen chapter in FE7, rather
	// than the FE8 chapter-number display. Retain the native route distinction.
	for i := range format.fields {
		if format.fields[i].ID == "chapter_number" {
			format.fields[i].ID = "preparation_chapter_id"
		}
	}
	shift := format.name - 112
	format.fields = append(
		format.fields,
		records.NumericField(
			"preparation_chapter_alt_id",
			records.GroupGeneral,
			129+shift,
			1,
		),
		records.NumericField("opening_music_id", "chapter_music", 38, 2),
		records.NumericField(
			"opening_music_route_1_id",
			"chapter_music",
			40,
			2,
		),
		records.NumericField(
			"opening_music_route_2_id",
			"chapter_music",
			42,
			2,
		),
		chapterText("fortune_intro_text_id", 118+shift, text.Description),
		chapterText("fortune_route_1_text_id", 120+shift, text.Description),
		chapterText("fortune_route_2_text_id", 122+shift, text.Description),
		chapterText("fortune_confirm_text_id", 124+shift, text.Description),
	)
	return format
}

func FE6() Format {
	fields := chapterCommonFields(17, 18, 25)
	fields = append(fields, chapterMusicFields(20, 1)...)
	fields = append(
		fields,
		records.NumericField("hard_mode_bonus", records.GroupGeneral, 19, 1),
		records.NumericField("world_map_music_id", "chapter_music", 23, 1),
		records.NumericField("opening_music_id", "chapter_music", 24, 1),
		chapterText("objective_text_id", 48, text.Description),
		chapterText("army_top_text_id", 50, text.Description),
		chapterText("army_bottom_text_id", 52, text.Description),
		chapterText("name_id", 56, text.Name),
		records.NumericField("chapter_events_id", "chapter_map", 58, 1),
		records.NumericField("world_map_events_id", "chapter_map", 59, 1),
		chapterText("world_map_name_id", 60, text.Description),
		records.NumericField("chapter_number", records.GroupGeneral, 62, 1),
		records.NumericField("world_map_x", records.GroupGeneral, 63, 1),
		records.NumericField("world_map_y", records.GroupGeneral, 64, 1),
		records.NumericField("world_map_marker_x", records.GroupGeneral, 65, 1),
		records.NumericField("world_map_marker_y", records.GroupGeneral, 66, 1),
		records.NumericField(
			"victory_music_threshold",
			records.GroupGeneral,
			67,
			1,
		),
	)
	return Format{
		pointers:          []int{0x2BB20},
		stride:            68,
		name:              56,
		boundaryByte:      11,
		boundaryTexts:     []int{56, 48, 60},
		fields:            fields,
		strideInstruction: 0x2BB12,
	}
}

func (f Format) Rule(data []byte, textCount int) records.Rule {
	stride := f.stride
	if f.strideInstruction != 0 &&
		gba.Within(data, f.strideInstruction, 2) &&
		gba.Uint16(data, f.strideInstruction) == 0x2048 {
		stride = 72
	}
	return records.Rule{
		Kind:          records.Chapters,
		Pointers:      f.pointers,
		Stride:        stride,
		Count:         256,
		Probe:         -1,
		NameOffset:    f.name,
		PointerFields: []int{0},
		Fields:        f.fields,
		Accepts:       func(data []byte, at int) bool { return f.accepts(data, at, stride, textCount) },
	}
}

func (f Format) accepts(data []byte, at, stride, textCount int) bool {
	if !gba.Within(data, at, stride) {
		return false
	}
	if _, valid := gba.Pointer(data, at); valid {
		return true
	}
	// Preserve the MapSetting forms' fallback for records without CP. The
	// legacy byte check is at 12 (11 in FE6); it is not the weather field.
	if data[at+f.boundaryByte] >= 0xE || textCount == 0 {
		return false
	}
	a, b := gba.Uint32(data, at+4), gba.Uint32(data, at+8)
	if (a == 0 || a == 0xFFFFFFFF) && (b == 0 || b == 0xFFFFFFFF) {
		return false
	}
	for _, offset := range f.boundaryTexts {
		if int(gba.Uint16(data, at+offset)) >= textCount {
			return false
		}
	}
	return true
}
