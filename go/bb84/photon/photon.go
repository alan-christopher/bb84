// Package photon provides utilities for handling photon-encoded qubits.
package photon

// A Sender sends qubits encoded as linearly-polarized photons to a Receiver.
type Sender interface {
	// Next returns the results of sending the next batch of qbits:
	//  - bits contains the logical bit values sent/received
	//  - bases specifies whether the "main" basis was used (0) or the "test"
	//    basis (1), as in basis-biased BB84. For efficient qbit usage, the
	//    ratio of 0s to 1s should be high.
	//  - lo, med, and hi provide bitmasks which indicate the pulse strength
	//    used to send the corresponding qbit.
	Next(bytes int) (bits, bases, lo, med, hi []byte, err error)
}

// A Receiver receives linearly-polarized photons and decodes them in a given
// measurement basis.
type Receiver interface {
	// Next returns the results of sending the next batch of qbits:
	//  - bits contains the logical bit values sent/received
	//  - bases specifies whether the "main" basis was used (0) or the "test"
	//    basis (1), as in basis-biased BB84. For efficient qbit usage, the
	//    ratio of 0s to 1s should be high.
	//  - dropped provides a bitmask indicating which pulses we failed to detect
	//    at all.
	Next(bytes int) (bits, bases, dropped []byte, err error)
}
