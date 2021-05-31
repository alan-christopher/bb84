// Package bitarray provides utilities for operating on densely-packed arrays of
// booleans.
package bitarray

// TODO: good god you need some comments in here boy.

import (
	"fmt"
	"math/bits"

	"github.com/alan-christopher/bb84/go/generated/bb84pb"
)

type Dense struct {
	bits []byte
	len  int

	offset int
}

const blockSize = 8

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

func DenseFromProto(dba *bb84pb.DenseBitArray) Dense {
	return Dense{
		bits: dba.Bits,
		len:  int(dba.Len),
	}
}

func (d Dense) Size() int {
	return d.len
}

func (d Dense) ByteSize() int {
	return blocksFor(d.len)
}

func (d Dense) Data() []byte {
	data := make([]byte, 0, blocksFor(d.len))
	for i := 0; i < blocksFor(d.len); i++ {
		data = append(data, d.getByte(i))
	}
	return data
}

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

func (d Dense) Not() Dense {
	return d.XNor(Dense{})
}

func (d Dense) Parity() bool {
	var sum byte
	for i := 0; i < blocksFor(d.len); i++ {
		sum ^= d.getByte(i)
	}
	return bits.OnesCount8(sum)%2 == 1
}

func (d Dense) CountOnes() int {
	var sum int
	for i := 0; i < blocksFor(d.len); i++ {
		sum += bits.OnesCount8(d.getByte(i))
	}
	return sum
}

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

func (d Dense) Get(idx int) bool {
	if idx >= d.len {
		return false
	}
	idx = idx + d.offset
	block := d.bits[idx/blockSize]
	pos := idx % blockSize
	return 0 < block&(1<<pos)
}

func (d *Dense) ToProto() *bb84pb.DenseBitArray {
	return &bb84pb.DenseBitArray{
		Bits: d.bits,
		Len:  int32(d.len),
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

func blocksFor(bits int) int {
	return (bits + blockSize - 1) / blockSize
}
