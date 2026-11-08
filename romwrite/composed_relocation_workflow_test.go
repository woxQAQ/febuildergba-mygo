package romwrite_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"reflect"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

func TestEncodedReplacementAndRelocationCompose(t *testing.T) {
	data, protection, record, palette, movement := resourceEditFixture()
	copy(data[32:40], []byte{0x24, 0, 0, 8, 0xB1, 0xB2, 0xB3, 0xB4})
	doc := openResourceDocument(t, data)
	builder := romwrite.NewBuilder(data, protection)
	if at, err := builder.AppendEncoded(func(at int) ([]byte, error) {
		if at != 132 {
			t.Fatalf("first allocation: %d", at)
		}
		return []byte{0x11, 0x12, 0x13}, nil
	}); err != nil || at != 132 {
		t.Fatalf("first payload: %d, %v", at, err)
	}
	before := builder.Plan()
	encodingFailure := errors.New("encoding failed")
	if _, err := builder.ReplaceEncoded(func(at int) ([]byte, error) {
		if at != 136 {
			t.Fatalf("replacement destination: %d", at)
		}
		return []byte{0xFF}, encodingFailure
	}, movement); !errors.Is(err, encodingFailure) || !reflect.DeepEqual(builder.Plan(), before) {
		t.Fatalf("encoding failure changed the plan: %v", err)
	}
	if _, err := builder.AppendEncoded(func(int) ([]byte, error) {
		_, err := builder.Append([]byte{0xFF})
		return nil, err
	}); !errors.Is(err, romwrite.ErrRange) || !reflect.DeepEqual(builder.Plan(), before) {
		t.Fatalf("reentrant encoding changed the plan: %v", err)
	}
	move := romwrite.Move{
		Owner:  palette.Owner,
		Source: romwrite.Range{Start: 32, End: 40},
		References: []romwrite.Reference{
			{Owner: palette.Owner, Address: 8},
			{Address: 32, Offset: 4},
		},
		Available: []romwrite.FreeRange{
			{
				Start: 64, End: 80,
				Source: "declared allocation",
			},
		},
	}
	wrongOwner := move
	wrongOwner.References = []romwrite.Reference{
		{Owner: movement.Owner, Address: 8},
	}
	if _, err := builder.Relocate(
		wrongOwner,
	); !errors.Is(err, romwrite.ErrClaim) ||
		!reflect.DeepEqual(builder.Plan(), before) {
		t.Fatalf("relocation skipped pointer ownership: %v", err)
	}
	conflict := move
	conflict.Available = []romwrite.FreeRange{
		{
			Start: 48, End: 64,
			Source: "conflicting allocation",
		},
	}
	if _, err := builder.Relocate(
		conflict,
	); !errors.Is(err, romwrite.ErrClaim) ||
		!reflect.DeepEqual(builder.Plan(), before) {
		t.Fatalf("relocation skipped existing-range protection: %v", err)
	}
	for _, missing := range []romwrite.Protection{{Pointer: protection.Pointer}, {Range: protection.Range}} {
		unguarded := romwrite.NewBuilder(data, missing)
		if _, err := unguarded.Relocate(
			move,
		); !errors.Is(err, romwrite.ErrClaim) ||
			len(unguarded.Plan().Edit.Changes) != 0 {
			t.Fatalf("relocation accepted missing protection: %v", err)
		}
	}
	if destination, err := builder.Relocate(
		move,
	); err != nil ||
		destination != (romwrite.Range{Start: 64, End: 72}) {
		t.Fatalf("relocation: %+v, %v", destination, err)
	}
	before = builder.Plan()
	if err := builder.Patch(
		palette.Owner,
		32,
		[]byte{0},
	); !errors.Is(err, romwrite.ErrRange) ||
		!reflect.DeepEqual(builder.Plan(), before) {
		t.Fatalf("later write changed the retained source: %v", err)
	}
	another := romwrite.Move{
		Owner: movement.Owner, Source: romwrite.Range{Start: 48, End: 56},
		References: []romwrite.Reference{{Owner: movement.Owner, Address: 12}},
		Available:  move.Available,
	}
	if _, err := builder.Relocate(
		another,
	); !errors.Is(err, romwrite.ErrRange) ||
		!reflect.DeepEqual(builder.Plan(), before) {
		t.Fatalf("relocation reused an earlier allocation: %v", err)
	}
	if destination, err := builder.ReplaceEncoded(func(at int) ([]byte, error) {
		if at != 136 {
			t.Fatalf(
				"failed or reused allocations changed the next append: %d",
				at,
			)
		}
		content := binary.LittleEndian.AppendUint32(
			nil,
			testutil.PointerWord(at+4),
		)
		return append(content, 0xA1, 0xA2, 0xA3, 0xA4), nil
	}, movement); err != nil || destination != 136 {
		t.Fatalf("encoded replacement: %d, %v", destination, err)
	}
	if err := builder.Patch(record, 16, []byte{0x31, 0x32}); err != nil {
		t.Fatal(err)
	}
	plan := builder.Plan()
	if plan.Destination != (romwrite.Range{Start: 64, End: 144}) {
		t.Fatalf("allocation envelope: %+v", plan.Destination)
	}
	if err := doc.ApplyEdit(0, "composed relocation", plan.Edit); err != nil {
		t.Fatal(err)
	}
	want := append(
		bytes.Clone(data),
		0xFF,
		0xFF,
		0xFF,
		0x11,
		0x12,
		0x13,
		0xFF,
		0x8C,
		0,
		0,
		8,
		0xA1,
		0xA2,
		0xA3,
		0xA4,
	)
	copy(want[8:18], []byte{0x40, 0, 0, 8, 0x88, 0, 0, 8, 0x31, 0x32})
	copy(want[64:72], []byte{0x44, 0, 0, 8, 0xB1, 0xB2, 0xB3, 0xB4})
	if !bytes.Equal(doc.Snapshot().Data, want) {
		t.Fatal(
			"combined writes, internal fixup, retained source or independent reference differ",
		)
	}
	if !doc.Undo() || !bytes.Equal(doc.Snapshot().Data, data) ||
		doc.State().CanUndo {
		t.Fatal("relocation and append did not undo together")
	}
	if !doc.Redo() || !bytes.Equal(doc.Snapshot().Data, want) {
		t.Fatal("relocation and append did not redo together")
	}
}

func TestPointerRetargetAndClearHistory(t *testing.T) {
	data, protection, _, pointer, _ := resourceEditFixture()
	doc := openResourceDocument(t, data)
	builder := romwrite.NewBuilder(data, protection)
	if err := builder.Repoint(pointer, 0); err != nil {
		t.Fatal(err)
	}
	if err := doc.ApplyEdit(
		0,
		"point at ROM start",
		builder.Plan().Edit,
	); err != nil {
		t.Fatal(err)
	}
	atStart := doc.Snapshot()
	if !bytes.Equal(atStart.Data[8:12], []byte{0, 0, 0, 8}) {
		t.Fatal("file offset zero became a null pointer")
	}
	pointer.ExpectedWord = 0x08000000
	builder = romwrite.NewBuilder(atStart.Data, protection)
	if err := builder.ClearPointer(pointer); err != nil {
		t.Fatal(err)
	}
	if err := doc.ApplyEdit(
		atStart.Revision,
		"clear pointer",
		builder.Plan().Edit,
	); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(doc.Snapshot().Data[8:12], []byte{0, 0, 0, 0}) {
		t.Fatal("clearing did not write a null pointer")
	}
	if !doc.Undo() || !bytes.Equal(doc.Snapshot().Data, atStart.Data) ||
		!doc.Undo() ||
		!bytes.Equal(doc.Snapshot().Data, data) {
		t.Fatal("retarget and clear did not retain distinct history operations")
	}
}
