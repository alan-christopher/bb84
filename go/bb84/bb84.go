// Package bb84 provides utilities for negotiating a shared secret using the
// BB84 protocol.
package bb84

import (
	"io"

	"github.com/alan-christopher/bb84/go/bb84/photon"
)

// TODO: can you condense this interface down to a single Reconcile method? Is
// there any advantage to that past aesthetics?
type Peer interface {
	NegotiateKey(rawByteCount int) ([]byte, float64, error)
}

// TODO: error correction
// TODO: decoy states
// TODO: Key Extraction
// TODO: public constructors for making peers

// An alice represents the first BB84 participant.
type alice struct {
	sender      photon.Sender
	sideChannel *protoFramer
	random      io.Reader
}

// A bob represents the second BB84 participant.
type bob struct {
	receiver    photon.Receiver
	sideChannel *protoFramer
	random      io.Reader
}
