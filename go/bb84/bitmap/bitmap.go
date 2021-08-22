// Package bitmap provides utilities for operating on densely-packed arrays of
// booleans.
package bitmap

import (
	"fmt"
	"math/bits"

	"github.com/alan-christopher/bb84/go/generated/bb84pb"
)

// TODO: this could be more efficient on many architectures if we used larger
//   blocks than 8-bit bytes.
const byteSize = 8

// Select selects a subset of bits from data, according to which bits are set in
// mask.
func Select(data, mask Dense) Dense {
	var d Dense
	for i := 0; i < data.Size(); i++ {
		if !mask.Get(i) {
			continue
		}
		d.AppendBit(data.Get(i))
	}
	return d
}

// Empty returns an empty, dense bit array.
func Empty() Dense {
	return Dense{}
}

// FromProto converts a DenseBitArray protocol buffer to a dense Map.
func DenseFromProto(dba *bb84pb.DenseBitArray) Dense {
	return NewDense(dba.Bits, int(dba.Len))
}

// FromString converts a string of '1's and '0's to a DenseBitArray.
func FromString(s string) (Dense, error) {
	d := Dense{}
	for _, c := range s {
		switch c {
		case '1':
			d.AppendBit(true)
		case '0':
			d.AppendBit(false)
		case ' ':
			continue
		default:
			return Dense{}, fmt.Errorf("invalid bitmap string rep: %s", s)
		}
	}
	return d, nil
}

// Dot treats compute the inner product (x^T * y) of x and y, treating them as
// vectors mod 2.
func Dot(x, y Dense) bool {
	var sum byte
	sb := x.SizeBytes()
	for i := 0; i < sb; i++ {
		sum ^= x.bits[i] & y.bits[i]
	}
	return bits.OnesCount8(sum)%2 == 1
}

// Parity returns the overall parity of m, with true corresponding to 1 and
// false to 0.
func Parity(d Dense) bool {
	var sum byte
	for _, b := range d.bits {
		sum ^= b
	}
	return bits.OnesCount8(sum)%2 == 1
}

// CountOnes returns the total number of bits set in d.
func CountOnes(d Dense) int {
	var sum int
	for _, b := range d.bits {
		sum += bits.OnesCount8(b)
	}
	return sum
}

// Equal returns true iff a and b contain the same bits.
func Equal(a, b Dense) bool {
	return CountOnes(XOr(a, b)) == 0
}

// BytesFor returns the number of bytes necessary to hold the provided number of
// bits.
func BytesFor(bits int) int {
	return (bits + 8 - 1) / 8
}
