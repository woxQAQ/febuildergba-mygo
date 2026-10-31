package text_test

import (
	"bytes"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestDialogueReadingView(t *testing.T) {
	for _, version := range testutil.TextVersions {
		t.Run(version.ID, func(t *testing.T) {
			f := testutil.TextFixtureFor(version)
			const address = testutil.NameRaw + 0x100
			f.WriteAt(
				address,
				[]byte{
					'A',
					2,
					'B',
					0x80,
					0x20,
					0x80,
					0x22,
					'[',
					'A',
					']',
					'\\',
					0x80,
					0x26,
					0x10,
					0x41,
					1,
					'B',
					0,
				},
			)
			f.WriteU32(testutil.NameTable+16, 0x88000000|address)
			s := testutil.Open(t, f)
			before := s.Document.Snapshot()
			value, err := s.Texts.Read(4)
			const want = "A\n\nB[Tact][Item][A]\\[0x0080][0x0026]B"
			if err != nil || value.Plain != want {
				t.Fatalf("reading view=%q, want %q: %v", value.Plain, want, err)
			}
			if value.Editable {
				t.Fatal("unknown command was treated as editable")
			}
			entries, err := s.Texts.Entries()
			if err != nil || entries[3].Plain != want {
				t.Fatal("picker lost the reading view", err)
			}
			if !bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				before.Revision != s.Document.State().Revision {
				t.Fatal("reading changed bytes")
			}
		})
	}
}
