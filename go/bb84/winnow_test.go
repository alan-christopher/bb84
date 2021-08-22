package bb84

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/alan-christopher/bb84/go/bb84/bitmap"
)

func TestSECDED(t *testing.T) {
	var w winnower
	tcs := []struct {
		name     string
		vec      bitmap.Dense
		hBits    int
		syndrome bitmap.Dense
	}{{
		name:     "[8,4] null syndrome",
		vec:      bitmap.NewDense([]byte{0b00101101}, 8),
		hBits:    3,
		syndrome: bitmap.NewDense([]byte{0b0000}, 4),
	}, {
		name:     "[8,4] total parity flip",
		vec:      bitmap.NewDense([]byte{0b10101101}, 8),
		hBits:    3,
		syndrome: bitmap.NewDense([]byte{0b1000}, 4),
	}, {
		name:     "[8,4] p1 flip",
		vec:      bitmap.NewDense([]byte{0b00101100}, 8),
		hBits:    3,
		syndrome: bitmap.NewDense([]byte{0b1001}, 4),
	}, {
		name:     "[8,4] p2 flip",
		vec:      bitmap.NewDense([]byte{0b00101111}, 8),
		hBits:    3,
		syndrome: bitmap.NewDense([]byte{0b1010}, 4),
	}, {
		name:     "[8,4] p3 flip",
		vec:      bitmap.NewDense([]byte{0b00100101}, 8),
		hBits:    3,
		syndrome: bitmap.NewDense([]byte{0b1100}, 4),
	}, {
		name:     "[8,4] single data flip",
		vec:      bitmap.NewDense([]byte{0b00101001}, 8),
		hBits:    3,
		syndrome: bitmap.NewDense([]byte{0b1011}, 4),
	}, {
		name:     "[8,4] double flip",
		vec:      bitmap.NewDense([]byte{0b00001100}, 8),
		hBits:    3,
		syndrome: bitmap.NewDense([]byte{0b0111}, 4),
	}, {
		name: "[16,5] null syndrome",
		// little-endian (data, hamming-ed): (01101011100, 00001100 10111000)
		vec:      bitmap.NewDense([]byte{0b00110000, 0b00011101}, 16),
		hBits:    4,
		syndrome: bitmap.NewDense([]byte{0b00000}, 5),
	},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			syn, err := w.secded(tc.vec, tc.hBits)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if syn.Size() != tc.syndrome.Size() {
				t.Errorf("got bitmap of len %d, want %d", syn.Size(), tc.syndrome.Size())
			}
			arr := syn.Data()
			eArr := tc.syndrome.Data()
			if !bytes.Equal(arr, eArr) {
				t.Errorf("hamming(%b) == %b, want %b", tc.vec.Data(), arr, eArr)
			}
		})
	}
}

func TestApplySyndromes(t *testing.T) {
	w := winnower{isAlice: false}
	const hBits = 3

	tcs := []struct {
		name     string
		x        bitmap.Dense
		expected bitmap.Dense
		synSums  []bitmap.Dense
		todo     bitmap.Dense
	}{{
		name:     "skip all",
		x:        bitmap.NewDense(nil, 3*8),
		expected: bitmap.NewDense(nil, 3*8),
		synSums:  []bitmap.Dense{},
		todo:     bitmap.NewDense([]byte{0b000}, 3),
	}, {
		name: "fix all",
		x:    bitmap.NewDense(nil, 3*8),
		expected: bitmap.NewDense([]byte{
			1,
			1 << (0b110 - 1),
			1 << 7}, 24),
		synSums: []bitmap.Dense{
			bitmap.NewDense([]byte{0b1001}, hBits+1),
			bitmap.NewDense([]byte{0b1110}, hBits+1),
			bitmap.NewDense([]byte{0b1000}, hBits+1),
		},
		todo: bitmap.NewDense([]byte{0b111}, 3),
	},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := w.applySyndromes(&tc.x, tc.synSums, tc.todo, hBits)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			arr, eArr := tc.x.Data(), tc.expected.Data()
			if !bytes.Equal(arr, eArr) {
				t.Errorf("x == %08b after correction, want %08b", arr, eArr)
			}
		})
	}
}

func TestPrivacyMaintenance(t *testing.T) {
	var w winnower
	tcs := []struct {
		hBits    int
		x        bitmap.Dense
		xTrimmed bitmap.Dense
		todo     bitmap.Dense
	}{{
		hBits:    2,
		x:        bitmap.NewDense([]byte{0b01111011}, 8),
		xTrimmed: bitmap.NewDense([]byte{0b1110}, 4),
		todo:     bitmap.NewDense([]byte{0b01}, 2),
	}, {
		hBits:    3,
		x:        bitmap.NewDense([]byte{0b10001011, 0b01111111}, 16),
		xTrimmed: bitmap.NewDense([]byte{0b11110000, 0b111}, 11),
		todo:     bitmap.NewDense([]byte{0b01}, 2),
	}, {
		hBits: 4,
		x: bitmap.NewDense([]byte{
			0b10001011, 0b10000000,
			0b11111111, 0b01111111,
		}, 32),
		xTrimmed: bitmap.NewDense([]byte{
			0b00000000, 0b11111000,
			0b11111111, 0b11}, 26),
		todo: bitmap.NewDense([]byte{0b01}, 2),
	},
	}

	for _, tc := range tcs {
		t.Run(fmt.Sprintf("m=%d", tc.hBits), func(t *testing.T) {
			x := w.maintainPrivacy(tc.x, tc.todo, tc.hBits)
			if x.Size() != tc.xTrimmed.Size() {
				t.Errorf("got bitmap of len %d, want %d", x.Size(), tc.xTrimmed.Size())
			}
			arr, eArr := x.Data(), tc.xTrimmed.Data()
			if !bytes.Equal(arr, eArr) {
				t.Errorf("x == %08b after privacy maintenance, want %08b", arr, eArr)
			}
		})
	}
}
