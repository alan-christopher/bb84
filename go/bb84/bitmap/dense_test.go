package bitmap

import (
	"bytes"
	"reflect"
	"testing"
)

func TestDenseGet(t *testing.T) {
	tcs := []struct {
		name  string
		data  Dense
		edata []bool
	}{
		{"implicit zeros", Dense{len: 3}, []bool{false, false, false}},
		{"aligned", mustDense(t, "10101010"), []bool{true, false, true, false, true, false, true, false}},
		{"multibyte",
			mustDense(t, "00000000 101"),
			[]bool{false, false, false, false, false, false, false, false, true, false, true}},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			var d []bool
			for i := 0; i < tc.data.Size(); i++ {
				d = append(d, tc.data.Get(i))
			}
			if !reflect.DeepEqual(d, tc.edata) {
				t.Errorf("t.Get() == %v, want %v", d, tc.edata)
			}
		})
	}
}

func TestDenseAppend(t *testing.T) {
	tcs := []struct {
		name string
		a, b Dense
		eout Dense
	}{
		{
			name: "no alloc",
			a:    mustDense(t, "101"),
			b:    mustDense(t, "111"),
			eout: mustDense(t, "101111"),
		}, {
			name: "aligned",
			a:    mustDense(t, "10101010"),
			b:    mustDense(t, "01010101"),
			eout: mustDense(t, "10101010 01010101"),
		}, {
			name: "unaligned",
			a:    mustDense(t, "10101010 01"),
			b:    mustDense(t, "01010101"),
			eout: mustDense(t, "10101010 01 01010101"),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tc.a.Append(tc.b)
			if tc.a.len != tc.eout.len {
				t.Errorf("got bitarray of len %d, want %d", tc.a.len, tc.eout.len)
			}
			if !bytes.Equal(tc.a.bits, tc.eout.bits) {
				t.Errorf("got %v, want %v", tc.a.bits, tc.eout.bits)
			}
		})
	}
}

func TestDenseSwap(t *testing.T) {
	tcs := []struct {
		name string
		d    Dense
		i, j int
		eout Dense
	}{
		{"zeros", mustDense(t, "00"), 0, 1, mustDense(t, "00")},
		{"ones", mustDense(t, "11"), 0, 1, mustDense(t, "11")},
		{"one zero", mustDense(t, "10"), 0, 1, mustDense(t, "01")},
		{"zero one", mustDense(t, "01"), 0, 1, mustDense(t, "10")},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tc.d.swap(tc.i, tc.j)
			if tc.d.len != tc.eout.len {
				t.Errorf("got bitarray of len %d, want %d", tc.d.len, tc.eout.len)
			}
			if !bytes.Equal(tc.d.bits, tc.eout.bits) {
				t.Errorf("got %v, want %v", tc.d.bits, tc.eout.bits)
			}
		})
	}
}

func TestAppendImplicitOnes(t *testing.T) {
	d := Dense{negated: true}
	d.Append(mustDense(t, "10"))
	want := mustDense(t, "10111111")
	if !bytes.Equal(d.Data(), want.Data()) {
		t.Fatalf("want %b, got %b", want.Data(), d.Data())
	}
	d.Append(mustDense(t, "0000000"))
	want = mustDense(t, "10000000 01111111")
	if !bytes.Equal(d.Data(), want.Data()) {
		t.Fatalf("want %b, got %b", want.Data(), d.Data())
	}
}

func TestAppendImplicitZeros(t *testing.T) {
	d := Dense{}
	d.Append(mustDense(t, "10"))
	want := mustDense(t, "10")
	if !bytes.Equal(d.Data(), want.Data()) {
		t.Fatalf("want %b, got %b", want.Data(), d.Data())
	}
	d.Append(mustDense(t, "11010100 1"))
	want = mustDense(t, "10 11010100 1")
	if !bytes.Equal(d.Data(), want.Data()) {
		t.Fatalf("want %b, got %b", want.Data(), d.Data())
	}
}
