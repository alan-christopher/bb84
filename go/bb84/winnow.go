package bb84

import (
	"fmt"
	"math/bits"
	"math/rand"

	"github.com/alan-christopher/bb84/go/bb84/bitmap"
	"github.com/alan-christopher/bb84/go/generated/bb84pb"
)

// TODO: we currently do a great deal of bit-by-bit operation within this
//   file. Much of it could be optimized to operate on bytes.

// A winnower implements the reconciler interface via the Winnow algorithm, as
// described in https://arxiv.org/abs/quant-ph/0203096.
type winnower struct {
	channel *protoFramer
	rand    *rand.Rand

	// TODO: infer the proper sequence of winnows according to an Epsilon
	//   parameter and the initial parameter estimation.
	iters   []int
	isAlice bool
}

func (w winnower) Reconcile(x bitmap.Dense, s *Stats) (reconcileResult, error) {
	var (
		xHat bitmap.Dense = x
		err  error
	)
	for _, hBits := range w.iters {
		xHat, err = w.winnow(xHat, hBits, s)
		if err != nil {
			return reconcileResult{}, err
		}
	}
	return reconcileResult{xHat: xHat}, nil
}

func (w winnower) winnow(x bitmap.Dense, hBits int, s *Stats) (bitmap.Dense, error) {
	x.Shuffle(w.rand)
	syndromes, err := w.getSyndromes(x, hBits)
	if err != nil {
		return bitmap.Empty(), err
	}
	todo, err := w.exchangeTotalParity(syndromes, hBits, s)
	if err != nil {
		return bitmap.Empty(), err
	}
	synSums, err := w.exchangeFullSyndromes(syndromes, todo, hBits, s)
	if err != nil {
		return bitmap.Empty(), err
	}
	w.applySyndromes(&x, synSums, todo, hBits)
	x = w.maintainPrivacy(x, todo, hBits)

	return x, nil
}

func (w winnower) exchangeTotalParity(syndromes []bitmap.Dense, hBits int, s *Stats) (bitmap.Dense, error) {
	tp := bitmap.Empty()
	for _, syn := range syndromes {
		tp.AppendBit(syn.Get(hBits))
	}
	tppb := &bb84pb.ParityAnnouncement{}
	// TODO: alice should be able to provide her total parity information in her
	//   full syndromes announcement, which reduces the number of messages she
	//   needs to send considerably.
	if w.isAlice {
		if err := w.channel.Write(&bb84pb.ParityAnnouncement{Parities: tp.ToProto()}, s); err != nil {
			return bitmap.Empty(), nil
		}
		if err := w.channel.Read(tppb, s); err != nil {
			return bitmap.Empty(), nil
		}
	} else {
		if err := w.channel.Read(tppb, s); err != nil {
			return bitmap.Empty(), nil
		}
		if err := w.channel.Write(&bb84pb.ParityAnnouncement{Parities: tp.ToProto()}, s); err != nil {
			return bitmap.Empty(), nil
		}
	}
	otherTP := bitmap.DenseFromProto(tppb.Parities)
	if tp.Size() != otherTP.Size() {
		return bitmap.Empty(), fmt.Errorf(
			"reconciling bitstrings of different block counts: %d != %d", tp.Size(), otherTP.Size())
	}

	return bitmap.XOr(tp, otherTP), nil
}

func (w winnower) exchangeFullSyndromes(
	syndromes []bitmap.Dense, todo bitmap.Dense, hBits int, s *Stats) ([]bitmap.Dense, error) {
	var filteredSyn []bitmap.Dense
	for i, syn := range syndromes {
		if todo.Get(i) {
			filteredSyn = append(filteredSyn, syn)
		}
	}
	// Alice announces, Bob fixes.
	if w.isAlice {
		msg := &bb84pb.SyndromeAnnouncement{}
		for _, syn := range filteredSyn {
			msg.Syndromes = append(msg.Syndromes, syn.ToProto())
		}
		return nil, w.channel.Write(msg, s)
	}
	synpb := &bb84pb.SyndromeAnnouncement{}
	if err := w.channel.Read(synpb, s); err != nil {
		return nil, err
	}
	if len(synpb.Syndromes) != len(filteredSyn) {
		return nil, fmt.Errorf(
			"reconciling syndromes of different block counts: %d != %d", len(filteredSyn), len(synpb.Syndromes))
	}
	var r []bitmap.Dense
	for i, syn := range filteredSyn {
		oSyn := bitmap.DenseFromProto(synpb.Syndromes[i])
		r = append(r, bitmap.XOr(syn, oSyn))
	}

	return r, nil
}

func (w winnower) applySyndromes(x *bitmap.Dense, synSums []bitmap.Dense, todo bitmap.Dense, hBits int) error {
	if w.isAlice {
		// Alice announces, Bob fixes.
		return nil
	}
	n := 1 << hBits
	for i, k := 0, -1; i < todo.Size(); i++ {
		if !todo.Get(i) {
			continue
		}
		k++
		syn := synSums[k]
		pos := 0
		for j := 0; j < hBits; j++ {
			if syn.Get(j) {
				pos |= 1 << j
			}
		}
		pos-- // cardinal/ordinal correction
		if pos < 0 {
			pos = n - 1 // total parity flip
		}
		idx := i*n + pos
		x.Flip(idx)
	}
	return nil
}

func (w winnower) maintainPrivacy(x bitmap.Dense, todo bitmap.Dense, hBits int) bitmap.Dense {
	keep := bitmap.Empty()
	n := 1 << hBits
	for i := 0; i < todo.Size(); i++ {
		if !todo.Get(i) {
			for j := 0; j < n-1; j++ {
				keep.AppendBit(true)
			}
			keep.AppendBit(false)
			continue
		}

		for j := 0; j < n; j++ {
			keep.AppendBit(bits.OnesCount(uint(j+1)) != 1)
		}
	}
	return bitmap.Select(x, keep)
}

func (w winnower) getSyndromes(x bitmap.Dense, hBits int) ([]bitmap.Dense, error) {
	var r []bitmap.Dense
	bSize := 1 << hBits
	for i := 0; i < x.Size(); i += bSize {
		block, err := bitmap.Slice(x, i, min(i+bSize, x.Size()))
		if err != nil {
			return nil, err
		}
		if i+bSize > x.Size() {
			block = bitmap.NewDense(block.Data(), bSize)
		}
		syndrome, err := w.secded(block, hBits)
		if err != nil {
			return nil, err
		}
		r = append(r, syndrome)
	}
	return r, nil
}

func (w winnower) secded(block bitmap.Dense, hBits int) (bitmap.Dense, error) {
	if block.Size() != 1<<hBits {
		return bitmap.Empty(), fmt.Errorf(
			"hamming SECDED with %d parity bits needs block of %d, got %d", hBits, 1<<hBits, block.Size())
	}
	r := bitmap.Empty()

	// The p-th hamming parity bit checks the parity of bits in strides of 2^p. E.g.
	// the 0th bit checks positions {0, 2, 4, ...}, the 1st checks
	// {1,2, 5,6, ...}, the 2nd {3,4,5,6, 11,12,13,14, ...}.
	for p := 0; p < hBits; p++ {
		stride := 1 << p
		parity := false
		for i := stride - 1; i < block.Size(); i += 2 * stride {
			for j := i; j < i+stride && j < block.Size(); j++ {
				parity = (block.Get(j) != parity)
			}
		}
		r.AppendBit(parity)
	}

	// Finish by inserting a total parity bit.
	r.AppendBit(bitmap.Parity(block))

	return r, nil
}
