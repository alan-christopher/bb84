package bitmap

import (
	"bytes"
	"testing"
)

func mustDense(t *testing.T, s string) Dense {
	d, err := FromString(s)
	if err != nil {
		t.Fatalf("bugged test setup: %v", err)
	}
	return d
}

func TestSelect(t *testing.T) {
	tcs := []struct {
		name string
		data Dense
		mask Dense
		eout Dense
	}{
		{
			name: "all",
			data: mustDense(t, "101"),
			mask: mustDense(t, "111"),
			eout: mustDense(t, "101"),
		}, {
			name: "some",
			data: mustDense(t, "10100011"),
			mask: mustDense(t, "11111100"),
			eout: mustDense(t, "101000"),
		}, {
			name: "none",
			data: mustDense(t, "10100011 111"),
			mask: mustDense(t, "00000000 000"),
			eout: mustDense(t, ""),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			out := Select(tc.data, tc.mask)
			if out.len != tc.eout.len {
				t.Errorf("got bitmap of len %d, want %d", out.len, tc.eout.len)
			}
			if !bytes.Equal(out.bits, tc.eout.bits) {
				t.Errorf("Select(%v, %v) == %v, want %v", tc.data.bits, tc.mask.bits, out.bits, tc.eout.bits)
			}
		})
	}
}

func TestParity(t *testing.T) {
	tcs := []struct {
		name string
		data Dense
		eout bool
	}{
		{"short even", mustDense(t, "101"), false},
		{"short odd", mustDense(t, "111"), true},
		{"empty", mustDense(t, ""), false},
		{"multibyte even", mustDense(t, "1111 1111 11"), false},
		{"multibyte odd", mustDense(t, "1111 1111 10"), true},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			out := Parity(tc.data)
			if out != tc.eout {
				t.Errorf("Parity(%v) == %v, want %v", tc.data.bits, out, tc.eout)
			}
		})
	}
}

func TestCountOnes(t *testing.T) {
	tcs := []struct {
		name string
		data Dense
		eout int
	}{
		{"short", mustDense(t, "101"), 2},
		{"empty", mustDense(t, ""), 0},
		{"multibyte one", mustDense(t, "1111 1111 11"), 10},
		{"multibyte two", mustDense(t, "1011 1011 10"), 7},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			out := CountOnes(tc.data)
			if out != tc.eout {
				t.Errorf("CountOnes(%v) == %v, want %v", tc.data.bits, out, tc.eout)
			}
		})
	}
}
