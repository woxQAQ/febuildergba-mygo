package game

import "slices"

// chapterFormat captures the MapSetting forms' version-specific layout. Text
// offsets used by the legacy stopping predicate can differ from the title
// shown in the chapter list, notably in FE7U.
type chapterFormat struct {
	pointers          []int
	stride, name      int
	boundaryByte      int
	boundaryTexts     []int
	fields            []Field
	strideInstruction int
}

func chapterText(id string, offset int, role TextRole) Field {
	f := field(id, "chapter_text", offset, 2)
	f.Text = role
	return f
}

func chapterCommonFields(weather, background, wall int) []Field {
	label := field("chapter_label_pointer", GroupAdvanced, 0, 4)
	label.Hex = true
	return []Field{
		label,
		field("map_object_id", "chapter_map", 4, 1),
		field("map_object_alt_id", "chapter_map", 5, 1),
		field("map_palette_id", "chapter_map", 6, 1),
		field("map_tileset_id", "chapter_map", 7, 1),
		field("map_id", "chapter_map", 8, 1),
		field("tile_animation_1_id", "chapter_map", 9, 1),
		field("tile_animation_2_id", "chapter_map", 10, 1),
		field("map_changes_id", "chapter_map", 11, 1),
		field("fog_level", GroupGeneral, 12, 1),
		field("battle_preparations", GroupGeneral, 13, 1),
		field("chapter_title_image_id", GroupGeneral, 14, 1),
		field("weather", GroupGeneral, weather, 1),
		field("battle_background_id", GroupGeneral, background, 1),
		field("breakable_wall_hp", GroupGeneral, wall, 1),
	}
}

func chapterMusicFields(offset, width int) []Field {
	return []Field{
		field("player_music_id", "chapter_music", offset, width),
		field("enemy_music_id", "chapter_music", offset+width, width),
		field("ally_music_id", "chapter_music", offset+width*2, width),
	}
}

func fe8Chapters(pointers ...int) chapterFormat {
	fields := chapterCommonFields(18, 19, 44)
	fields = append(fields, chapterMusicFields(22, 2)...)
	fields = append(fields,
		field("chapter_title_image_alt_id", GroupGeneral, 15, 1),
		field("player_music_alt_id", "chapter_music", 28, 2),
		field("enemy_music_alt_id", "chapter_music", 30, 2),
		field("ally_music_alt_id", "chapter_music", 32, 2),
		chapterText("name_id", 112, NameText),
		chapterText("chapter_name_alt_id", 114, DescriptionText),
		field("chapter_events_id", "chapter_map", 116, 1),
		field("world_map_events_id", "chapter_map", 117, 1),
		field("chapter_number", GroupGeneral, 128, 1),
		field("victory_music_threshold", GroupGeneral, 134, 1),
		field("fade_before_events", GroupGeneral, 135, 1),
		chapterText("objective_text_id", 136, DescriptionText),
		chapterText("objective_detail_text_id", 138, DescriptionText),
	)
	return chapterFormat{
		pointers:      pointers,
		stride:        148,
		name:          112,
		boundaryByte:  12,
		boundaryTexts: []int{112, 114, 136, 138},
		fields:        fields,
	}
}

func fe7Chapters(pointer int, english bool) chapterFormat {
	format := fe8Chapters(pointer)
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
			chapterText("chapter_lettering_id", 112, DescriptionText),
			chapterText("chapter_lettering_alt_id", 114, DescriptionText),
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
	format.fields = append(format.fields,
		field("preparation_chapter_alt_id", GroupGeneral, 129+shift, 1),
		field("opening_music_id", "chapter_music", 38, 2),
		field("opening_music_route_1_id", "chapter_music", 40, 2),
		field("opening_music_route_2_id", "chapter_music", 42, 2),
		chapterText("fortune_intro_text_id", 118+shift, DescriptionText),
		chapterText("fortune_route_1_text_id", 120+shift, DescriptionText),
		chapterText("fortune_route_2_text_id", 122+shift, DescriptionText),
		chapterText("fortune_confirm_text_id", 124+shift, DescriptionText),
	)
	return format
}

func fe6Chapters() chapterFormat {
	fields := chapterCommonFields(17, 18, 25)
	fields = append(fields, chapterMusicFields(20, 1)...)
	fields = append(fields,
		field("hard_mode_bonus", GroupGeneral, 19, 1),
		field("world_map_music_id", "chapter_music", 23, 1),
		field("opening_music_id", "chapter_music", 24, 1),
		chapterText("objective_text_id", 48, DescriptionText),
		chapterText("army_top_text_id", 50, DescriptionText),
		chapterText("army_bottom_text_id", 52, DescriptionText),
		chapterText("name_id", 56, NameText),
		field("chapter_events_id", "chapter_map", 58, 1),
		field("world_map_events_id", "chapter_map", 59, 1),
		chapterText("world_map_name_id", 60, DescriptionText),
		field("chapter_number", GroupGeneral, 62, 1),
		field("world_map_x", GroupGeneral, 63, 1),
		field("world_map_y", GroupGeneral, 64, 1),
		field("world_map_marker_x", GroupGeneral, 65, 1),
		field("world_map_marker_y", GroupGeneral, 66, 1),
		field("victory_music_threshold", GroupGeneral, 67, 1),
	)
	return chapterFormat{
		pointers:          []int{0x2BB20},
		stride:            68,
		name:              56,
		boundaryByte:      11,
		boundaryTexts:     []int{56, 48, 60},
		fields:            fields,
		strideInstruction: 0x2BB12,
	}
}

func (g gameFormat) tableRules(data []byte) []tableRule {
	rules := slices.Clone(g.tables)
	format := g.chapters
	if format.stride == 0 {
		return rules
	}
	stride := format.stride
	if format.strideInstruction != 0 &&
		within(data, format.strideInstruction, 2) &&
		u16(data, format.strideInstruction) == 0x2048 {
		stride = 72
	}
	textTable, _ := g.TextTable(data)
	rule := tableRule{
		kind:          Chapters,
		pointers:      format.pointers,
		stride:        stride,
		count:         256,
		probe:         -1,
		nameOffset:    format.name,
		pointerFields: []int{0},
		fields:        format.fields,
		accepts:       func(data []byte, at int) bool { return format.accepts(data, at, stride, textTable.Count) },
	}
	return append(rules, rule)
}

func (f chapterFormat) accepts(data []byte, at, stride, textCount int) bool {
	if !within(data, at, stride) {
		return false
	}
	if _, valid := pointer(data, at); valid {
		return true
	}
	// Preserve the MapSetting forms' fallback for records without CP. The
	// legacy byte check is at 12 (11 in FE6); it is not the weather field.
	if data[at+f.boundaryByte] >= 0xE || textCount == 0 {
		return false
	}
	a, b := u32(data, at+4), u32(data, at+8)
	if (a == 0 || a == 0xFFFFFFFF) && (b == 0 || b == 0xFFFFFFFF) {
		return false
	}
	for _, offset := range f.boundaryTexts {
		if int(u16(data, at+offset)) >= textCount {
			return false
		}
	}
	return true
}
