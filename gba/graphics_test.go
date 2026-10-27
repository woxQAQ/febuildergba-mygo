package gba

import (
	"bytes"
	"errors"
	"testing"
)

func TestLZ77Streams(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []byte
	}{
		{
			"literal groups",
			[]byte{
				0x10,
				9,
				0,
				0,
				0,
				'A',
				'B',
				'C',
				'D',
				'E',
				'F',
				'G',
				'H',
				0,
				'I',
			},
			[]byte("ABCDEFGHI"),
		},
		{
			"overlapping reference",
			[]byte{0x10, 12, 0, 0, 0x10, 'A', 'B', 'C', 0x60, 2},
			[]byte("ABCABCABCABC"),
		},
		{
			"distance one",
			[]byte{0x10, 20, 0, 0, 0x40, 'A', 0xF0, 0, 'B'},
			append(bytes.Repeat([]byte{'A'}, 19), 'B'),
		},
		{
			"final token bounded by header",
			[]byte{0x10, 4, 0, 0, 0x40, 'A', 0xF0, 0},
			[]byte("AAAA"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecompressLZ77(tt.data, len(tt.want))
			if err != nil || !bytes.Equal(got, tt.want) {
				t.Fatalf("decoded %q, error %v; want %q", got, err, tt.want)
			}
			if _, err := DecompressLZ77(
				tt.data,
				len(tt.want)-1,
			); !errors.Is(
				err,
				ErrGraphics,
			) {
				t.Fatalf("undersized output limit: %v", err)
			}
		})
	}
}

func TestLZ77RejectsDamagedStreams(t *testing.T) {
	for _, data := range [][]byte{
		nil,
		{0x10},
		{0x11, 1, 0, 0, 0, 'A'},
		{0x10, 0, 0, 0},
		{0x10, 1, 0, 0},
		{0x10, 1, 0, 0, 0},
		{0x10, 1, 0, 0, 0x80},
		{0x10, 1, 0, 0, 0x80, 0},
		{0x10, 3, 0, 0, 0x80, 0, 0},      // No produced bytes to refer back to.
		{0x10, 4, 0, 0, 0x40, 'A', 0, 1}, // Distance two after one literal.
		{0x10, 0xFF, 0xFF, 0xFF},
	} {
		if _, err := DecompressLZ77(data, 4096); !errors.Is(err, ErrGraphics) {
			t.Errorf("stream %x: %v, want ErrGraphics", data, err)
		}
	}
}
