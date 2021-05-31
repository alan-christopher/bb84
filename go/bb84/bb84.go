// Package bb84 provides utilities for negotiating a shared secret using the
// BB84 protocol.
package bb84

type Peer interface {
	NegotiateKey(rawBitCount int) ([]byte, float64, error)
}
