package bitmap

import (
	"bytes"
	"testing"
)

func TestBinaryOperators(t *testing.T) {
	tcs := []struct {
		name string
		a, b Dense
		eout Dense
		op   func(a, b Dense) Dense
	}{
		{
			name: "AND aligned",
			a:    mustDense(t, "10100000"),
			b:    mustDense(t, "01100000"),
			eout: mustDense(t, "00100000"),
			op:   And,
		}, {
			name: "AND short a",
			a:    mustDense(t, "101"),
			b:    mustDense(t, "01111000"),
			eout: mustDense(t, "001"),
			op:   And,
		}, {
			name: "AND short b",
			b:    mustDense(t, "01111000"),
			a:    mustDense(t, "101"),
			eout: mustDense(t, "001"),
			op:   And,
		}, {
			name: "AND multibyte",
			b:    mustDense(t, "0111 1000 1011 1011"),
			a:    mustDense(t, "1010 1010 1100 0110"),
			eout: mustDense(t, "0010 1000 1000 0010"),
			op:   And,
		},

		{
			name: "OR aligned",
			a:    mustDense(t, "10100000"),
			b:    mustDense(t, "01100000"),
			eout: mustDense(t, "11100000"),
			op:   Or,
		}, {
			name: "OR short a",
			a:    mustDense(t, "101"),
			b:    mustDense(t, "01111000"),
			eout: mustDense(t, "11111000"),
			op:   Or,
		}, {
			name: "OR short b",
			b:    mustDense(t, "01111000"),
			a:    mustDense(t, "101"),
			eout: mustDense(t, "11111000"),
			op:   Or,
		}, {
			name: "OR multibyte",
			b:    mustDense(t, "0111 1000 1011 1011"),
			a:    mustDense(t, "1010 1010 1100 0110"),
			eout: mustDense(t, "1111 1010 1111 1111"),
			op:   Or,
		},

		{
			name: "XOR aligned",
			a:    mustDense(t, "10100000"),
			b:    mustDense(t, "01100000"),
			eout: mustDense(t, "11000000"),
			op:   XOr,
		}, {
			name: "XOR short a",
			a:    mustDense(t, "101"),
			b:    mustDense(t, "01111000"),
			eout: mustDense(t, "11011000"),
			op:   XOr,
		}, {
			name: "XOR short b",
			b:    mustDense(t, "01111000"),
			a:    mustDense(t, "101"),
			eout: mustDense(t, "11011000"),
			op:   XOr,
		}, {
			name: "XOR multibyte",
			b:    mustDense(t, "0111 1000 1011 1011"),
			a:    mustDense(t, "1010 1010 1100 0110"),
			eout: mustDense(t, "1101 0010 0111 1101"),
			op:   XOr,
		},

		{
			name: "XNOR aligned",
			a:    mustDense(t, "10100000"),
			b:    mustDense(t, "01100000"),
			eout: mustDense(t, "00111111"),
			op:   XNor,
		}, {
			name: "XNOR short a",
			a:    mustDense(t, "101"),
			b:    mustDense(t, "01111000"),
			eout: mustDense(t, "00100111"),
			op:   XNor,
		}, {
			name: "XNOR short b",
			b:    mustDense(t, "01111000"),
			a:    mustDense(t, "101"),
			eout: mustDense(t, "00100111"),
			op:   XNor,
		}, {
			name: "XNOR multibyte",
			b:    mustDense(t, "0111 1000 1011 1011"),
			a:    mustDense(t, "1010 1010 1100 0110"),
			eout: mustDense(t, "0010 1101 1000 0010"),
			op:   XNor,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			out := tc.op(tc.a, tc.b)
			if out.Size() != tc.eout.Size() {
				t.Fatalf("got bitmap of len %d, want %d", out.Size(), tc.eout.Size())
			}
			if !bytes.Equal(out.Data(), tc.eout.Data()) {
				t.Errorf("Data() == %v, want %v", out.Data(), tc.eout.Data())
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
			a:    mustDense(t, "10100000"),
			eout: mustDense(t, "01011111"),
		}, {
			name: "multi-bytes",
			a:    mustDense(t, "1010 1101 0000 0101"),
			eout: mustDense(t, "0101 0010 1111 1010"),
		}, {
			name: "unaligned",
			a:    mustDense(t, "101"),
			eout: Dense{[]byte{0b11111010}, 3, true},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			out := Not(tc.a)
			if out.Size() != tc.eout.Size() {
				t.Fatalf("got bitmap of len %d, want %d", out.Size(), tc.eout.Size())
			}
			if !bytes.Equal(out.Data(), tc.eout.Data()) {
				t.Errorf("Data() == %v, want %v", out.Data(), tc.eout.Data())
			}
		})
	}
}

func TestImplicitValPropagation(t *testing.T) {
	tcs := []struct {
		name string
		d    Dense
		eout bool
	}{
		{"plain dense", Dense{}, false},
		{"not", Not(Dense{}), true},
		{"not not", Not(Not(Dense{})), false},
		{"xor", XOr(Dense{}, Dense{}), false},
		{"xnor", XNor(Dense{}, Dense{}), true},
		{"xor not", XOr(Not(Dense{}), Dense{}), true},
		{"xor double not", XOr(Not(Dense{}), Not(Dense{})), false},
		{"and not", And(Dense{}, Not(Dense{})), false},
		{"and double not", And(Not(Dense{}), Not(Dense{})), true},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.d.Get(100000)
			if got != tc.eout {
				t.Fatalf("got implicit val of %v, want %v", got, tc.eout)
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
			bits:  mustDense(t, "11101101"),
			start: 0,
			end:   8,
			eout:  mustDense(t, "11101101"),
		}, {
			name: "empty slice",
			bits: mustDense(t, "11101101"),
			eout: mustDense(t, ""),
		},
		{
			name:  "aligned",
			bits:  mustDense(t, "10000010 11101101 01000001"),
			start: 8,
			end:   16,
			eout:  mustDense(t, "11101101"),
		},
		{
			name:  "unaligned start",
			bits:  mustDense(t, "10000010 11101101 01000001"),
			start: 1,
			end:   16,
			eout:  mustDense(t, "0000010 11101101"),
		}, {
			name:  "unaligned end",
			bits:  mustDense(t, "11111111 00000000 1000 0000"),
			start: 8,
			end:   17,
			eout:  mustDense(t, "00000000 1"),
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
			out, err := Slice(tc.bits, tc.start, tc.end)
			if err != nil {
				t.Fatalf("slice(%d, %d) = %v, want nil error", tc.start, tc.end, err)
			}
			if out.Size() != tc.eout.Size() {
				t.Errorf("got bitmap of len %d, want %d", out.Size(), tc.eout.Size())
			}
			if !bytes.Equal(out.Data(), tc.eout.Data()) {
				t.Errorf("Data() == %v, want %v", out.Data(), tc.eout.Data())
			}
		})
	}
}
