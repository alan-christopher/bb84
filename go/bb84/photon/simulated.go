package photon

import (
	"crypto/rand"
	"fmt"

	"github.com/alan-christopher/bb84/go/bb84/bitarray"
)

// NewSimulatedChannel creates a pair of (Sender, Receiver) structs simulating a
// Quantum channel. It is expected that each call to Send() will be mirrored by
// a call to Receive(). Expect errors if that is not the case, and for calls to
// Send() to hang if more than bufSize of them are made before Receive().
func NewSimulatedChannel(bufSize int) (*SimulatedSender, *SimulatedReceiver) {
	bits := make(chan bitarray.Dense, bufSize)
	bases := make(chan bitarray.Dense, bufSize)
	ss := &SimulatedSender{bits: bits, bases: bases}
	sr := &SimulatedReceiver{bits: bits, bases: bases}
	return ss, sr
}

type SimulatedSender struct {
	bits  chan<- bitarray.Dense
	bases chan<- bitarray.Dense
}

type SimulatedReceiver struct {
	DropMask []byte
	Errors   []byte

	bits  <-chan bitarray.Dense
	bases <-chan bitarray.Dense
}

func (ss *SimulatedSender) Send(bits, bases []byte) error {
	if len(bits) != len(bases) {
		return fmt.Errorf("bit and basis length must agree: %d != %d", len(bits), len(bases))
	}
	ss.bits <- bitarray.NewDense(bits, -1)
	ss.bases <- bitarray.NewDense(bases, -1)
	return nil
}

func (sr *SimulatedReceiver) Receive(bases []byte) (bits, dropped []byte, err error) {
	sendBits := <-sr.bits
	sendBases := <-sr.bases
	if len(bases) != sendBits.ByteSize() {
		return nil, nil, fmt.Errorf("send byte length must match receive basis length: %d != %d", sendBits.ByteSize(), len(bases))
	}
	if len(bases) != sendBases.ByteSize() {
		return nil, nil, fmt.Errorf("send basis length must match receive basis length: %d != %d", sendBits.ByteSize(), len(bases))
	}

	buf := make([]byte, len(bases))
	rand.Read(buf)
	flips := bitarray.NewDense(buf, -1)
	recBases := bitarray.NewDense(bases, -1)
	flips = flips.And(sendBases.XOr(recBases))
	flips = flips.Or(bitarray.NewDense(sr.Errors, -1))
	return flips.XOr(sendBits).Data(), sr.DropMask, nil
}
