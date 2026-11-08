package romwrite_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/rom"
	"github.com/woxQAQ/febuildergba-mygo/romwrite"
)

func TestComposedResourceEditHistory(t *testing.T) {
	data, protection, record, palette, movement := resourceEditFixture()
	doc := openResourceDocument(t, data)
	image := doc.Snapshot()
	builder := romwrite.NewBuilder(image.Data, protection)
	path := []byte{0x91, 0x92, 0x93}
	if destination, err := builder.Append(
		path,
	); err != nil ||
		destination != 132 {
		t.Fatalf("path allocation: %d, %v", destination, err)
	}
	pixels := []byte{0xA1, 0xA2, 0xA3, 0xA4, 0xA5}
	if destination, err := builder.Replace(
		pixels,
		palette,
	); err != nil ||
		destination != 136 {
		t.Fatalf("palette replacement: %d, %v", destination, err)
	}
	preview := builder.Plan()
	if err := builder.Repoint(movement, 132); err != nil {
		t.Fatal(err)
	}
	if err := builder.Patch(record, 16, []byte{0x31, 0x32}); err != nil {
		t.Fatal(err)
	}
	if len(preview.Edit.Changes) != 3 {
		t.Fatal("later operations changed an earlier preview")
	}
	path[0], pixels[0] = 0, 0
	for i := range preview.Edit.Changes {
		clear(preview.Edit.Changes[i].Data)
		preview.Edit.Changes[i].Offset = 0
	}
	preview.Edit.Size = 1
	plan := builder.Plan()
	if plan.Destination != (romwrite.Range{Start: 132, End: 141}) ||
		plan.Edit.Size != 141 {
		t.Fatalf("combined allocation: %+v", plan)
	}
	if !bytes.Equal(image.Data, data) ||
		!bytes.Equal(doc.Snapshot().Data, data) ||
		doc.State().CanUndo {
		t.Fatal("planning changed the source or document history")
	}
	if err := doc.ApplyEdit(
		image.Revision,
		"replace resources",
		plan.Edit,
	); err != nil {
		t.Fatal(err)
	}
	want := append(
		bytes.Clone(data),
		0xFF,
		0xFF,
		0xFF,
		0x91,
		0x92,
		0x93,
		0xFF,
		0xA1,
		0xA2,
		0xA3,
		0xA4,
		0xA5,
	)
	copy(want[8:18], []byte{0x88, 0, 0, 8, 0x84, 0, 0, 8, 0x31, 0x32})
	if !bytes.Equal(doc.Snapshot().Data, want) {
		t.Fatal(
			"replacement, independent reference, old resource or unknown bytes differ",
		)
	}
	if !doc.Undo() || !bytes.Equal(doc.Snapshot().Data, data) ||
		doc.State().CanUndo {
		t.Fatal("one undo did not restore all bytes and the original length")
	}
	if !doc.Redo() || !bytes.Equal(doc.Snapshot().Data, want) ||
		doc.State().CanRedo {
		t.Fatal("one redo did not restore the complete edit")
	}
	before := doc.State()
	if err := doc.ApplyEdit(
		image.Revision,
		"stale replacement",
		plan.Edit,
	); !errors.Is(err, rom.ErrStale) ||
		doc.State() != before {
		t.Fatalf("stale plan changed the document: %v", err)
	}
}

func TestResourcePlanFailureAndRecovery(t *testing.T) {
	data, protection, record, palette, movement := resourceEditFixture()
	for _, scenario := range []struct {
		name string
		fail func(*romwrite.Builder) error
		want error
	}{
		{"second reference mismatch", func(b *romwrite.Builder) error {
			wrong := movement
			wrong.ExpectedWord = 0x08000034
			_, err := b.Replace([]byte{1, 2}, palette, wrong)
			return err
		}, romwrite.ErrReference},
		{"duplicate selected reference", func(b *romwrite.Builder) error {
			_, err := b.Replace([]byte{1, 2}, palette, palette)
			return err
		}, romwrite.ErrRange},
		{"foreign pointer owner", func(b *romwrite.Builder) error {
			wrong := movement
			wrong.Owner = palette.Owner
			_, err := b.Replace([]byte{1, 2}, palette, wrong)
			return err
		}, romwrite.ErrClaim},
		{"unaligned pointer", func(b *romwrite.Builder) error {
			wrong := palette
			wrong.Address++
			_, err := b.Replace([]byte{1}, wrong)
			return err
		}, romwrite.ErrRange},
		{"pointer arithmetic overflow", func(b *romwrite.Builder) error {
			wrong := palette
			wrong.Address = int(^uint(0)>>1) - 3
			_, err := b.Replace([]byte{1}, wrong)
			return err
		}, romwrite.ErrRange},
		{"overlapping patch", func(b *romwrite.Builder) error {
			return b.Patch(record, 17, []byte{1, 2})
		}, romwrite.ErrRange},
		{"pointer ownership does not grant host ownership", func(b *romwrite.Builder) error {
			return b.Patch(palette.Owner, 8, make([]byte, 16))
		}, romwrite.ErrClaim},
		{"patch outside source", func(b *romwrite.Builder) error {
			return b.Patch(record, len(data), []byte{1})
		}, romwrite.ErrRange},
		{"missing replacement references", func(b *romwrite.Builder) error {
			_, err := b.Replace([]byte{1})
			return err
		}, romwrite.ErrReference},
		{"empty payload", func(b *romwrite.Builder) error {
			_, err := b.Replace(nil, palette)
			return err
		}, romwrite.ErrRange},
		{"unallocated target", func(b *romwrite.Builder) error {
			return b.Repoint(movement, 135)
		}, romwrite.ErrReference},
		{"non-ROM target", func(b *romwrite.Builder) error {
			return b.Repoint(movement, -1)
		}, romwrite.ErrReference},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			doc := openResourceDocument(t, data)
			builder := romwrite.NewBuilder(data, protection)
			if _, err := builder.Append([]byte{0x91, 0x92, 0x93}); err != nil {
				t.Fatal(err)
			}
			if err := builder.Patch(
				record,
				16,
				[]byte{0x31, 0x32},
			); err != nil {
				t.Fatal(err)
			}
			before := builder.Plan()
			if err := scenario.fail(builder); !errors.Is(err, scenario.want) {
				t.Fatalf("failure: %v, want %v", err, scenario.want)
			}
			if !reflect.DeepEqual(builder.Plan(), before) ||
				!bytes.Equal(doc.Snapshot().Data, data) ||
				doc.State().CanUndo {
				t.Fatal(
					"failed operation changed the accumulated edit or document",
				)
			}
			if destination, err := builder.Replace(
				[]byte{0xA1},
				palette,
			); err != nil ||
				destination != 136 {
				t.Fatalf(
					"recovery consumed space from the failed operation: %d, %v",
					destination,
					err,
				)
			}
			if err := builder.ClearPointer(movement); err != nil {
				t.Fatal(err)
			}
			if err := doc.ApplyEdit(
				0,
				"recovered edit",
				builder.Plan().Edit,
			); err != nil {
				t.Fatal(err)
			}
			want := append(
				bytes.Clone(data),
				0xFF,
				0xFF,
				0xFF,
				0x91,
				0x92,
				0x93,
				0xFF,
				0xA1,
			)
			copy(want[8:18], []byte{0x88, 0, 0, 8, 0, 0, 0, 0, 0x31, 0x32})
			if !bytes.Equal(doc.Snapshot().Data, want) || !doc.Undo() ||
				!bytes.Equal(doc.Snapshot().Data, data) ||
				doc.State().CanUndo {
				t.Fatal("recovered composition or its single undo differs")
			}
		})
	}
}

func TestResourcePlanningRequiresProtectionAndCapacity(t *testing.T) {
	data, protection, record, palette, _ := resourceEditFixture()
	builder := romwrite.NewBuilder(data, romwrite.Protection{})
	if _, err := builder.Append([]byte{1}); err != nil {
		t.Fatal(err)
	}
	before := builder.Plan()
	if _, err := builder.Replace(
		[]byte{2},
		palette,
	); !errors.Is(
		err,
		romwrite.ErrClaim,
	) {
		t.Fatalf("unguarded replacement: %v", err)
	}
	if err := builder.Patch(
		record,
		16,
		[]byte{3},
	); !errors.Is(
		err,
		romwrite.ErrClaim,
	) {
		t.Fatalf("unguarded patch: %v", err)
	}
	if err := builder.ClearPointer(
		palette,
	); !errors.Is(
		err,
		romwrite.ErrClaim,
	) {
		t.Fatalf("unguarded repoint: %v", err)
	}
	if !reflect.DeepEqual(builder.Plan(), before) {
		t.Fatal("missing protection changed the plan")
	}
	full := bytes.Repeat([]byte{0xFF}, rom.MaxSize-8)
	copy(full, data)
	builder = romwrite.NewBuilder(full, protection)
	if _, err := builder.Append([]byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	before = builder.Plan()
	if _, err := builder.Replace(
		make([]byte, 5),
		palette,
	); !errors.Is(err, romwrite.ErrCapacity) ||
		!reflect.DeepEqual(builder.Plan(), before) {
		t.Fatalf("capacity failure changed the plan: %v", err)
	}
	if destination, err := builder.Replace(
		[]byte{4, 5, 6, 7},
		palette,
	); err != nil || destination != rom.MaxSize-4 ||
		builder.Plan().Edit.Size != rom.MaxSize {
		t.Fatalf("last aligned allocation: %d, %v", destination, err)
	}
}

func resourceEditFixture() ([]byte, romwrite.Protection, romwrite.Owner, romwrite.PointerSite, romwrite.PointerSite) {
	data := bytes.Repeat([]byte{0xCC}, 129)
	copy(data[8:16], []byte{0x20, 0, 0, 8, 0x30, 0, 0, 8})
	copy(
		data[24:28],
		[]byte{0x20, 0, 0, 8},
	) // Independent reference to the old palette.
	record := romwrite.Owner{Resource: "record", ID: 1}
	palette := romwrite.PointerSite{
		Owner:        romwrite.Owner{Resource: "palette", ID: 1},
		Address:      8,
		ExpectedWord: 0x08000020,
	}
	movement := romwrite.PointerSite{
		Owner:        romwrite.Owner{Resource: "movement", ID: 1},
		Address:      12,
		ExpectedWord: 0x08000030,
	}
	claims := []romwrite.Claim{
		{Start: 8, End: 24, Owner: record},
		{Start: 8, End: 12, Owner: palette.Owner, Host: record, Site: true},
		{Start: 12, End: 16, Owner: movement.Owner, Host: record, Site: true},
		{Start: 32, End: 40, Owner: palette.Owner},
		{Start: 48, End: 56, Owner: movement.Owner},
	}
	return data, romwrite.Protection{
		Pointer: func(_ []byte, owner romwrite.Owner, site int) error {
			return romwrite.CheckPointerWrite(claims, owner, site)
		},
		Range: func(_ []byte, owner romwrite.Owner, region romwrite.Range) error {
			return romwrite.CheckRangeWrite(claims, owner, region)
		},
	}, record, palette, movement
}

func openResourceDocument(t *testing.T, data []byte) *rom.Document {
	t.Helper()
	path := filepath.Join(t.TempDir(), "resources.gba")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	doc, err := rom.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}
