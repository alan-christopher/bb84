// Package bb84 provides utilities for negotiating a shared secret using the
// BB84 protocol.
package bb84

import (
	"io"

	"github.com/alan-christopher/bb84/go/bb84/bitarray"
	"github.com/alan-christopher/bb84/go/bb84/photon"
)

type Peer interface {
	NegotiateKey(rawByteCount int) (bitarray.Dense, float64, error)
}

type reconcileResult struct {
	xHat       bitarray.Dense
	bitLeakage float64
}

type reconciler interface {
	// Reconcile performs "error correction" on x, so that this reconciler and
	// its sibling compute the same xHat with high probability. Note that the
	// reconciler interface does not guarantee that all modifications to x occur
	// on one side of the channel.
	Reconcile(x bitarray.Dense) (reconcileResult, error)
}

// TODO: public constructors for making peers

// An alice represents the first BB84 participant.
type alice struct {
	sender      photon.Sender
	sideChannel *protoFramer
	random      io.Reader
	reconciler  reconciler
}

// A bob represents the second BB84 participant.
type bob struct {
	receiver    photon.Receiver
	sideChannel *protoFramer
	random      io.Reader
	reconciler  reconciler
}
