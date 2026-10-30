package records_test

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/fault"
	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestDescriptionEdits(t *testing.T) {
	for _, sample := range []struct {
		kind  records.Kind
		id    int
		field string
	}{
		{records.Units, 1, "description_id"},
		{records.Classes, 0, "description_id"},
		{records.Items, 0, "description_id"},
		{records.Items, 0, "use_description_id"},
	} {
		t.Run(string(sample.kind)+"/"+sample.field, func(t *testing.T) {
			s := testutil.Open(t, testutil.DescriptionFixture())
			original, err := s.Records.Text(
				sample.kind,
				sample.id,
				sample.field,
			)
			if err != nil || original.Content != testutil.OriginalDescription ||
				!original.Editable ||
				!slices.Equal(
					original.ControlCodes(),
					[]uint16{0x80, 0x21, 0x10, 0x0141, 0x1F},
				) {
				t.Fatalf(
					"control boundaries were lost while reading: %+v, %v",
					original,
					err,
				)
			}
			before := s.Document.Snapshot()
			edit, err := s.Records.PlanTextEdit(
				sample.kind,
				sample.id,
				sample.field,
				testutil.EditedDescription,
			)
			if err != nil {
				t.Fatal(err)
			}
			wantUses := []text.Use{
				{Kind: string(records.Units), ID: 1, Field: "description_id"},
				{Kind: string(records.Units), ID: 2, Field: "name_id"},
				{Kind: string(records.Units), ID: 2, Field: "description_id"},
				{Kind: string(records.Classes), ID: 0, Field: "description_id"},
				{Kind: string(records.Items), ID: 0, Field: "description_id"},
				{
					Kind:  string(records.Items),
					ID:    0,
					Field: "use_description_id",
				},
			}
			if edit.TextID() != 4 || !slices.Equal(edit.Uses(), wantUses) ||
				s.Document.State().Revision != before.Revision {
				t.Fatal("preview missed shared fields or modified the document")
			}
			if err := s.Texts.Apply(edit); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(
				s.Document.Snapshot().Data[edit.Destination():],
				[]byte{0x2C, 0xE6, 0x91, 0xA8, 0, 0, 0, 0},
			) {
				t.Fatal(
					"description did not match the independent fixed bitstream",
				)
			}
			value, err := s.Records.Text(records.Items, 0, "use_description_id")
			if err != nil || value.Content != testutil.EditedDescription ||
				!slices.Equal(value.ControlCodes(), original.ControlCodes()) {
				t.Fatalf("shared content or 16-bit arguments changed: %v", err)
			}
			alias, err := s.Records.Text(records.Classes, 1, "description_id")
			if err != nil || alias.Content != testutil.OriginalDescription ||
				!bytes.Equal(
					before.Data[testutil.DescriptionStream:testutil.DescriptionStream+8],
					s.Document.Snapshot().Data[testutil.DescriptionStream:testutil.DescriptionStream+8],
				) {
				t.Fatal(
					"independent alias or original resource was overwritten",
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				!s.Document.Redo() {
				t.Fatal(
					"single undo/redo did not restore bytes, pointer and file length",
				)
			}
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			value, err = reopened.Records.Text(
				sample.kind,
				sample.id,
				sample.field,
			)
			if err != nil || value.Content != testutil.EditedDescription ||
				!slices.Equal(value.ControlCodes(), original.ControlCodes()) {
				t.Fatalf("saved description did not reopen losslessly: %v", err)
			}
			next, err := reopened.Records.PlanTextEdit(
				sample.kind,
				sample.id,
				sample.field,
				value.Content+"\n",
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Texts.Apply(next); err != nil {
				t.Fatal(err)
			}
			value, err = reopened.Records.Text(
				sample.kind,
				sample.id,
				sample.field,
			)
			if err != nil || value.Content != testutil.EditedDescription+"\n" {
				t.Fatalf("could not continue editing after reopening: %v", err)
			}
		})
	}
}

func TestDescriptionRejectsLossyEdits(t *testing.T) {
	s := testutil.Open(t, testutil.DescriptionFixture())
	before := s.Document.Snapshot()
	for _, sample := range []struct {
		content string
		want    fault.Code
	}{
		{testutil.EditedDescription + "[Unknown]", text.ErrTextControl},
		{strings.Replace(testutil.EditedDescription, "[ToggleRed]", "[Item]", 1), text.ErrTextEncoding},
		{strings.Replace(testutil.EditedDescription, "[LoadFace][0x0141]", "[0x0141][LoadFace]", 1), text.ErrTextControl},
		{testutil.EditedDescription + "[LoadFace]", text.ErrTextArgument},
		{testutil.EditedDescription + "[0x0000]", text.ErrTextControl},
		{testutil.EditedDescription + "[0x0100]", text.ErrTextControl},
		{testutil.EditedDescription + "[0x01]", text.ErrTextEncoding},
		{testutil.EditedDescription + "\\n", text.ErrTextEncoding},
		{testutil.EditedDescription + "é", text.ErrTextEncoding},
		{testutil.EditedDescription + "\x00", text.ErrTextEncoding},
		{testutil.EditedDescription + "\t", text.ErrTextEncoding},
		{testutil.EditedDescription + strings.Repeat("A", 4097), text.ErrTextEncoding},
	} {
		if _, err := s.Records.PlanTextEdit(
			records.Units,
			1,
			"description_id",
			sample.content,
		); fault.CodeOf(
			err,
		) != sample.want {
			t.Fatalf(
				"text %q: got %v, want %v",
				sample.content,
				err,
				sample.want,
			)
		}
	}
	for _, content := range []string{testutil.OriginalDescription, strings.ReplaceAll(testutil.OriginalDescription, "\n", "\r\n")} {
		if _, err := s.Records.PlanTextEdit(
			records.Units,
			1,
			"description_id",
			content,
		); !errors.Is(
			err,
			edit.ErrValue,
		) {
			t.Fatalf("unchanged text was unnecessarily reencoded: %v", err)
		}
	}
	if _, err := s.Records.PlanTextEdit(
		records.Units,
		1,
		"base.hp",
		"AB",
	); !errors.Is(
		err,
		edit.ErrField,
	) {
		t.Fatalf("numeric field accepted as a text ID: %v", err)
	}
	if _, err := s.Records.PlanTextEdit(
		records.Items,
		1,
		"description_id",
		"AB",
	); !errors.Is(
		err,
		text.ErrText,
	) {
		t.Fatalf("reserved text ID zero accepted: %v", err)
	}
	if s.Document.State().Revision != before.Revision ||
		s.Document.State().Dirty ||
		!bytes.Equal(before.Data, s.Document.Snapshot().Data) {
		t.Fatal("rejected input left document changes")
	}
}

func TestUnsupportedDescriptionControls(t *testing.T) {
	for _, sample := range []struct {
		label string
		leaf  int
		word  uint32
	}{
		{"unknown-command", 1, 0x1E},
		{"unknown-extension", 7, 0x26},
		{"missing-argument", 9, 0},
		{"unsupported-packed-portrait", 1, 0x4110},
	} {
		t.Run(sample.label, func(t *testing.T) {
			f := testutil.DescriptionFixture()
			f.WriteU32(
				testutil.NameTree+(sample.leaf+15)*4,
				0x80000000|sample.word,
			)
			s := testutil.Open(t, f)
			value, err := s.Records.Text(records.Units, 1, "description_id")
			if err != nil || value.Editable || value.Content == "" {
				t.Fatalf(
					"unsupported controls were not retained for read-only display: %v",
					err,
				)
			}
			if _, err := s.Records.PlanTextEdit(
				records.Units,
				1,
				"description_id",
				value.Content+"\n",
			); fault.CodeOf(err) != fault.CodeOf(value.ReadOnlyReason) ||
				s.Document.State().Dirty {
				t.Fatalf(
					"unsupported control semantics were rewritten: %v",
					err,
				)
			}
		})
	}
}

func TestRawDescriptionAndLiteralEscapes(t *testing.T) {
	s := testutil.Open(t, testutil.DescriptionFixture())
	value, err := s.Records.Text(records.Items, 1, "use_description_id")
	if err != nil || value.Content != "B\n[.]" || !value.Editable {
		t.Fatalf("anti-Huffman text did not preserve controls: %v", err)
	}
	const content = "\\[AB]\\\\\nB[.]"
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
	value, err = s.Records.Text(records.Items, 1, "use_description_id")
	if err != nil || value.Content != content ||
		!slices.Equal(value.ControlCodes(), []uint16{0x1F}) ||
		gba.Uint32(
			s.Document.Snapshot().Data,
			testutil.NameTable+24,
		) >= 0x88000000 {
		t.Fatalf(
			"literal escapes or raw-input conversion changed the text: %v",
			err,
		)
	}
	f := testutil.DescriptionFixture()
	f.WriteU32(testutil.NameTree+16*4, 0x80002180)
	s = testutil.Open(t, f)
	value, err = s.Records.Text(records.Units, 1, "description_id")
	if err != nil || !value.Editable ||
		!strings.HasPrefix(value.Content, "[ToggleRed]") {
		t.Fatalf("packed extended command was not retained: %v", err)
	}
	edit, err = s.Records.PlanTextEdit(
		records.Units,
		1,
		"description_id",
		value.Content+"\n",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Texts.Apply(edit); err != nil {
		t.Fatal(err)
	}
	changed, err := s.Records.Text(records.Units, 1, "description_id")
	if err != nil || changed.Content != value.Content+"\n" ||
		!slices.Equal(changed.ControlCodes(), value.ControlCodes()) {
		t.Fatalf("packed command was split or changed: %v", err)
	}
}

func TestDescriptionVersions(t *testing.T) {
	for _, version := range testutil.TextVersions {
		t.Run(version.ID, func(t *testing.T) {
			f := testutil.DescriptionFixtureFor(version)
			originalContent, content := testutil.OriginalDescription, testutil.EditedDescription
			want := []byte{0x2C, 0xE6, 0x91, 0xA8, 0, 0, 0, 0}
			if version.Japanese {
				f.WriteU32(testutil.NameTree+16*4, 0x80008A83) // リ.
				f.WriteU32(
					testutil.NameTree+17*4,
					0x80008083,
				) // ム; the trail byte must not start an extended command.
				f.WriteU32(
					testutil.NameTree+24*4,
					0x80004183,
				) // A portrait argument that also forms the glyph ア.
				originalContent = "リ\n[ToggleRed][LoadFace][0x4183]ム[.]"
				content = "リリム\n[ToggleRed][LoadFace][0x4183]リ[.]"
				want = []byte{0x88, 0x24, 0xE6, 0x91, 0xA8, 0, 0, 0, 0}
			}
			s := testutil.Open(t, f)
			original, err := s.Records.Text(records.Units, 1, "description_id")
			if err != nil || !original.Editable ||
				original.Content != originalContent {
				t.Fatalf(
					"version-specific text or parameters did not decode: %v",
					err,
				)
			}
			before := s.Document.Snapshot()
			edit, err := s.Records.PlanTextEdit(
				records.Units,
				1,
				"description_id",
				content,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Texts.Apply(edit); err != nil {
				t.Fatal(err)
			}
			value, err := s.Records.Text(records.Units, 1, "description_id")
			if err != nil || value.Content != content ||
				!slices.Equal(value.ControlCodes(), original.ControlCodes()) ||
				!bytes.Equal(
					s.Document.Snapshot().Data[edit.Destination():],
					want,
				) {
				t.Fatalf(
					"character boundary, parameter or fixed stream changed: %v",
					err,
				)
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				!s.Document.Redo() {
				t.Fatal("version-specific history lost bytes")
			}
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			value, err = reopened.Records.Text(
				records.Items,
				0,
				"use_description_id",
			)
			if err != nil || value.Content != content ||
				!slices.Equal(value.ControlCodes(), original.ControlCodes()) {
				t.Fatalf("shared description did not survive saving: %v", err)
			}
			next, err := reopened.Records.PlanTextEdit(
				records.Items,
				0,
				"use_description_id",
				content+"\n",
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Texts.Apply(next); err != nil {
				t.Fatal(err)
			}
		})
	}
}
