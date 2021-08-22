package bb84

import (
	"fmt"

	"github.com/alan-christopher/bb84/go/bb84/bitmap"
)

// A toeplitz represents a matrix whose diagonals are all constant. It operates
// in F_2, i.e. all of its scalars are 0 or 1.
type toeplitz struct {
	// The diagonal constants for this toeplitz matrix, starting from the bottom
	// left and ending with the top right.
	diags bitmap.Dense

	m int
	n int
}

// TODO: consider implementing a non-toeplitz universal hashing scheme.
//   https://www.cs.princeton.edu/courses/archive/fall09/cos521/Handouts/universalclasses.pdf
//   suggests that there are O(n) schemes.
//   See also https://arxiv.org/abs/1202.4961,
//     - https://eprint.iacr.org/2008/216.pdf
//     - https://arxiv.org/abs/1311.5322
//     - https://ee.stanford.edu/~gray/toeplitz.pdf
// TODO: surely there are ways to take advantage of the structure of a toeplitz
//   matrix to achieve vector mul in better than O(mn) time. Even constant
//   factor improvements are worth investigating; profiling indicate that this
//   is the long pole in the tent when it comes to performance.
// Mul computes the matrix product Av between the toeplitz matrix t and the
// provided vector.
func (t toeplitz) Mul(vec bitmap.Dense) (bitmap.Dense, error) {
	if t.diags.Size() < t.m+t.n-1 {
		return bitmap.Dense{}, fmt.Errorf("improper toeplitz construction, has %d diagonals, needs %d", t.diags.Size(), t.m+t.n-1)
	}
	if t.n != vec.Size() {
		return bitmap.Dense{}, fmt.Errorf("multiplying %dx%d matrix into %d-dim vector", t.m, t.n, vec.Size())
	}

	r := bitmap.Dense{}
	for off := t.m - 1; off >= 0; off-- {
		row, err := bitmap.Slice(t.diags, off, off+t.n)
		if err != nil {
			return bitmap.Empty(), err
		}
		r.AppendBit(bitmap.Parity(bitmap.And(row, vec)))
	}
	return r, nil
}
