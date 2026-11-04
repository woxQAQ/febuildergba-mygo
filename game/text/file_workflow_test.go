package text_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/game/text"
	"github.com/woxQAQ/febuildergba-mygo/gba"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/space"
)

func TestTextFileEditingAndHistory(t *testing.T) {
	for _, version := range testutil.TextVersions {
		t.Run(version.ID, func(t *testing.T) {
			s := testutil.Open(t, testutil.TextFixtureFor(version))
			before := s.Document.Snapshot()
			file, err := s.Texts.File()
			if err != nil {
				t.Fatal(err)
			}
			output, err := file.Export([]uint16{4, 3})
			const expected = "[0004]\r\nA@0001@0080@0021@0010@0141B@001F\r\n[0003]\r\nB\r\n"
			if err != nil || string(output) != expected {
				t.Fatalf("legacy fixed file bytes: %q, %v", output, err)
			}
			unchanged, err := file.Import(output)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Texts.PlanImport(
				unchanged,
			); !errors.Is(
				err,
				edit.ErrValue,
			) {
				t.Fatal("export changed the game text", err)
			}
			input := []byte(
				"\xEF\xBB\xBF// notes\r\n[0004] // first\r\nAB@0080@0021[LoadFace][0x041][NL][.]\r\n\r\n[0003]\r\nA[X]\r\n[0005]\r\nA[NL][ToggleRed][LoadFace][0x141]B[.]\r\n",
			)
			draft, err := file.Import(input)
			if err != nil {
				t.Fatal(err)
			}
			clear(input)
			rows := draft.Entries()
			if len(rows) != 3 || !rows[0].Changed || !rows[1].Changed ||
				rows[2].Changed ||
				rows[0].Err != nil {
				t.Fatal("review rows", rows)
			}
			rows[0].After = "corrupt copy"
			plan, err := s.Texts.PlanImport(draft)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Entries()[0].Uses) != 6 || plan.AddedBytes() <= 0 {
				t.Fatal("sharing or allocation")
			}
			rows = plan.Entries()
			rows[0].Uses[0].Field = "corrupt copy"
			if plan.Entries()[0].Uses[0].Field == "corrupt copy" ||
				plan.Entries()[0].After == "corrupt copy" {
				t.Fatal("mutable plan")
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) {
				t.Fatal("preview wrote ROM")
			}
			if err := s.Texts.ApplyImport(plan); err != nil {
				t.Fatal(err)
			}
			after := s.Document.Snapshot()
			first, _ := gba.Pointer(after.Data, testutil.NameTable+16)
			second, _ := gba.Pointer(after.Data, testutil.NameTable+12)
			if !bytes.Equal(
				after.Data[first:first+8],
				[]byte{0x6C, 0x1E, 0x28, 0x0A, 0, 0, 0, 0},
			) ||
				!bytes.Equal(
					after.Data[second:second+5],
					[]byte{0x08, 0, 0, 0, 0},
				) {
				t.Fatal(
					"fixed Huffman streams",
					after.Data[first:first+8],
					after.Data[second:second+5],
				)
			}
			preserved := bytes.Clone(after.Data[:len(before.Data)])
			for _, site := range []int{testutil.NameTable + 16, testutil.NameTable + 12} {
				copy(preserved[site:site+4], before.Data[site:site+4])
			}
			if !bytes.Equal(preserved, before.Data) {
				t.Fatal("unknown bytes, alias or unselected entry changed")
			}
			if file.Current(s.Texts) ||
				!errors.Is(s.Texts.ApplyImport(plan), rom.ErrStale) {
				t.Fatal("stale plan accepted")
			}
			if !s.Document.Undo() || s.Document.State().CanUndo ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				!s.Document.Redo() ||
				!bytes.Equal(after.Data, s.Document.Snapshot().Data) {
				t.Fatal("one batch history")
			}
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			value, err := reopened.Texts.Read(4)
			if err != nil ||
				value.Content != "AB[ToggleRed][LoadFace][0x0041]\n[.]" {
				t.Fatal("saved batch", value, err)
			}
			alias, _ := reopened.Texts.Read(5)
			if alias.Content != testutil.OriginalDescription {
				t.Fatal("independent alias changed")
			}
		})
	}
}

func TestTextFileLiteralAndLanguageHandling(t *testing.T) {
	for _, version := range testutil.TextVersions {
		t.Run(version.ID, func(t *testing.T) {
			s := testutil.Open(t, testutil.TextFixtureFor(version))
			file, _ := s.Texts.File()
			draft, err := file.Import(
				[]byte(
					"// skipped\n[0004]\nA\t{U}\nB\t{J}\n@005B@005D@005CA\n\n[0003]\n@0000\n",
				),
			)
			if err != nil {
				t.Fatal(err)
			}
			want := "A\n\\[]\\\\A"
			if version.Japanese {
				want = "B\n\\[]\\\\A"
			}
			rows := draft.Entries()
			if rows[0].After != want || rows[1].After != "" {
				t.Fatal(
					"literal controls or language filter",
					rows[0].After,
					want,
				)
			}
			plan, err := s.Texts.PlanImport(draft)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Texts.ApplyImport(plan); err != nil {
				t.Fatal(err)
			}
			file, _ = s.Texts.File()
			content, err := file.Export([]uint16{4, 3})
			prefix := "A"
			if version.Japanese {
				prefix = "B"
			}
			if err != nil ||
				string(
					content,
				) != "[0004]\r\n"+prefix+"@0001@005B@005D@005CA\r\n[0003]\r\n@0000\r\n" {
				t.Fatal("literal fixed export", string(content), err)
			}
		})
	}
}

func TestTextFileFailuresDoNotPartiallyApply(t *testing.T) {
	for _, scenario := range []string{"duplicate", "zero", "new-id", "wide-id", "missing-header", "signed-id", "signed-marker", "invalid-utf8", "oversize", "unknown-command", "missing-argument", "unsupported-character", "embedded-end", "readonly", "protected-entry", "partial-capacity", "revision", "layout", "document"} {
		t.Run(scenario, func(t *testing.T) {
			f := testutil.TextFixtureFor(testutil.TextVersions[0])
			if scenario == "readonly" {
				f.WriteU32(testutil.NameTable+12, 0x02000000)
			}
			s := testutil.Open(t, f)
			if scenario == "protected-entry" {
				if err := s.Records.UseLayout(
					records.Items,
					records.Layout{
						Address: testutil.NameTable + 12,
						Stride:  36,
						Count:   1,
					},
				); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "partial-capacity" {
				if err := s.Document.ApplyEdit(
					s.Document.State().Revision,
					"grow",
					rom.Edit{Size: rom.MaxSize - 8},
				); err != nil {
					t.Fatal(err)
				}
			}
			file, err := s.Texts.File()
			if err != nil {
				t.Fatal(err)
			}
			content := "[0004]\nAB\n[0003]\nA\n"
			switch scenario {
			case "duplicate":
				content += "[04]\nA\n"
			case "zero":
				content = "[0000]\nA\n"
			case "new-id":
				content = "[0007]\nA\n"
			case "wide-id":
				content = "[FFFFFFFF]\nA\n"
			case "missing-header":
				content = "A\n"
			case "signed-id":
				content = "[+1]\nA\n"
			case "signed-marker":
				content += "[0005]\n@+001\n"
			case "invalid-utf8":
				content = "[0004]\n\xff\n"
			case "oversize":
				content = strings.Repeat("A", text.MaxFileSize+1)
			case "unknown-command":
				content += "[0005]\n[Unknown]\n"
			case "missing-argument":
				content += "[0005]\n[LoadFace]\n"
			case "unsupported-character":
				content += "[0005]\nC\n"
			case "embedded-end":
				content += "[0005]\n@0000A\n"
			}
			before, state := s.Document.Snapshot(), s.Document.State()
			draft, err := file.Import([]byte(content))
			if err == nil {
				var plan *text.ImportPlan
				plan, err = s.Texts.PlanImport(draft)
				if scenario == "revision" || scenario == "layout" ||
					scenario == "document" {
					if err != nil {
						t.Fatal(err)
					}
					switch scenario {
					case "revision":
						err = s.Records.Set(records.Units, []int{1}, "level", 9)
					case "layout":
						table := s.Records.Table(records.Units)
						err = s.Records.UseLayout(
							records.Units,
							records.Layout{
								Address: table.Address,
								Stride:  table.Stride,
								Count:   1,
							},
						)
					case "document":
						s = testutil.Open(t, f)
					}
					if err != nil {
						t.Fatal(err)
					}
					before, state = s.Document.Snapshot(), s.Document.State()
					if _, err := s.Texts.PlanImport(
						draft,
					); !errors.Is(
						err,
						rom.ErrStale,
					) {
						t.Fatal("stale draft", err)
					}
					err = s.Texts.ApplyImport(plan)
				}
			}
			if err == nil {
				t.Fatal("unsafe batch accepted", scenario)
			}
			if scenario == "partial-capacity" &&
				!errors.Is(err, space.ErrCapacity) {
				t.Fatal("partial allocation", err)
			}
			if scenario == "duplicate" &&
				!errors.Is(err, text.ErrFileDuplicate) {
				t.Fatal("duplicate", err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				state != s.Document.State() {
				t.Fatal("partial bytes or history", fmt.Sprint(err))
			}
		})
	}
}

func TestTextFileExportKeepsEscapedNativeText(t *testing.T) {
	f := testutil.TextFixtureFor(testutil.TextVersions[0])
	// The literal delimiters would otherwise be consumed by the legacy file
	// parser as comments, commands, language selectors or section headers.
	f.WriteAt(
		testutil.NameRaw+64,
		append([]byte("[AB] @0041 // {J} \\"), 1, 1, 0),
	)
	f.WriteU32(testutil.NameTable+12, uint32(0x88000000+testutil.NameRaw+64))
	s := testutil.Open(t, f)
	file, _ := s.Texts.File()
	output, err := file.Export([]uint16{3})
	want := "[0003]\r\n@005BAB@005D @00400041 @002F@002F @007BJ} @005C@0001@0001\r\n"
	if err != nil || string(output) != want {
		t.Fatal("escaped fixed export", string(output), err)
	}
	draft, err := file.Import(output)
	if err != nil {
		t.Fatal(err)
	}
	if draft.Entries()[0].Changed {
		t.Fatal("escaped file changed native text", draft.Entries())
	}
	before := s.Document.Snapshot()
	if _, err := s.Texts.PlanImport(draft); !errors.Is(err, edit.ErrValue) {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint32(
		before.Data[testutil.NameTable+12:],
	) != uint32(
		0x88000000+testutil.NameRaw+64,
	) {
		t.Fatal("source moved")
	}
}

func TestTextFileRetainsUnchangedReadOnlyEntries(t *testing.T) {
	f := testutil.TextFixtureFor(testutil.TextVersions[0])
	f.WriteAt(testutil.NameRaw+64, []byte{0x1E, 0})
	f.WriteU32(testutil.NameTable+12, uint32(0x88000000+testutil.NameRaw+64))
	s := testutil.Open(t, f)
	file, err := s.Texts.File()
	if err != nil {
		t.Fatal(err)
	}
	output, err := file.Export([]uint16{3})
	if err != nil || string(output) != "[0003]\r\n@001E\r\n" {
		t.Fatal("read-only native export", string(output), err)
	}
	draft, err := file.Import(output)
	if err != nil || draft.Entries()[0].Changed ||
		draft.Entries()[0].Err != nil {
		t.Fatal("unchanged read-only entry", err)
	}
	if _, err := s.Texts.PlanImport(draft); !errors.Is(err, edit.ErrValue) {
		t.Fatal(err)
	}
	draft, err = file.Import([]byte("[0003]\nA\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Texts.PlanImport(
		draft,
	); !errors.Is(
		err,
		text.ErrFileEntries,
	) {
		t.Fatal("read-only write", err)
	}
}
