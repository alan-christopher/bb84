package bb84

import (
	"fmt"
	"math/bits"
	"math/rand"

	"github.com/alan-christopher/bb84/go/bb84/bitarray"
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

func (w winnower) Reconcile(x bitarray.Dense) (reconcileResult, error) {
	var (
		xHat bitarray.Dense = x
		err  error
	)
	for _, hBits := range w.iters {
		xHat, err = w.winnow(xHat, hBits)
		if err != nil {
			return reconcileResult{}, err
		}
	}
	return reconcileResult{xHat: xHat}, nil
}

func (w winnower) winnow(x bitarray.Dense, hBits int) (bitarray.Dense, error) {
	x.Shuffle(w.rand)
	syndromes, err := w.getSyndromes(x, hBits)
	if err != nil {
		return bitarray.Empty(), err
	}
	todo, err := w.exchangeTotalParity(syndromes, hBits)
	if err != nil {
		return bitarray.Empty(), err
	}
	synSums, err := w.exchangeFullSyndromes(syndromes, todo, hBits)
	if err != nil {
		return bitarray.Empty(), err
	}
	w.applySyndromes(&x, synSums, todo, hBits)
	x = w.maintainPrivacy(x, todo, hBits)

	return x, nil
}

func (w winnower) exchangeTotalParity(syndromes []bitarray.Dense, hBits int) (bitarray.Dense, error) {
	tp := bitarray.Empty()
	for _, syn := range syndromes {
		tp.AppendBit(syn.Get(hBits))
	}
	tppb := &bb84pb.ParityAnnouncement{}
	if w.isAlice {
		if err := w.channel.Write(&bb84pb.ParityAnnouncement{Parities: tp.ToProto()}); err != nil {
			return bitarray.Empty(), nil
		}
		if err := w.channel.Read(tppb); err != nil {
			return bitarray.Empty(), nil
		}
	} else {
		if err := w.channel.Read(tppb); err != nil {
			return bitarray.Empty(), nil
		}
		if err := w.channel.Write(&bb84pb.ParityAnnouncement{Parities: tp.ToProto()}); err != nil {
			return bitarray.Empty(), nil
		}
	}
	otherTP := bitarray.DenseFromProto(tppb.Parities)
	if tp.Size() != otherTP.Size() {
		return bitarray.Empty(), fmt.Errorf(
			"reconciling bitstrings of different block counts: %d != %d", tp.Size(), otherTP.Size())
	}

	return tp.XOr(otherTP), nil
}

func (w winnower) exchangeFullSyndromes(syndromes []bitarray.Dense, todo bitarray.Dense, hBits int) ([]bitarray.Dense, error) {
	var filteredSyn []bitarray.Dense
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
		return nil, w.channel.Write(msg)
	}
	synpb := &bb84pb.SyndromeAnnouncement{}
	if err := w.channel.Read(synpb); err != nil {
		return nil, err
	}
	if len(synpb.Syndromes) != len(filteredSyn) {
		return nil, fmt.Errorf(
			"reconciling syndromes of different block counts: %d != %d", len(filteredSyn), len(synpb.Syndromes))
	}
	var r []bitarray.Dense
	for i, syn := range filteredSyn {
		oSyn := bitarray.DenseFromProto(synpb.Syndromes[i])
		r = append(r, syn.XOr(oSyn))
	}

	return r, nil
}

func (w winnower) applySyndromes(x *bitarray.Dense, synSums []bitarray.Dense, todo bitarray.Dense, hBits int) error {
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
		// if idx >= x.Size() {
		// 	continue // trying to flip an illusory padding bit.
		// }
		if err := x.Set(idx, !x.Get(idx)); err != nil {
			return err
		}
	}
	return nil
}

func (w winnower) maintainPrivacy(x bitarray.Dense, todo bitarray.Dense, hBits int) bitarray.Dense {
	keep := bitarray.Empty()
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
	return x.Select(keep)
}

func (w winnower) getSyndromes(x bitarray.Dense, hBits int) ([]bitarray.Dense, error) {
	var r []bitarray.Dense
	bSize := 1 << hBits
	for i := 0; i < x.Size(); i += bSize {
		block, err := x.Slice(i, min(i+bSize, x.Size()))
		if err != nil {
			return nil, err
		}
		if i+bSize > x.Size() {
			block = bitarray.NewDense(block.Data(), bSize)
		}
		syndrome, err := w.secded(block, hBits)
		if err != nil {
			return nil, err
		}
		r = append(r, syndrome)
	}
	return r, nil
}

func (w winnower) secded(block bitarray.Dense, hBits int) (bitarray.Dense, error) {
	if block.Size() != 1<<hBits {
		return bitarray.Empty(), fmt.Errorf(
			"hamming SECDED with %d parity bits needs block of %d, got %d", hBits, 1<<hBits, block.Size())
	}
	r := bitarray.Empty()

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
	r.AppendBit(block.Parity())

	return r, nil
}
