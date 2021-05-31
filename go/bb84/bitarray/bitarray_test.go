package bitarray

import (
	"bytes"
	"testing"
)

func TestAnd(t *testing.T) {
	tcs := []struct {
		name string
		a    Dense
		b    Dense
		eout Dense
	}{
		{
			name: "aligned",
			a:    Dense{bits: []byte{0b101}, len: 8},
			b:    Dense{bits: []byte{0b110}, len: 8},
			eout: Dense{bits: []byte{0b100}, len: 8},
		}, {
			name: "short a",
			a:    Dense{bits: []byte{0b101}, len: 8},
			b:    Dense{bits: []byte{0b110, 0b1}, len: 9},
			eout: Dense{bits: []byte{0b100}, len: 8},
		}, {
			name: "short b",
			a:    Dense{bits: []byte{0b101, 0b1}, len: 9},
			b:    Dense{bits: []byte{0b110}, len: 8},
			eout: Dense{bits: []byte{0b100}, len: 8},
		}, {
			name: "empty a",
			b:    Dense{bits: []byte{0b110}, len: 8},
		}, {
			name: "empty b",
			a:    Dense{bits: []byte{0b110}, len: 8},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			out := tc.a.And(tc.b)
			if out.len != tc.eout.len {
				t.Errorf("got bitarray of len %d, want %d", out.len, tc.eout.len)
			}
			if !bytes.Equal(out.bits, tc.eout.bits) {
				t.Errorf("and(%v, %v) == %v, want %v", tc.a.bits, tc.b.bits, out.bits, tc.eout.bits)
			}
		})
	}
}

func TestXOr(t *testing.T) {
	tcs := []struct {
		name string
		a    Dense
		b    Dense
		eout Dense
	}{
		{
			name: "aligned",
			a:    Dense{bits: []byte{0b101}, len: 8},
			b:    Dense{bits: []byte{0b110}, len: 8},
			eout: Dense{bits: []byte{0b011}, len: 8},
		}, {
			name: "short a",
			a:    Dense{bits: []byte{0b101}, len: 8},
			b:    Dense{bits: []byte{0b110, 0b1}, len: 9},
			eout: Dense{bits: []byte{0b011, 0b1}, len: 9},
		}, {
			name: "short b",
			a:    Dense{bits: []byte{0b101, 0b1}, len: 9},
			b:    Dense{bits: []byte{0b110}, len: 8},
			eout: Dense{bits: []byte{0b011, 0b1}, len: 9},
		}, {
			name: "empty a",
			b:    Dense{bits: []byte{0b110}, len: 8},
			eout: Dense{bits: []byte{0b110}, len: 8},
		}, {
			name: "empty b",
			a:    Dense{bits: []byte{0b110}, len: 8},
			eout: Dense{bits: []byte{0b110}, len: 8},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			out := tc.a.XOr(tc.b)
			if out.len != tc.eout.len {
				t.Errorf("got bitarray of len %d, want %d", out.len, tc.eout.len)
			}
			if !bytes.Equal(out.bits, tc.eout.bits) {
				t.Errorf("xor(%v, %v) == %v, want %v", tc.a.bits, tc.b.bits, out.bits, tc.eout.bits)
			}
		})
	}
}

func TestXNor(t *testing.T) {
	tcs := []struct {
		name string
		a    Dense
		b    Dense
		eout Dense
	}{
		{
			name: "aligned",
			a:    Dense{bits: []byte{0b00000101}, len: 8},
			b:    Dense{bits: []byte{0b00000110}, len: 8},
			eout: Dense{bits: []byte{0b11111100}, len: 8},
		}, {
			name: "short a",
			a:    Dense{bits: []byte{0b00000101}, len: 8},
			b:    Dense{bits: []byte{0b00000110, 0b10}, len: 10},
			eout: Dense{bits: []byte{0b11111100, 0b11111101}, len: 10},
		}, {
			name: "short b",
			a:    Dense{bits: []byte{0b00000110, 0b10}, len: 10},
			b:    Dense{bits: []byte{0b00000101}, len: 8},
			eout: Dense{bits: []byte{0b11111100, 0b11111101}, len: 10},
		}, {
			name: "empty a",
			b:    Dense{bits: []byte{0b00000110}, len: 8},
			eout: Dense{bits: []byte{0b11111001}, len: 8},
		}, {
			name: "empty b",
			a:    Dense{bits: []byte{0b00000110}, len: 8},
			eout: Dense{bits: []byte{0b11111001}, len: 8},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			out := tc.a.XNor(tc.b)
			if out.len != tc.eout.len {
				t.Errorf("got bitarray of len %d, want %d", out.len, tc.eout.len)
			}
			if !bytes.Equal(out.bits, tc.eout.bits) {
				t.Errorf("xnor(%v, %v) == %v, want %v", tc.a.bits, tc.b.bits, out.bits, tc.eout.bits)
			}
		})
	}
}

func TestNot(t *testing.T) {
	tcs := []struct {
		name string
		a    Dense
		eout Dense
	}{
		{
			name: "one byte",
			a:    Dense{bits: []byte{0b00000101}, len: 8},
			eout: Dense{bits: []byte{0b11111010}, len: 8},
		}, {
			name: "multi-bytes",
			a:    Dense{bits: []byte{0b10101101, 0b00000101}, len: 16},
			eout: Dense{bits: []byte{0b01010010, 0b11111010}, len: 16},
		}, {
			name: "unaligned",
			a:    Dense{bits: []byte{0b101}, len: 3},
			eout: Dense{bits: []byte{0b010}, len: 3},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			out := tc.a.Not()
			if out.len != tc.eout.len {
				t.Errorf("got bitarray of len %d, want %d", out.len, tc.eout.len)
			}

			if !bytes.Equal(out.Data(), tc.eout.bits) {
				t.Errorf("not(%v) == %v, want %v", tc.a.bits, out.Data(), tc.eout.bits)
			}
		})
	}
}

func TestSelect(t *testing.T) {
	tcs := []struct {
		name string
		bits Dense
		mask Dense
		eout Dense
	}{
		{
			name: "all",
			bits: Dense{bits: []byte{0b11101101}, len: 8},
			mask: Dense{bits: []byte{0b11111111}, len: 8},
			eout: Dense{bits: []byte{0b11101101}, len: 8},
		}, {
			name: "none",
			bits: Dense{bits: []byte{0b1101101}, len: 8},
		}, {
			name: "some",
			bits: Dense{bits: []byte{0b11101101, 0b0010101}, len: 13},
			mask: Dense{bits: []byte{0b10001011, 0b0101011}, len: 15},
			eout: Dense{bits: []byte{0b0011101}, len: 7},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			out := tc.bits.Select(tc.mask)
			if out.len != tc.eout.len {
				t.Errorf("got bitarray of len %d, want %d", out.len, tc.eout.len)
			}
			if !bytes.Equal(out.bits, tc.eout.bits) {
				t.Errorf("select(%v, %v) == %v, want %v", tc.bits.bits, tc.mask.bits, out.bits, tc.eout.bits)
			}
		})
	}
}

func TestSlice(t *testing.T) {
	tcs := []struct {
		name  string
		start int
		end   int
		bits  Dense
		eout  Dense
	}{
		{
			name:  "full slice",
			bits:  Dense{bits: []byte{0b11101101}, len: 8},
			start: 0,
			end:   8,
			eout:  Dense{bits: []byte{0b11101101}, len: 8},
		}, {
			name: "empty slice",
			bits: Dense{bits: []byte{0b11101101}, len: 8},
		}, {
			name:  "aligned",
			bits:  Dense{bits: []byte{0b1, 0b11101101, 0b1}, len: 24},
			start: 8,
			end:   16,
			eout:  Dense{bits: []byte{0b11101101}, len: 8},
		}, {
			name:  "unaligned start",
			bits:  Dense{bits: []byte{0b10, 0b1, 0b1}, len: 24},
			start: 1,
			end:   16,
			eout:  Dense{bits: []byte{0b10000001, 0}, len: 15},
		}, {
			name:  "unaligned end",
			bits:  Dense{bits: []byte{0b11111111, 0, 0b1}, len: 24},
			start: 8,
			end:   17,
			eout:  Dense{bits: []byte{0, 0b1}, len: 9},
		}, {
			name:  "long slice",
			bits:  Dense{bits: []byte{1, 2, 3, 4, 5, 6}, len: 48},
			start: 8,
			end:   48,
			eout:  Dense{bits: []byte{2, 3, 4, 5, 6}, len: 40},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			sArr, err := tc.bits.Slice(tc.start, tc.end)
			if err != nil {
				t.Fatalf("slice(%d, %d) = %v, want nil error", tc.start, tc.end, err)
			}
			if sArr.len != tc.eout.len {
				t.Errorf("got bitarray of len %d, want %d", sArr.len, tc.eout.len)
			}
			sData := sArr.Data()
			eData := tc.eout.Data()
			if !bytes.Equal(sData, eData) {
				t.Errorf("slice(%v, %d, %d) == %v, want %v", tc.bits.bits, tc.start, tc.end, sData, eData)
			}
		})
	}
}
