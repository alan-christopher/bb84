package bb84

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/alan-christopher/bb84/go/bb84/bitarray"
	"github.com/alan-christopher/bb84/go/bb84/photon"
	"github.com/alan-christopher/bb84/go/generated/bb84pb"
)

// An alice represents the first BB84 participant.
type alice struct {
	sender      photon.Sender
	sideChannel *protoFramer
	rand        *rand.Rand
	reconciler  reconciler
	qBytes      int
	epsPriv     float64
	sampleProp  float64
	qBytesFunc  func() []byte
	basisFunc   func() []byte
}

// A bob represents the second BB84 participant.
type bob struct {
	receiver    photon.Receiver
	sideChannel *protoFramer
	rand        *rand.Rand
	reconciler  reconciler
	qBytes      int
	epsPriv     float64
	sampleProp  float64
	qBytesFunc  func() []byte
	basisFunc   func() []byte
}

// NegotiateKey implements the Peer interface.
func (a *alice) NegotiateKey() (key bitarray.Dense, stats Stats, err error) {
	bits, bases, err := a.sendQBits()
	if err != nil {
		return
	}
	sifted, err := a.sift(bits, bases, &stats)
	if err != nil {
		return
	}
	unleaked, err := a.estimateQBER(sifted, &stats)
	if err != nil {
		return
	}
	recRes, err := a.reconciler.Reconcile(unleaked, &stats)
	if err != nil {
		return
	}
	bitsLeaked := recRes.bitLeakage + calcMaxEveInfo(
		stats.QBER, a.epsPriv, unleaked.Size(), sifted.Size()-unleaked.Size())
	seed, err := a.sendSeed(recRes.xHat.Size(), int(bitsLeaked), &stats)
	if err != nil {
		return
	}
	key, err = extractKey(seed, recRes.xHat, bitsLeaked, a.epsPriv)
	if err != nil {
		return
	}
	return
}

// NegotiateKey implements the Peer interface.
func (b *bob) NegotiateKey() (key bitarray.Dense, stats Stats, err error) {
	bits, bases, dropped, err := b.receiveQBits()
	if err != nil {
		return
	}
	sifted, err := b.sift(bits, bases, dropped, &stats)
	if err != nil {
		return
	}
	unleaked, err := b.estimateQBER(sifted, &stats)
	if err != nil {
		return
	}
	recRes, err := b.reconciler.Reconcile(unleaked, &stats)
	if err != nil {
		return
	}
	bitsLeaked := recRes.bitLeakage + calcMaxEveInfo(
		stats.QBER, b.epsPriv, unleaked.Size(), sifted.Size()-unleaked.Size())
	seed, err := b.receiveSeed(&stats)
	if err != nil {
		return
	}
	key, err = extractKey(seed, recRes.xHat, bitsLeaked, b.epsPriv)
	if err != nil {
		return
	}
	return
}

func (a *alice) sendQBits() (bits, bases bitarray.Dense, err error) {
	var bitArr, basisArr []byte
	if a.qBytesFunc == nil {
		bitArr = make([]byte, a.qBytes)
		a.rand.Read(bitArr)
	} else {
		bitArr = a.qBytesFunc()
	}
	if a.basisFunc == nil {
		basisArr = make([]byte, a.qBytes)
		a.rand.Read(basisArr)
	} else {
		basisArr = a.basisFunc()
	}
	bits = bitarray.NewDense(bitArr, -1)
	bases = bitarray.NewDense(basisArr, -1)
	if err := a.sender.Send(bits.Data(), bases.Data()); err != nil {
		return bitarray.Empty(), bitarray.Empty(), err
	}
	return bits, bases, err
}

func (b *bob) receiveQBits() (bits, bases, dropped bitarray.Dense, err error) {
	var basisArr []byte
	if b.qBytesFunc == nil {
		basisArr = make([]byte, b.qBytes)
		b.rand.Read(basisArr)
	} else {
		basisArr = b.basisFunc()
	}
	bases = bitarray.NewDense(basisArr, -1)
	bitsArr, droppedArr, err := b.receiver.Receive(basisArr)
	bits = bitarray.NewDense(bitsArr, -1)
	dropped = bitarray.NewDense(droppedArr, -1)
	if err != nil {
		return bitarray.Empty(), bitarray.Empty(), bitarray.Empty(), fmt.Errorf("receiving qubits: %w", err)
	}
	return bits, bases, dropped, nil
}

func (a *alice) sift(bits, bases bitarray.Dense, s *Stats) (sifted bitarray.Dense, err error) {
	bba := new(bb84pb.BasisAnnouncement)
	if err := a.sideChannel.Read(bba, s); err != nil {
		return bitarray.Empty(), fmt.Errorf("receiving basis announcement: %w", err)
	}
	aba := &bb84pb.BasisAnnouncement{Bases: bases.ToProto()}
	if err := a.sideChannel.Write(aba, s); err != nil {
		return bitarray.Empty(), fmt.Errorf("announcing bases: %w", err)
	}
	bBases := bitarray.DenseFromProto(bba.Bases)
	bDropped := bitarray.DenseFromProto(bba.Dropped)
	sifted = sift(bits, bases, bBases, bDropped)
	return sifted, nil
}

func (b *bob) sift(bits, bases, dropped bitarray.Dense, s *Stats) (sifted bitarray.Dense, err error) {
	bba := &bb84pb.BasisAnnouncement{
		Bases:   bases.ToProto(),
		Dropped: dropped.ToProto(),
	}
	if err := b.sideChannel.Write(bba, s); err != nil {
		return bitarray.Empty(), fmt.Errorf("sending basis announcement: %w", err)
	}
	aba := new(bb84pb.BasisAnnouncement)
	if err := b.sideChannel.Read(aba, s); err != nil {
		return bitarray.Empty(), fmt.Errorf("receiving bases: %w", err)
	}
	aBasis := bitarray.DenseFromProto(aba.Bases)
	sifted = sift(bits, bases, aBasis, dropped)
	return sifted, nil
}

func (a *alice) estimateQBER(sifted bitarray.Dense, s *Stats) (unleaked bitarray.Dense, err error) {
	// Announce sampled bits
	seed := a.rand.Int63()
	unsampled, sampled, err := sample(sifted, a.sampleProp, seed)
	if err != nil {
		return bitarray.Empty(), err
	}
	bitsAnnounce := &bb84pb.BitAnnouncement{
		Bits:        sampled.ToProto(),
		ShuffleSeed: seed,
	}
	if err := a.sideChannel.Write(bitsAnnounce, s); err != nil {
		return bitarray.Empty(), fmt.Errorf("announcing bases: %w", err)
	}

	// Receive QBER for sample
	qa := new(bb84pb.QBERAnnouncement)
	if err := a.sideChannel.Read(qa, s); err != nil {
		return bitarray.Empty(), fmt.Errorf("receiving QBER announcement: %w", err)
	}
	s.QBER = qa.Qber
	return unsampled, nil
}

func (b *bob) estimateQBER(sifted bitarray.Dense, s *Stats) (unleaked bitarray.Dense, err error) {
	bitsAnnounce := new(bb84pb.BitAnnouncement)
	if err := b.sideChannel.Read(bitsAnnounce, s); err != nil {
		return bitarray.Empty(), fmt.Errorf("receiving sampled bits: %w", err)
	}
	aSampled := bitarray.DenseFromProto(bitsAnnounce.Bits)
	unsampled, bSampled, err := sample(sifted, b.sampleProp, bitsAnnounce.ShuffleSeed)
	if err != nil {
		return bitarray.Empty(), fmt.Errorf("sampling bits: %w", err)
	}
	// Calculate and announce sampled QBER
	errors := aSampled.XOr(bSampled).CountOnes()
	s.QBER = float64(errors) / float64(aSampled.Size())
	qa := &bb84pb.QBERAnnouncement{Qber: s.QBER}
	if err := b.sideChannel.Write(qa, s); err != nil {
		return bitarray.Empty(), fmt.Errorf("sending QBER announcement: %w", err)
	}

	return unsampled, nil
}

func (a *alice) sendSeed(bitCount, leakage int, s *Stats) (bitarray.Dense, error) {
	// This actually slightly overcounts the number of bits of seed to generate
	// and send, by overestimating the hash output dimension by not accounting
	// for the epsilon parameter in the LHL.
	needed := bitCount + (bitCount - leakage) - 1
	seed := make([]byte, bitarray.BytesFor(needed))
	a.rand.Read(seed)
	if err := a.sideChannel.Write(&bb84pb.SeedAnnouncement{Seed: seed}, s); err != nil {
		return bitarray.Empty(), err
	}
	return bitarray.NewDense(seed, -1), nil
}

func (b *bob) receiveSeed(s *Stats) (bitarray.Dense, error) {
	m := &bb84pb.SeedAnnouncement{}
	if err := b.sideChannel.Read(m, s); err != nil {
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

func sample(bits bitarray.Dense, proportion float64, seed int64) (unsampled, sampled bitarray.Dense, err error) {
	r := rand.New(rand.NewSource(seed))
	bits.Shuffle(r)
	n := bits.Size()
	k := int(proportion * float64(n))
	unsampled, err = bits.Slice(0, n-k)
	if err != nil {
		return bitarray.Empty(), bitarray.Empty(), nil
	}
	sampled, err = bits.Slice(n-k, n)
	if err != nil {
		return bitarray.Empty(), bitarray.Empty(), nil
	}
	return unsampled, sampled, nil
}

func extractKey(seed, x bitarray.Dense, bitsLeaked, eps float64) (bitarray.Dense, error) {
	t := toeplitz{
		diags: seed,
		m:     x.Size() - int(math.Ceil(bitsLeaked+2*math.Log(1/eps))),
		n:     x.Size(),
	}
	return t.Mul(x)
}
