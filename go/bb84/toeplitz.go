package bb84

import (
	"fmt"

	"github.com/alan-christopher/bb84/go/bb84/bitarray"
)

// A toeplitz represents a matrix whose diagonals are all constant. It operates
// in F_2, i.e. all of its scalars are 0 or 1.
type toeplitz struct {
	// The diagonal constants for this toeplitz matrix, starting from the bottom
	// left and ending with the top right.
	diags bitarray.Dense

	m int
	n int
}

// Mul computes the matrix product Av between the toeplitz matrix t and the
// provided vector.
func (t toeplitz) Mul(vec bitarray.Dense) (bitarray.Dense, error) {
	if t.diags.Size() < t.m+t.n-1 {
		return bitarray.Dense{}, fmt.Errorf("improper toeplitz construction, has %d diagonals, needs %d", t.diags.Size(), t.m+t.n-1)
	}
	if t.n != vec.Size() {
		return bitarray.Dense{}, fmt.Errorf("multiplying %dx%d matrix into %d-dim vector", t.m, t.n, vec.Size())
	}

	r := bitarray.Dense{}
	for off := t.m - 1; off >= 0; off-- {
		row, err := t.diags.Slice(off, off+t.n)
		if err != nil {
			return bitarray.Dense{}, err
		}
		r.AppendBit(row.And(vec).Parity())
	}
	return r, nil
}
