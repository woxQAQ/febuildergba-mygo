//go:build integration

package integration

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game"
)

func TestRealDialogue(t *testing.T) {
	samples := map[string]struct {
		id, face, nextFace uint16
		entries            int
	}{
		"FE8U": {0x0900, 0x0101, 0x0102, 3403},
		"FE8J": {0x0900, 0x0101, 0x0102, 3338},
		"FE7U": {0x0900, 0x0102, 0x015B, 4925},
		"FE7J": {0x0900, 0x0101, 0x0102, 4660},
		"FE6":  {0x0001, 0x0102, 0x0150, 3341},
	}
	for _, fixture := range realROMFixtures {
		t.Run(fixture.id, func(t *testing.T) {
			sample := samples[fixture.id]
			s := openRealROM(t, fixture.filename)
			entries, err := s.TextEntries()
			if err != nil || len(entries) != sample.entries {
				t.Fatalf(
					"unexpected native text extent: %d entries, %v",
					len(entries),
					err,
				)
			}
			original, err := s.TextByID(sample.id)
			if err != nil || !original.Editable {
				t.Fatalf(
					"cannot edit fixed dialogue %04X: %v, %v",
					sample.id,
					err,
					original.ReadOnlyReason,
				)
			}
			// The native FE6 tree has no Wink argument. ToggleSmile is present
			// in the fixed native dialogue 0003; portrait words are fixed above.
			extra, rawExtra := "[Wink]", "[0x0080][0x001F]"
			tail := []uint16{0xA, 0x10, sample.face, 3, 0x80, 0x1F}
			if fixture.id == "FE6" {
				extra, rawExtra = "[ToggleSmile]", "[0x0017]"
				tail = []uint16{0xA, 0x10, sample.face, 3, 0x17}
			}
			before := s.Document.Snapshot()
			base := original.Content + fixture.descriptionSuffix
			content := base + fmt.Sprintf(
				"[OpenLeft][LoadFace][0x%04X][A]",
				sample.face,
			) + extra
			edit, err := s.PlanTextIDEdit(sample.id, content)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ApplyTextEdit(edit); err != nil {
				t.Fatal(err)
			}
			value, err := s.TextByID(sample.id)
			rawSuffix := fmt.Sprintf(
				"[0x000A][0x0010][0x%04X][0x0003]",
				sample.face,
			) + rawExtra
			if err != nil || value.Content != content ||
				!strings.HasSuffix(value.Raw, rawSuffix) {
				t.Fatalf(
					"dialogue or inserted commands changed: %v, %q",
					err,
					value.Raw,
				)
			}
			controls := value.ControlCodes()
			if len(controls) < len(tail) ||
				!slices.Equal(controls[len(controls)-len(tail):], tail) {
				t.Fatal("portrait argument or instruction boundaries were lost")
			}
			if !s.Document.Undo() ||
				!bytes.Equal(before.Data, s.Document.Snapshot().Data) ||
				!s.Document.Redo() {
				t.Fatal("dialogue history lost bytes or file length")
			}
			if err := s.Document.Save(); err != nil {
				t.Fatal(err)
			}
			reopened, err := game.Open(s.Document.State().Path)
			if err != nil {
				t.Fatal(err)
			}
			value, err = reopened.TextByID(sample.id)
			if err != nil || value.Content != content ||
				!slices.Equal(controls, value.ControlCodes()) {
				t.Fatalf("saved dialogue did not reopen losslessly: %v", err)
			}
			continued := base + fmt.Sprintf(
				"[LoadFace][0x%04X][A]",
				sample.nextFace,
			)
			next, err := reopened.PlanTextIDEdit(sample.id, continued)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.ApplyTextEdit(next); err != nil {
				t.Fatal(err)
			}
			value, err = reopened.TextByID(sample.id)
			if err != nil || value.Content != continued {
				t.Fatalf(
					"could not change the portrait parameter after reopening: %v",
					err,
				)
			}
			t.Logf(
				"text ID %04X, %d indexed entries, edited portrait %04X → %04X",
				sample.id,
				len(entries),
				sample.face,
				sample.nextFace,
			)
		})
	}
}
