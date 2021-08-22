package bb84

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	"github.com/alan-christopher/bb84/go/bb84/bitmap"
)

func TestToeplitzMul(t *testing.T) {
	tcs := []struct {
		mat  toeplitz
		vec  bitmap.Dense
		eout bitmap.Dense
	}{
		{
			// (0 1 0)
			// (0 0 1)
			// (1 0 0)
			mat: toeplitz{
				diags: bitmap.NewDense([]byte{0b01001}, 5),
				m:     3,
				n:     3,
			},
			// (0 1 1)^T
			vec: bitmap.NewDense([]byte{0b110}, 3),
			// (1 1 0)^T
			eout: bitmap.NewDense([]byte{0b011}, 3),
		}, {
			// (0 0)
			// (1 0)
			// (0 1)
			// (1 0)
			mat: toeplitz{
				diags: bitmap.NewDense([]byte{0b00101}, 5),
				m:     4,
				n:     2,
			},
			// (1 0)^T
			vec: bitmap.NewDense([]byte{0b01}, 2),
			// (0 1 0 1)^T
			eout: bitmap.NewDense([]byte{0b1010}, 4),
		}, {
			// (1 1 1 0)
			// (0 1 1 1)
			mat: toeplitz{
				diags: bitmap.NewDense([]byte{0b01110}, 5),
				m:     2,
				n:     4,
			},
			// (0 1 0 1)^T
			vec: bitmap.NewDense([]byte{0b01}, 4),
			// (1 0)^T
			eout: bitmap.NewDense([]byte{0b01}, 2),
		},
	}

	for _, tc := range tcs {
		t.Run(fmt.Sprintf("%dx%d", tc.mat.m, tc.mat.n), func(t *testing.T) {
			out, err := tc.mat.Mul(tc.vec)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.Size() != tc.eout.Size() {
				t.Errorf("got bitmap of len %d, want %d", out.Size(), tc.eout.Size())
			}
			outArr := out.Data()
			eoutArr := tc.eout.Data()
			if !bytes.Equal(outArr, eoutArr) {
				t.Errorf("T*v == %v, want %v", outArr, eoutArr)
			}
		})
	}
}

func TestToeplitzShape(t *testing.T) {
	tcs := []struct {
		name string
		mat  toeplitz
		vec  bitmap.Dense
		eErr bool
	}{
		{
			name: "mismatched dims",
			mat: toeplitz{
				diags: bitmap.NewDense(nil, 5),
				m:     3,
				n:     3,
			},
			vec:  bitmap.NewDense(nil, 2),
			eErr: true,
		}, {
			name: "insufficient diags",
			mat: toeplitz{
				diags: bitmap.NewDense(nil, 2),
				m:     3,
				n:     3,
			},
			vec:  bitmap.NewDense(nil, 3),
			eErr: true,
		}, {
			name: "extra diags",
			mat: toeplitz{
				diags: bitmap.NewDense(nil, 1024),
				m:     3,
				n:     3,
			},
			vec:  bitmap.NewDense(nil, 3),
			eErr: false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.mat.Mul(tc.vec)
			if !tc.eErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.eErr && err == nil {
				t.Errorf("expected error: got nil")
			}
		})
	}
}

func BenchmarkToeplitzMul(b *testing.B) {
	m := 40
	n := 655360
	bd := make([]byte, (m+n)/8+1)
	rand.Read(bd)
	t := toeplitz{
		diags: bitmap.NewDense(bd, m+n),
		m:     m,
		n:     n,
	}
	bx := make([]byte, n/8+1)
	rand.Read(bx)
	x := bitmap.NewDense(bx, n)
	b.ResetTimer()
	if _, err := t.Mul(x); err != nil {
		b.Errorf("fuck: %v", err)
	}
}
