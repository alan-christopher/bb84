package bb84

import (
	"fmt"
	"math/rand"

	"github.com/alan-christopher/bb84/go/bb84/bitarray"
	"github.com/alan-christopher/bb84/go/bb84/photon"
	"github.com/alan-christopher/bb84/go/generated/bb84pb"
)

// TODO: error correction
// TODO: decoy states
// TODO: Key Extraction
// TODO: public constructors for making peers

// An alice represents the BB84 participant responsible for sending photons.
type alice struct {
	sideChannel *protoFramer
	sender      photon.Sender
	random      *rand.Rand
}

// NegotiateKey implements the Peer interface.
func (a *alice) NegotiateKey(rawByteCount int) ([]byte, float64, error) {
	// Send a sequence of qbits to Bob.
	bitArr := make([]byte, rawByteCount)
	basisArr := make([]byte, rawByteCount)
	a.random.Read(bitArr)
	a.random.Read(basisArr)
	bits := bitarray.NewDense(bitArr, -1)
	aBasis := bitarray.NewDense(basisArr, -1)
	if err := a.sender.Send(bits.Data(), aBasis.Data()); err != nil {
		return nil, 0, err
	}

	// Announce basis choices to Bob
	aba := &bb84pb.BasisAnnouncement{Bases: aBasis.ToProto()}
	if err := a.sideChannel.Write(aba); err != nil {
		return nil, 0, fmt.Errorf("announcing bases: %w", err)
	}

	// Receive basis choices from Bob, and which pulses were dropped.
	bba := new(bb84pb.BasisAnnouncement)
	if err := a.sideChannel.Read(bba); err != nil {
		return nil, 0, fmt.Errorf("receiving basis announcement: %w", err)
	}
	bBasis := bitarray.DenseFromProto(bba.Bases)
	bDropped := bitarray.DenseFromProto(bba.Dropped)
	sifted := sift(bits, aBasis, bBasis, bDropped)

	// TODO: configurable sampling proportion
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
		return nil, 0, fmt.Errorf("announcing bases: %w", err)
	}

	// Receive QBER for sample
	qa := new(bb84pb.QBERAnnouncement)
	if err := a.sideChannel.Read(qa); err != nil {
		return nil, 0, fmt.Errorf("receiving QBER announcement: %w", err)
	}
	return unsampled.Data(), qa.Qber, nil
}

// A bob represents the BB84 participant responsible for receiving photons.
type bob struct {
	sideChannel *protoFramer
	receiver    photon.Receiver
	random      *rand.Rand
}

// NegotiateKey implements the Peer interface.
func (b *bob) NegotiateKey(rawByteCount int) ([]byte, float64, error) {
	// Receive a sequence of qubits from Alice.
	basisArr := make([]byte, rawByteCount)
	b.random.Read(basisArr)
	bBasis := bitarray.NewDense(basisArr, -1)
	bitsArr, droppedArr, err := b.receiver.Receive(basisArr)
	bits := bitarray.NewDense(bitsArr, -1)
	dropped := bitarray.NewDense(droppedArr, -1)
	if err != nil {
		return nil, 0, fmt.Errorf("receiving qubits: %w", err)
	}

	// Receive basis choices from Alice.
	aba := new(bb84pb.BasisAnnouncement)
	if err := b.sideChannel.Read(aba); err != nil {
		return nil, 0, fmt.Errorf("receiving bases: %w", err)
	}
	aBasis := bitarray.DenseFromProto(aba.Bases)

	// Send basis choices to Alice, and which pulses were dropped.
	bba := &bb84pb.BasisAnnouncement{
		Bases:   bBasis.ToProto(),
		Dropped: dropped.ToProto(),
	}
	if err := b.sideChannel.Write(bba); err != nil {
		return nil, 0, fmt.Errorf("sending basis announcement: %w", err)
	}
	sifted := sift(bits, aBasis, bBasis, dropped)

	// TODO: configurable sampling proportion
	// Receive sampled values
	bitsAnnounce := new(bb84pb.BitAnnouncement)
	if err := b.sideChannel.Read(bitsAnnounce); err != nil {
		return nil, 0, fmt.Errorf("Receiving sampled bits: %w", err)
	}
	aSampled := bitarray.DenseFromProto(bitsAnnounce.Bits)
	sampleMask := bitarray.DenseFromProto(bitsAnnounce.Mask)
	bSampled, unsampled := partition(sifted, sampleMask)

	// Calculate and announce sampled QBER
	errors := aSampled.XOr(bSampled).CountOnes()
	qber := float64(errors) / float64(aSampled.Size())
	qa := &bb84pb.QBERAnnouncement{Qber: qber}
	if err := b.sideChannel.Write(qa); err != nil {
		return nil, 0, fmt.Errorf("receiving QBER announcement: %w", err)
	}

	return unsampled.Data(), qber, nil
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
