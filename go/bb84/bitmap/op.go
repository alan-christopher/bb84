package bitmap

import "fmt"

// And returns the bitwise AND of two bitmaps.
func And(a, b Dense) Dense {
	short, long := a, b
	if b.len < a.len {
		short, long = b, a
	}
	rLen := short.len
	if short.negated {
		rLen = long.len
	}
	r := Dense{
		bits:    make([]byte, 0, BytesFor(rLen)),
		len:     rLen,
		negated: a.negated && b.negated,
	}
	for i := range short.bits {
		r.bits = append(r.bits, a.bits[i]&b.bits[i])
	}
	if short.negated {
		for i := len(short.bits); i < len(long.bits); i++ {
			r.bits = append(r.bits, long.bits[i])
		}
	}
	return r
}

// Or returns the bitwise OR of two bitmaps.
func Or(a, b Dense) Dense {
	short, long := a, b
	if b.len < a.len {
		short, long = b, a
	}
	rLen := long.len
	if short.negated {
		rLen = short.len
	}
	r := Dense{
		bits:    make([]byte, 0, BytesFor(rLen)),
		len:     rLen,
		negated: a.negated || b.negated,
	}
	for i := range short.bits {
		r.bits = append(r.bits, a.bits[i]|b.bits[i])
	}
	if !short.negated {
		for i := len(short.bits); i < len(long.bits); i++ {
			r.bits = append(r.bits, long.bits[i])
		}
	}
	return r
}

// XOr returns the bitwise XOR of two bitmaps.
func XOr(a, b Dense) Dense {
	short, long := a, b
	if b.len < a.len {
		short, long = b, a
	}
	r := Dense{
		bits:    make([]byte, 0, BytesFor(long.len)),
		len:     long.len,
		negated: a.negated != b.negated,
	}
	for i := range short.bits {
		r.bits = append(r.bits, a.bits[i]^b.bits[i])
	}
	var trail byte
	if a.negated {
		trail = 0xFF
	}
	for i := len(short.bits); i < len(long.bits); i++ {
		r.bits = append(r.bits, trail^long.bits[i])
	}
	return r
}

// XNOr returns the bitwise XNOR of two bitmaps.
func XNor(a, b Dense) Dense {
	short, long := a, b
	if b.len < a.len {
		short, long = b, a
	}
	r := Dense{
		bits:    make([]byte, 0, BytesFor(long.len)),
		len:     long.len,
		negated: a.negated == b.negated,
	}
	for i := range short.bits {
		r.bits = append(r.bits, ^(a.bits[i] ^ b.bits[i]))
	}
	var trail byte
	if a.negated {
		trail = 0xFF
	}
	for i := len(short.bits); i < len(long.bits); i++ {
		r.bits = append(r.bits, ^(trail ^ long.bits[i]))
	}
	return r
}

// Not returns the bitwise negation of a bitmap.
func Not(d Dense) Dense {
	r := Dense{
		bits:    make([]byte, 0, BytesFor(d.len)),
		len:     d.len,
		negated: !d.negated,
	}
	for i := range d.bits {
		r.bits = append(r.bits, ^d.bits[i])
	}
	return r
}

// Slice creates a view into m including bits [start, end).
func Slice(d Dense, start, end int) (Dense, error) {
	if end-start > d.len {
		return Dense{}, fmt.Errorf("slicing bitmap of len %d up to %d", d.len, end-start)
	}
	if start < 0 {
		return Dense{}, fmt.Errorf("slicing bitmap with negative start: %d", start)
	}
	if end < start {
		return Dense{}, fmt.Errorf("slicing bitmap to negative length: %d", end-start)
	}

	r := Dense{}
	for ; start%byteSize != 0; start++ {
		r.AppendBit(d.Get(start))
	}
	j := start / byteSize
	tmp := NewDense(d.bits[j:j+BytesFor(end-start)], end-start)
	r.Append(tmp)
	return r, nil
}
