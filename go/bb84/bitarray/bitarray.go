// Package bitarray provides utilities for operating on densely-packed arrays of
// booleans.
package bitarray

import (
	"fmt"
	"math/bits"

	"github.com/alan-christopher/bb84/go/generated/bb84pb"
)

// TODO: this could be more efficient on many architectures if we used larger
//   blocks than 8-bit bytes.
// TODO: Heavy use of copy semantics makes it easy to achieve correctness, but
//   is fairly wasteful. Add support for in-place operations.

// A Dense is a bit array where every bit is explicitly represented.
type Dense struct {
	bits []byte
	len  int

	offset int
}

const blockSize = 8

// NewDense returns a new Dense whose data is a copy of data,
// and whose length is bitLen. If bitLen is longer than data, then
// trailing zeros are added. If bitLen is negative, then it is inferred
// from data.
func NewDense(data []byte, bitLen int) Dense {
	if bitLen < 0 {
		bitLen = len(data) * blockSize
	}
	bits := make([]byte, blocksFor(bitLen))
	copy(bits, data)
	return Dense{
		bits: bits,
		len:  bitLen,
	}
}

// Empty returns an empty, dense bit array.
func Empty() Dense {
	return Dense{}
}

// DenseFromProto converts a DenseBitArray protocol buffer to a Dense.
func DenseFromProto(dba *bb84pb.DenseBitArray) Dense {
	return Dense{
		bits: dba.Bits,
		len:  int(dba.Len),
	}
}

// Size returns the number of bits in d.
func (d Dense) Size() int {
	return d.len
}

// ByteSize returns the number of bytes necessary to represent d.
func (d Dense) ByteSize() int {
	return blocksFor(d.len)
}

// Data returns a copy of the bytes data underlying d.
func (d Dense) Data() []byte {
	data := make([]byte, 0, blocksFor(d.len))
	for i := 0; i < blocksFor(d.len); i++ {
		data = append(data, d.getByte(i))
	}
	return data
}

// And computes a bitwise AND operation between d and other. If one of the two
// is shorter than the other, then trailing 0s are implicitly added to make the
// sizes match.
func (d Dense) And(other Dense) Dense {
	short := other
	if d.len < other.len {
		short = d
	}
	r := Dense{
		bits: make([]byte, 0, blocksFor(short.len)),
		len:  short.len,
	}
	for i := range short.bits {
		r.bits = append(r.bits, d.getByte(i)&other.getByte(i))
	}
	return r
}

// XOr computes a bitwise XOR operation between d and other. If one of the two
// is shorter than the other, then trailing 0s are implicitly added to make the
// sizes match.
func (d Dense) XOr(other Dense) Dense {
	short, long := other, d
	if d.len < other.len {
		short, long = d, other
	}
	r := Dense{
		bits: make([]byte, 0, blocksFor(long.len)),
		len:  long.len,
	}
	for i := range short.bits {
		r.bits = append(r.bits, short.getByte(i)^long.getByte(i))
	}
	for j := len(short.bits); j < len(long.bits); j++ {
		r.bits = append(r.bits, long.getByte(j)) // 0^v == v
	}
	return r
}

// XNor computes a bitwise equality operation between d and other. If one of the
// two is shorter than the other, then trailing 0s are implicitly added to make
// the sizes match.
func (d Dense) XNor(other Dense) Dense {
	short, long := other, d
	if d.len < other.len {
		short, long = d, other
	}
	r := Dense{
		bits: make([]byte, 0, blocksFor(long.len)),
		len:  long.len,
	}
	for i := range short.bits {
		r.bits = append(r.bits, ^short.getByte(i)^long.getByte(i))
	}
	for j := len(short.bits); j < len(long.bits); j++ {
		r.bits = append(r.bits, ^long.getByte(j)) // ~(0^v) == ~v
	}
	return r
}

// Not returns a copy of d whose bits have all been flipped.
func (d Dense) Not() Dense {
	return d.XNor(Dense{})
}

// Parity returns the overall parity of d, with true corresponding to 1 and
// false to 0.
func (d Dense) Parity() bool {
	var sum byte
	for i := 0; i < blocksFor(d.len); i++ {
		sum ^= d.getByte(i)
	}
	return bits.OnesCount8(sum)%2 == 1
}

// CountOnes returns the total number of bits set in d.
func (d Dense) CountOnes() int {
	var sum int
	for i := 0; i < blocksFor(d.len); i++ {
		sum += bits.OnesCount8(d.getByte(i))
	}
	return sum
}

// Select selects a subset of bits from d, according to which bits are set in
// mask.
func (d Dense) Select(mask Dense) Dense {
	var r Dense
	for i := 0; i < d.len; i++ {
		if !mask.Get(i) {
			continue
		}
		r.AppendBit(d.Get(i))
	}
	return r
}

// Slice creates a view into d including bits [start, end).
func (d Dense) Slice(start, end int) (Dense, error) {
	if end-start > d.len {
		return Dense{}, fmt.Errorf("slicing bitarray of len %d up to %d", d.len, end-start)
	}
	if start < 0 {
		return Dense{}, fmt.Errorf("slicing bitarray with negative start: %d", start)
	}
	if end < start {
		return Dense{}, fmt.Errorf("slicing bitarray to negative length: %d", end-start)
	}
	blockStart := start / blockSize
	blockEnd := blockStart + blocksFor(end-start)
	return Dense{
		bits:   d.bits[blockStart:blockEnd],
		len:    end - start,
		offset: start % blockSize,
	}, nil
}

// Get returns the bit at idx.
func (d Dense) Get(idx int) bool {
	if idx >= d.len {
		return false
	}
	idx = idx + d.offset
	block := d.bits[idx/blockSize]
	pos := idx % blockSize
	return 0 < block&(1<<pos)
}

// ToProto converts d into an equivalent DenseBitArray proto.
func (d *Dense) ToProto() *bb84pb.DenseBitArray {
	return &bb84pb.DenseBitArray{
		Bits: d.bits,
		Len:  int32(d.len),
	}
}

// AppendBit adds a single bit to the end of d.
func (d *Dense) AppendBit(bit bool) {
	pos := d.len % blockSize
	d.len += 1
	if pos == 0 {
		d.bits = append(d.bits, 0)
	}
	if bit {
		d.bits[len(d.bits)-1] |= 1 << pos
	}
}

func (d *Dense) getByte(i int) byte {
	lo := d.bits[i] >> d.offset
	var hi byte
	if i+1 < len(d.bits) {
		hi = d.bits[i+1] << (blockSize - d.offset)
	}
	r := lo | hi
	overdraw := (i+1)*blockSize - d.len
	if overdraw < 0 {
		overdraw = 0
	}
	return r << overdraw >> overdraw
}

func blocksFor(bits int) int {
	return (bits + blockSize - 1) / blockSize
}
