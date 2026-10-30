package records_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/woxQAQ/febuildergba-mygo/game/edit"
	"github.com/woxQAQ/febuildergba-mygo/game/records"
	"github.com/woxQAQ/febuildergba-mygo/internal/testutil"
)

func TestSessionLayoutValidation(t *testing.T) {
	s := testutil.Open(t, testutil.NewMinimalFE8U())
	original := s.Document.Snapshot()
	for _, layout := range []records.Layout{
		{Address: -4, Stride: 52, Count: 2},
		{Address: 0x200001, Stride: 52, Count: 2},
		{Address: 0x200000, Stride: 4, Count: 2},
		{Address: 0x200000, Stride: 53, Count: 2},
		{Address: 0x200000, Stride: 52, Count: 0},
		{Address: 0x200000, Stride: 52, Count: 256},
		{Address: len(original.Data) - 4, Stride: 52, Count: 2},
		{Address: 0x200000, Stride: int(^uint(0) >> 1), Count: 2},
	} {
		if err := s.Records.UseLayout(
			records.Units,
			layout,
		); !errors.Is(
			err,
			edit.ErrLayout,
		) {
			t.Errorf("layout %+v: %v", layout, err)
		}
	}
	if err := s.Records.UseLayout(
		records.Units,
		records.Layout{Address: 0x200000, Stride: 52, Count: 2},
	); err != nil {
		t.Fatal(err)
	}
	if !s.Records.Table(records.Units).Manual ||
		s.Records.Table(records.Units).Count != 2 ||
		!bytes.Equal(original.Data, s.Document.Snapshot().Data) {
		t.Fatal("manual interpretation changed ROM bytes or ignored layout")
	}
	s.Records.UseAutomaticLayout(records.Units)
	if s.Records.Table(records.Units).Manual ||
		s.Records.Table(records.Units).Count != 255 {
		t.Fatal("automatic interpretation not restored")
	}
}
