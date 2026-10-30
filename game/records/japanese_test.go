package records_test

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestJapaneseDescriptionCharacters(t *testing.T) {
	for _, version := range testutil.TextVersions {
		if !version.Japanese {
			continue
		}
		t.Run(version.ID, func(t *testing.T) {
			f := testutil.DescriptionFixtureFor(version)
			f.WriteU32(testutil.NameTree+16*4, 0x80008A83) // リ.
			f.WriteU32(testutil.NameTree+17*4, 0x80008083) // ム.
			f.WriteU32(testutil.NameTree+28*4, 0x800000B1) // Half-width ｱ.
			f.WriteAt(
				testutil.NameRaw+16,
				[]byte{0x83, 0x8A, 0x80, 0x21, 0x83, 0x80, 0x1F, 0},
			)
			s := testutil.Open(t, f)
			original, err := s.Records.Text(
				records.Items,
				1,
				"use_description_id",
			)
			if err != nil || !original.Editable ||
				original.Content != "リ[ToggleRed]ム[.]" {
				t.Fatalf(
					"raw Shift-JIS glyphs and commands did not retain their boundaries: %v",
					err,
				)
			}
			const content = " リABｱ[ToggleRed]ム[.] \n"
			edit, err := s.Records.PlanTextEdit(
				records.Items,
				1,
				"use_description_id",
				content,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Texts.Apply(edit); err != nil {
				t.Fatal(err)
			}
			value, err := s.Records.Text(records.Items, 1, "use_description_id")
			if err != nil || value.Content != content ||
				!slices.Equal(value.ControlCodes(), original.ControlCodes()) ||
				gba.Uint32(
					s.Document.Snapshot().Data,
					testutil.NameTable+24,
				) >= 0x88000000 {
				t.Fatalf(
					"mixed Japanese/ASCII text, whitespace or raw-input conversion changed the content: %v",
					err,
				)
			}
			before := s.Document.Snapshot()
			for _, invalid := range []string{
				strings.Replace(content, "リ", "🙂", 1),
				strings.Replace(content, "リ", "¥", 1),
				strings.Replace(content, "[ToggleRed]", "[Item]", 1),
				content + string([]byte{0xFF}),
				content + "\t",
				content + strings.Repeat("リ", 2049),
			} {
				if _, err := s.Records.PlanTextEdit(
					records.Items,
					1,
					"use_description_id",
					invalid,
				); !errors.Is(
					err,
					text.ErrTextEncoding,
				) {
					t.Fatalf(
						"accepted a lossy or invalid Japanese edit: %v",
						err,
					)
				}
			}
			large := strings.Repeat("リ", 1400) + "[ToggleRed]ム[.]"
			if _, err := s.Records.PlanTextEdit(
				records.Items,
				1,
				"use_description_id",
				large,
			); err != nil {
				t.Fatalf(
					"applied the byte bound to UTF-8 instead of game data: %v",
					err,
				)
			}
			if s.Document.State().Revision != before.Revision ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("planning or rejected input changed the document")
			}
			f = testutil.DescriptionFixtureFor(version)
			f.WriteU32(
				testutil.NameTree+16*4,
				0x80007F83,
			) // An invalid Shift-JIS trail byte.
			broken := testutil.Open(t, f)
			value, err = broken.Records.Text(records.Units, 1, "description_id")
			if err != nil || value.Editable ||
				!strings.Contains(value.Content, "[0x0083]") {
				t.Fatalf(
					"invalid Shift-JIS bytes were not preserved read-only: %v",
					err,
				)
			}
			if _, err := broken.Records.PlanTextEdit(
				records.Units,
				1,
				"description_id",
				value.Content+"\n",
			); fault.CodeOf(err) != fault.CodeOf(value.ReadOnlyReason) ||
				broken.Document.State().Dirty {
				t.Fatalf(
					"invalid source bytes were silently rewritten: %v",
					err,
				)
			}
		})
	}
}

func TestJapaneseDescriptionCompactGlyphs(t *testing.T) {
	for _, sample := range []struct {
		version testutil.TextVersion
		glyphs  []uint16
		want    []byte
	}{
		{testutil.TextVersions[1], []uint16{0x7084, 0x7184, 0x7284, 0x7384}, []byte{0xA0, 0x1C, 0, 0, 0, 0}},
		{testutil.TextVersions[3], []uint16{0x4087, 0x4187, 0x4287, 0x4387, 0x4487, 0x4587, 0x4687}, []byte{0xA0, 0x9C, 0xEE, 0, 0, 0, 0}},
		{testutil.TextVersions[4], []uint16{0x4A87, 0x4B87, 0x4C87, 0x4D87, 0x4E87, 0x4F87, 0x5087}, []byte{0xA0, 0x9C, 0xEE, 0, 0, 0, 0}},
	} {
		t.Run(sample.version.ID, func(t *testing.T) {
			f := testutil.DescriptionFixtureFor(sample.version)
			for i := range 7 {
				f.WriteU16(testutil.NameTree+i*4, uint16(i*2+1))
				f.WriteU16(testutil.NameTree+i*4+2, uint16(i*2+2))
			}
			for i := range 8 {
				word := uint32(0x80000000)
				if i < len(sample.glyphs) {
					word |= uint32(sample.glyphs[i])
				}
				f.WriteU32(testutil.NameTree+(i+7)*4, word)
			}
			f.WriteAt(
				testutil.DescriptionStream,
				[]byte{7, 0},
			) // An end leaf makes the existing description empty.
			s := testutil.Open(t, f)
			const content = "(ﾄﾞﾗｺﾞﾝﾏｽﾀｰ)"
			edit, err := s.Records.PlanTextEdit(
				records.Classes,
				0,
				"description_id",
				content,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Texts.Apply(edit); err != nil {
				t.Fatal(err)
			}
			value, err := s.Records.Text(records.Classes, 0, "description_id")
			if err != nil || value.Content != content ||
				len(value.ControlCodes()) != 0 ||
				!bytes.Equal(
					s.Document.Snapshot().Data[edit.Destination():],
					sample.want,
				) {
				t.Fatalf(
					"description glyphs were split into raw controls or lost the compact mapping: %v",
					err,
				)
			}
		})
	}
}
