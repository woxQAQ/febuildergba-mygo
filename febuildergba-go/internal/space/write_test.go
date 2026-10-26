package space

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/rom"
)

func TestResourceMove(t *testing.T) {
	for _, reuse := range []bool{false, true} {
		t.Run(map[bool]string{false: "append", true: "declared space"}[reuse], func(t *testing.T) {
			data := make([]byte, 129)
			binary.LittleEndian.PutUint32(data[4:], 0x08000020)
			binary.LittleEndian.PutUint32(data[8:], 0x08000020)  // Unselected shared reference.
			binary.LittleEndian.PutUint32(data[32:], 0x08000024) // Known internal reference.
			copy(data[36:48], "resourceDATA")
			move := Move{Source: Range{32, 48}, References: []Reference{{Address: 4}, {Address: 32, Offset: 4}}}
			want := 132
			if reuse {
				move.Available = []FreeRange{{Range: Range{65, 96}, Source: "explicit allocation"}}
				want = 68
			}
			path := filepath.Join(t.TempDir(), "resource.gba")
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			doc, err := rom.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			plan, err := PlanMove(data, move)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Destination.Start != want {
				t.Fatalf("destination %d, want %d", plan.Destination.Start, want)
			}
			if err := doc.ApplyEdit(0, "move", plan.Edit); err != nil {
				t.Fatal(err)
			}
			after := doc.Snapshot().Data
			if binary.LittleEndian.Uint32(after[4:]) != uint32(0x08000000+want) ||
				binary.LittleEndian.Uint32(after[want:]) != uint32(0x08000000+want+4) ||
				!bytes.Equal(after[8:12], data[8:12]) || !bytes.Equal(after[32:48], data[32:48]) ||
				!bytes.Equal(after[want+4:want+16], data[36:48]) {
				t.Fatal("resource or selected references differ")
			}
			if !reuse && !bytes.Equal(after[129:132], []byte{0xFF, 0xFF, 0xFF}) {
				t.Fatal("alignment gap was not filled with FF")
			}
			if !doc.Undo() || !bytes.Equal(data, doc.Snapshot().Data) || doc.State().CanUndo {
				t.Fatal("move did not undo completely")
			}
		})
	}
}

func TestMoveRejectsConflicts(t *testing.T) {
	data := make([]byte, 128)
	binary.LittleEndian.PutUint32(data[4:], 0x08000020)
	for _, tt := range []struct {
		name string
		move Move
		want error
	}{
		{"missing reference", Move{Source: Range{32, 48}}, ErrReference},
		{"wrong pointer", Move{Source: Range{32, 48}, References: []Reference{{Address: 8}}}, ErrReference},
		{"interior target mismatch", Move{Source: Range{32, 48}, References: []Reference{{Address: 4, Offset: 1}}}, ErrReference},
		{"duplicate reference", Move{Source: Range{32, 48}, References: []Reference{{Address: 4}, {Address: 4}}}, ErrReference},
		{"unbounded source", Move{Source: Range{32, 129}, References: []Reference{{Address: 4}}}, ErrRange},
		{"no evidence", Move{Source: Range{32, 48}, References: []Reference{{Address: 4}}, Available: []FreeRange{{Range: Range{64, 96}}}}, ErrRange},
		{"source overlap", Move{Source: Range{32, 48}, References: []Reference{{Address: 4}}, Available: []FreeRange{{Range: Range{40, 96}, Source: "declared"}}}, ErrRange},
		{"reference overlap", Move{Source: Range{32, 48}, References: []Reference{{Address: 4}}, Available: []FreeRange{{Range: Range{0, 32}, Source: "declared"}}}, ErrRange},
		{"reserved overlap", Move{Source: Range{32, 48}, References: []Reference{{Address: 4}}, Available: []FreeRange{{Range: Range{64, 96}, Source: "declared"}}, Reserved: []Range{{80, 84}}}, ErrRange},
		{"free range overlap", Move{Source: Range{32, 48}, References: []Reference{{Address: 4}}, Available: []FreeRange{{Range: Range{64, 96}, Source: "declared"}, {Range: Range{80, 100}, Source: "declared"}}}, ErrRange},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := PlanMove(data, tt.move); !errors.Is(err, tt.want) {
				t.Fatalf("error %v, want %v", err, tt.want)
			}
		})
	}
}

func TestFullROMRequiresDeclaredSpace(t *testing.T) {
	data := bytes.Repeat([]byte{0xFF}, rom.MaxSize)
	binary.LittleEndian.PutUint32(data[4:], 0x08000020)
	move := Move{Source: Range{32, 48}, References: []Reference{{Address: 4}}}
	if _, err := PlanMove(data, move); !errors.Is(err, ErrCapacity) {
		t.Fatalf("FF bytes treated as free space: %v", err)
	}
	move.Available = []FreeRange{{Range: Range{rom.MaxSize - 16, rom.MaxSize}, Source: "declared interval"}}
	plan, err := PlanMove(data, move)
	if err != nil || plan.Edit.Size != rom.MaxSize || plan.Destination.Start != rom.MaxSize-16 {
		t.Fatalf("last addressable interval: %+v, %v", plan.Destination, err)
	}
}
