package bb84

import (
	"fmt"
	"io"
	"math"

	"github.com/alan-christopher/bb84/go/bb84/bitarray"
	"github.com/alan-christopher/bb84/go/bb84/photon"
	"github.com/alan-christopher/bb84/go/generated/bb84pb"
)

// An alice represents the first BB84 participant.
type alice struct {
	sender      photon.Sender
	sideChannel *protoFramer
	rand        io.Reader
	reconciler  reconciler
	qBytes      int
	epsPriv     float64
}

// A bob represents the second BB84 participant.
type bob struct {
	receiver    photon.Receiver
	sideChannel *protoFramer
	rand        io.Reader
	reconciler  reconciler
	qBytes      int
	epsPriv     float64
}

// NegotiateKey implements the Peer interface.
func (a *alice) NegotiateKey() (bitarray.Dense, float64, error) {
	bits, bases, err := a.sendQBits()
	if err != nil {
		return bitarray.Empty(), 0, err
	}
	sifted, err := a.sift(bits, bases)
	if err != nil {
		return bitarray.Empty(), 0, err
	}
	unleaked, qber, err := a.estimateQBER(sifted)
	if err != nil {
		return bitarray.Empty(), 0, err
	}
	recRes, err := a.reconciler.Reconcile(unleaked)
	if err != nil {
		return bitarray.Empty(), 0, err
	}
	bitsLeaked := recRes.bitLeakage + calcMaxEveInfo(qber, a.epsPriv, unleaked.Size(), sifted.Size()-unleaked.Size())
	seed, err := a.sendSeed(recRes.xHat.Size(), int(bitsLeaked))
	if err != nil {
		return bitarray.Empty(), 0, err
	}
	k, err := extractKey(seed, recRes.xHat, bitsLeaked, a.epsPriv)
	if err != nil {
		return bitarray.Empty(), 0, err
	}
	return k, qber, nil
}

// NegotiateKey implements the Peer interface.
func (b *bob) NegotiateKey() (bitarray.Dense, float64, error) {
	bits, bases, dropped, err := b.receiveQBits()
	if err != nil {
		return bitarray.Empty(), 0, err
	}
	sifted, err := b.sift(bits, bases, dropped)
	if err != nil {
		return bitarray.Empty(), 0, err
	}
	unleaked, qber, err := b.estimateQBER(sifted)
	if err != nil {
		return bitarray.Empty(), 0, err
	}
	recRes, err := b.reconciler.Reconcile(unleaked)
	if err != nil {
		return bitarray.Empty(), 0, err
	}
	bitsLeaked := recRes.bitLeakage + calcMaxEveInfo(qber, b.epsPriv, unleaked.Size(), sifted.Size()-unleaked.Size())
	seed, err := b.receiveSeed()
	if err != nil {
		return bitarray.Empty(), 0, err
	}
	k, err := extractKey(seed, recRes.xHat, bitsLeaked, b.epsPriv)
	if err != nil {
		return bitarray.Empty(), 0, err
	}
	return k, qber, nil
}

func (a *alice) sendQBits() (bits, bases bitarray.Dense, err error) {
	bitArr := make([]byte, a.qBytes)
	basisArr := make([]byte, a.qBytes)
	a.rand.Read(bitArr)
	a.rand.Read(basisArr)
	bits = bitarray.NewDense(bitArr, -1)
	bases = bitarray.NewDense(basisArr, -1)
	if err := a.sender.Send(bits.Data(), bases.Data()); err != nil {
		return bitarray.Empty(), bitarray.Empty(), err
	}
	return bits, bases, err
}

func (b *bob) receiveQBits() (bits, bases, dropped bitarray.Dense, err error) {
	basisArr := make([]byte, b.qBytes)
	b.rand.Read(basisArr)
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

// TODO: support configurable sampling proportion
// TODO: just send a random seed to bob, then use the same sampling procedure on
//   both ends. Less bandwidth that way.
func (a *alice) estimateQBER(sifted bitarray.Dense) (unleaked bitarray.Dense, qber float64, err error) {
	// Announce sampled values
	buf := make([]byte, sifted.ByteSize())
	a.rand.Read(buf)
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

func (a *alice) sendSeed(bitCount, leakage int) (bitarray.Dense, error) {
	// This actually slightly overcounts the number of bits of seed to generate
	// and send, by overestimating the hash output dimension by not accounting
	// for the epsilon parameter in the LHL.
	needed := bitCount + (bitCount - leakage) - 1
	seed := make([]byte, bitarray.BytesFor(needed))
	a.rand.Read(seed)
	if err := a.sideChannel.Write(&bb84pb.SeedAnnouncement{Seed: seed}); err != nil {
		return bitarray.Empty(), err
	}
	return bitarray.NewDense(seed, -1), nil
}

func (b *bob) receiveSeed() (bitarray.Dense, error) {
	m := &bb84pb.SeedAnnouncement{}
	if err := b.sideChannel.Read(m); err != nil {
		return bitarray.Empty(), err
	}
	return bitarray.NewDense(m.Seed, -1), nil
}

// calcMaxEveInfo returns a theoretical bound on the number of bits of
// information that Eve could have discerned from a quantum communication
// consisting of n qbits for which an error rate of qber was observed.
//
// See also, https://link.springer.com/article/10.1007/BF00191318
func calcMaxEveInfo(qber, eps float64, n, k int) float64 {
	// TODO: account for possible beam-splitting leakage.

	// See https://arxiv.org/abs/1506.08458, lemma 6.
	A := float64(n*k*k) / float64((n+k)*(k+1))
	nu := math.Sqrt(0.5 * math.Log(1/eps) / A)
	qberPessimistic := qber + nu

	// See https://link.springer.com/article/10.1007/BF00191318.
	return 2 * math.Sqrt(2) * qberPessimistic * float64(n)
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

func extractKey(seed, x bitarray.Dense, bitsLeaked, eps float64) (bitarray.Dense, error) {
	t := toeplitz{
		diags: seed,
		m:     x.Size() - int(math.Ceil(bitsLeaked+2*math.Log(1/eps))),
		n:     x.Size(),
	}
	return t.Mul(x)
}
