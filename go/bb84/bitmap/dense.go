package bitmap

import (
	"math/rand"

	"github.com/alan-christopher/bb84/go/generated/bb84pb"
)

// A Dense is a bitmap where every bit is explicitly represented.
type Dense struct {
	bits []byte
	len  int

	negated bool
}

// NewDense returns a new dense bitmap whose contents are a view of data, and
// whose length is bitLen. If bitLen is longer than data, then trailing zeros
// are added. If bitLen is negative, then it is inferred from data.
func NewDense(data []byte, bitLen int) Dense {
	if bitLen < 0 {
		bitLen = len(data) * byteSize
	}
	r := Dense{
		bits: data,
		len:  bitLen,
	}
	r.allocSpace()
	return r
}

// Get returns the i-th bit in this bitmap.
func (d Dense) Get(i int) bool {
	if i >= d.len {
		return d.negated
	}
	j, pos := i/byteSize, i%byteSize
	if j >= len(d.bits) {
		return d.negated
	}
	block := d.bits[j]
	return 0 < block&(1<<pos)
}

// Size returns the number of bits in this bitmap, excluding implicit trailing
// zeros.
func (d Dense) Size() int {
	return d.len
}

// SizeBytes returns the number of bytes in this bitmap, excluding implicit
// trailing zeros.
func (d Dense) SizeBytes() int {
	return BytesFor(d.len)
}

// Data returns a view of the bytes underlying this bitmap. Modifying the
// returned slice modifies this bitmap.
func (d Dense) Data() []byte {
	return d.bits
}

// Shuffle randomly permutes the contents of d, using r as a source of
// randomness.
func (d *Dense) Shuffle(r *rand.Rand) {
	r.Shuffle(d.len, d.swap)
}

func (d *Dense) swap(i, j int) {
	a, b := d.Get(i), d.Get(j)
	if a == b {
		return
	}
	d.Flip(i)
	d.Flip(j)
}

func (d *Dense) Flip(i int) {
	j, pos := i/byteSize, i%byteSize
	d.bits[j] ^= 1 << pos
}

func (d *Dense) allocSpace() {
	var defVal byte
	if d.negated {
		defVal = 0xFF
	}
	for len(d.bits) < d.SizeBytes() {
		d.bits = append(d.bits, defVal)
	}
}

// ToProto converts d into an equivalent DenseBitArray proto.
func (d *Dense) ToProto() *bb84pb.DenseBitArray {
	return &bb84pb.DenseBitArray{
		Bits: d.Data(),
		Len:  int32(d.len),
	}
}

// AppendBit adds a single bit to the end of d.
func (d *Dense) AppendBit(bit bool) {
	i, pos := d.len/byteSize, d.len%byteSize
	d.len += 1
	if pos == 0 {
		d.bits = append(d.bits, 0)
	}
	if bit {
		d.bits[i] |= 1 << pos
	} else {
		d.bits[i] &= ^(1 << pos)
	}
}

// Append adds the contents of m to the end of d.
func (d *Dense) Append(d2 Dense) {
	off := d.len % byteSize
	if off == 0 {
		d.bits = append(d.bits, d2.bits...)
		d.len += d2.len
		d.fixLastByte()
		return
	}
	needed := BytesFor(d.len+d2.len) - d.SizeBytes()
	if needed > 0 {
		if d.negated {
			d.bits = append(d.bits, ones(needed)...)
		} else {
			d.bits = append(d.bits, zeros(needed)...)
		}
	}
	dj := d.len / byteSize
	d2j := 0
	for ; dj+1 < len(d.bits); dj++ {
		b := d2.bits[d2j]
		if d.negated {
			d.bits[dj] &= (b << off) | (0xFF >> (byteSize - off))
			d.bits[dj+1] &= (b >> (byteSize - off)) | (0xFF << off)
		} else {
			d.bits[dj] |= b << off
			d.bits[dj+1] |= b >> (byteSize - off)
		}
		d2j++
	}
	if d2j < d2.SizeBytes() {
		b := d2.bits[d2j]
		if d.negated {
			d.bits[dj] &= (b << off) | (0xFF >> (byteSize - off))
		} else {
			d.bits[dj] |= b << off
		}
	}
	d.len += d2.len
	d.fixLastByte()
}

func (d *Dense) fixLastByte() {
	if !d.negated {
		return
	}
	j, off := d.len/byteSize, d.len%byteSize
	if off == 0 {
		return
	}
	d.bits[j] |= 0xFF << off
}

func zeros(k int) []byte {
	return make([]byte, k)
}

func ones(k int) []byte {
	r := make([]byte, k)
	for i := range r {
		r[i] = 0xFF
	}
	return r
}
