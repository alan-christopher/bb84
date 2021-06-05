package bb84

import (
	"fmt"

	"github.com/alan-christopher/bb84/go/bb84/bitarray"
	"github.com/alan-christopher/bb84/go/generated/bb84pb"
)

// NegotiateKey implements the Peer interface.
func (a *alice) NegotiateKey(rawByteCount int) ([]byte, float64, error) {
	bits, bases, err := a.sendQBits(rawByteCount)
	if err != nil {
		return nil, 0, err
	}
	sifted, err := a.sift(bits, bases)
	if err != nil {
		return nil, 0, err
	}
	unleaked, qber, err := a.estimateQBER(sifted)
	if err != nil {
		return nil, 0, err
	}
	recRes, err := a.reconciler.Reconcile(unleaked)
	if err != nil {
		return nil, 0, err
	}
	// TODO: calculate possible bit leakage from sifting phase
	// TODO: extract key
	//   see https://link.springer.com/article/10.1007/BF00191318 for the number
	//   of bits to eat.
	// TODO: if unleaked isn't byte-aligned then this returns a key with a few
	//   predictable zeros on the end. We can either fix that by trimming the
	//   last, unaligned byte, or by using bitarray.Dense as part of the public
	//   interface.
	return recRes.xHat.Data(), qber, nil
}

// NegotiateKey implements the Peer interface.
func (b *bob) NegotiateKey(rawByteCount int) ([]byte, float64, error) {
	bits, bases, dropped, err := b.receiveQBits(rawByteCount)
	if err != nil {
		return nil, 0, err
	}
	sifted, err := b.sift(bits, bases, dropped)
	if err != nil {
		return nil, 0, err
	}
	unleaked, qber, err := b.estimateQBER(sifted)
	if err != nil {
		return nil, 0, err
	}
	recRes, err := b.reconciler.Reconcile(unleaked)
	if err != nil {
		return nil, 0, err
	}
	// TODO: calculate possible bit leakage from sifting phase
	// TODO: extract key
	// TODO: if unleaked isn't byte-aligned then this returns a key with a few
	//   predictable zeros on the end. We can either fix that by trimming the
	//   last, unaligned byte, or by using bitarray.Dense as part of the public
	//   interface.
	return recRes.xHat.Data(), qber, nil
}

func (a *alice) sendQBits(rbc int) (bits, bases bitarray.Dense, err error) {
	bitArr := make([]byte, rbc)
	basisArr := make([]byte, rbc)
	a.random.Read(bitArr)
	a.random.Read(basisArr)
	bits = bitarray.NewDense(bitArr, -1)
	bases = bitarray.NewDense(basisArr, -1)
	if err := a.sender.Send(bits.Data(), bases.Data()); err != nil {
		return bitarray.Empty(), bitarray.Empty(), err
	}
	return bits, bases, err
}

func (b *bob) receiveQBits(rbc int) (bits, bases, dropped bitarray.Dense, err error) {
	basisArr := make([]byte, rbc)
	b.random.Read(basisArr)
	bases = bitarray.NewDense(basisArr, -1)
	bitsArr, droppedArr, err := b.receiver.Receive(basisArr)
	bits = bitarray.NewDense(bitsArr, -1)
	dropped = bitarray.NewDense(droppedArr, -1)
	if err != nil {
		return bitarray.Empty(), bitarray.Empty(), bitarray.Empty(), fmt.Errorf("receiving qubits: %w", err)
	}
	return bits, bases, dropped, nil
}

func (a *alice) sift(bits, bases bitarray.Dense) (sifted bitarray.Dense, err error) {
	bba := new(bb84pb.BasisAnnouncement)
	if err := a.sideChannel.Read(bba); err != nil {
		return bitarray.Empty(), fmt.Errorf("receiving basis announcement: %w", err)
	}
	aba := &bb84pb.BasisAnnouncement{Bases: bases.ToProto()}
	if err := a.sideChannel.Write(aba); err != nil {
		return bitarray.Empty(), fmt.Errorf("announcing bases: %w", err)
	}
	bBases := bitarray.DenseFromProto(bba.Bases)
	bDropped := bitarray.DenseFromProto(bba.Dropped)
	return sift(bits, bases, bBases, bDropped), nil
}

func (b *bob) sift(bits, bases, dropped bitarray.Dense) (sifted bitarray.Dense, err error) {
	bba := &bb84pb.BasisAnnouncement{
		Bases:   bases.ToProto(),
		Dropped: dropped.ToProto(),
	}
	if err := b.sideChannel.Write(bba); err != nil {
		return bitarray.Empty(), fmt.Errorf("sending basis announcement: %w", err)
	}
	aba := new(bb84pb.BasisAnnouncement)
	if err := b.sideChannel.Read(aba); err != nil {
		return bitarray.Empty(), fmt.Errorf("receiving bases: %w", err)
	}
	aBasis := bitarray.DenseFromProto(aba.Bases)
	return sift(bits, bases, aBasis, dropped), nil
}

// TODO: support configurable sampling proporation
// TODO: just send a random seed to bob, then use the same sampling procedure on
//   both ends. Less bandwidth that way.
func (a *alice) estimateQBER(sifted bitarray.Dense) (unleaked bitarray.Dense, qber float64, err error) {
	// Announce sampled values
	buf := make([]byte, sifted.ByteSize())
	a.random.Read(buf)
	sampleMask := bitarray.NewDense(buf, sifted.Size())
	sampled, unsampled := partition(sifted, sampleMask)
	bitsAnnounce := &bb84pb.BitAnnouncement{
		Bits: sampled.ToProto(),
		Mask: sampleMask.ToProto(),
	}
	if err := a.sideChannel.Write(bitsAnnounce); err != nil {
		return bitarray.Empty(), 0, fmt.Errorf("announcing bases: %w", err)
	}

	// Receive QBER for sample
	qa := new(bb84pb.QBERAnnouncement)
	if err := a.sideChannel.Read(qa); err != nil {
		return bitarray.Empty(), 0, fmt.Errorf("receiving QBER announcement: %w", err)
	}
	return unsampled, qa.Qber, nil
}

func (b *bob) estimateQBER(sifted bitarray.Dense) (unleaked bitarray.Dense, qber float64, err error) {
	bitsAnnounce := new(bb84pb.BitAnnouncement)
	if err := b.sideChannel.Read(bitsAnnounce); err != nil {
		return bitarray.Empty(), 0, fmt.Errorf("Receiving sampled bits: %w", err)
	}
	aSampled := bitarray.DenseFromProto(bitsAnnounce.Bits)
	sampleMask := bitarray.DenseFromProto(bitsAnnounce.Mask)
	bSampled, unsampled := partition(sifted, sampleMask)

	// Calculate and announce sampled QBER
	errors := aSampled.XOr(bSampled).CountOnes()
	qber = float64(errors) / float64(aSampled.Size())
	qa := &bb84pb.QBERAnnouncement{Qber: qber}
	if err := b.sideChannel.Write(qa); err != nil {
		return bitarray.Empty(), 0, fmt.Errorf("receiving QBER announcement: %w", err)
	}
	return unsampled, qber, nil
}

func sift(bits, sendBasis, receiveBasis, dropped bitarray.Dense) bitarray.Dense {
	siftMask := sendBasis.XNor(receiveBasis)
	if dropped.Size() > 0 {
		siftMask = siftMask.And(dropped)
	}
	return bits.Select(siftMask)
}

func partition(bits, mask bitarray.Dense) (masked, unmasked bitarray.Dense) {
	return bits.Select(mask), bits.Select(mask.Not())
}
