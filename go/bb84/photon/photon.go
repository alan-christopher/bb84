// Package photon provides utilities for handling photon-encoded qubits.
package photon

// A Sender sends qubits encoded as linearly-polarized photons to a Receiver.
type Sender interface {
	Send(bits, bases []byte) error
}

// A Receiver receives linearly-polarized photons and decodes them in a given
// measurement basis.
type Receiver interface {
	Receive(bases []byte) (bits, dropped []byte, err error)
}
