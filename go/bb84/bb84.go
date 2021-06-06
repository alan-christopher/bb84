// Package bb84 provides utilities for negotiating a shared secret using the
// BB84 protocol.
package bb84

import (
	"errors"
	"io"
	"math"
	"math/rand"

	"github.com/alan-christopher/bb84/go/bb84/bitarray"
	"github.com/alan-christopher/bb84/go/bb84/photon"
)

var (
	DefaultQBytes           = 16 << 10
	DefaultEpsilon          = 1e-12
	DefaultSampleProportion = 0.5
)

// Stats packages together a collection of potentially interesting metrics
// pertaining to a BB84 key negotiation.
type Stats struct {
	QBER             float64
	MessagesSent     int
	MessagesReceived int
	BytesRead        int
	BytesSent        int
}

// A Peer represents one of the two legitimate participants in a BB84 key
// exchange.
type Peer interface {
	// NegotiateKey performs one round of BB84 key exchange, including
	// "post-processing" steps, e.g.  error correction and privacy
	// amplification.
	NegotiateKey() (bitarray.Dense, Stats, error)
}

// A PeerOpts packages together the arguments necessary to construct a new Peer. Many of the fields
// of a PeerOpts do *not* have reasonable defaults, and leaving those fields to zero-initialize will
// result in NewPeer returning an error.
type PeerOpts struct {
	// Sender/Receiver is responsible sending/receiving photons. Exactly one must be non-nil.
	Sender   photon.Sender
	Receiver photon.Receiver

	// ClassicalChannel provides a channel for classical communications. Must be
	// non-nil.
	ClassicalChannel io.ReadWriter

	// Rand provides a source of randomness. This may use pRNG for experimental
	// and/or testing, but for unconditional security this must be
	// truly random. Must be non-nil.
	Rand *rand.Rand

	// Secret provides a bootstrap secret shared between Alice and Bob for
	// authentication. Must be non-nil.
	Secret io.Reader

	// QBytes specifies the number of quantum bytes to exchange
	// per call to NegotiateKey.  Defaults to DefaultQBytes.
	QBytes int

	// EpsilonAuth specifies the probability that we are willing to accept that
	// Eve can forge a message. Each classical message exchanged spends
	// log_2(1/EpsilonAuth) bits of Secret, rounded up to the nearest byte.
	//
	// Defaults to DefaultEpsilon.
	EpsilonAuth float64

	// EpsilonPrivacy specifies the statistical distance from uniform we are willing
	// to tolerate our final extracted key being, conditioned on the information made
	// available during the public phases of the protocol.
	//
	// Defaults to DefaultEpsilon.
	EpsilonPrivacy float64

	// SampleProportion specifies the proportion of sifted bits to sample during
	// error rate estimation. Defaults to half.
	SampleProportion float64

	// WinnowOpts provides options for using Winnow (see
	// https://arxiv.org/abs/quant-ph/0203096) for error correction. Non-nil iff
	// using Winnow for information reconciliation.
	WinnowOpts *WinnowOpts
}

// A WinnowOpts packages together the parameters necessary for the Winnow error
// correction scheme (see https://arxiv.org/abs/quant-ph/0203096).
type WinnowOpts struct {
	// SyncRand provides a *synchronized* randomness source between Alice and
	// Bob. This source is only used to de-correlate error positions in the
	// shared secret and may operate on pRNG. Must be non-nil.
	SyncRand *rand.Rand

	// Iters specifies the sequence of hamming bit counts to use during
	// winnowing. E.g.  a sequence {3,3,4} performs two rounds of winnowing with
	// 8-bit code blocks, followed by one with 16-bit code blocks.
	Iters []int
}

// NewPeer returns a new Peer, configured in accordance with opts, or an error
// if the options are nonsensical.
func NewPeer(opts PeerOpts) (Peer, error) {
	if (opts.Sender == nil) == (opts.Receiver == nil) {
		return nil, errors.New("exactly one of {Sender, Receiver} must be specified")
	}
	if opts.ClassicalChannel == nil {
		return nil, errors.New("must provide ClassicalChannel")
	}
	if opts.Rand == nil {
		return nil, errors.New("must provide Rand")
	}
	if opts.Secret == nil {
		return nil, errors.New("must provide Secret")
	}
	// Only option for reconciliation at the moment is winnow.
	if opts.WinnowOpts == nil {
		return nil, errors.New("must provide reconciliation options")
	}
	qBytes := opts.QBytes
	if qBytes == 0 {
		qBytes = DefaultQBytes
	}
	epsAuth := opts.EpsilonAuth
	if epsAuth == 0 {
		epsAuth = DefaultEpsilon
	}
	epsPriv := opts.EpsilonAuth
	if epsPriv == 0 {
		epsPriv = DefaultEpsilon
	}
	sampleProp := opts.SampleProportion
	if sampleProp == 0 {
		sampleProp = DefaultSampleProportion
	}

	diags := make([]byte, 2*qBytes+8)
	if _, err := io.ReadFull(opts.Secret, diags); err != nil {
		return nil, err
	}
	pf := &protoFramer{
		rw:     opts.ClassicalChannel,
		secret: opts.Secret,
		t: toeplitz{
			diags: bitarray.NewDense(diags, -1),
			m:     int(math.Ceil(math.Log2(1 / epsAuth))),
		},
	}
	rec := winnower{
		channel: pf,
		rand:    opts.WinnowOpts.SyncRand,
		iters:   opts.WinnowOpts.Iters,
		isAlice: opts.Sender != nil,
	}

	if opts.Sender != nil {
		return &alice{
			sender:      opts.Sender,
			sideChannel: pf,
			reconciler:  rec,
			rand:        opts.Rand,
			qBytes:      qBytes,
			epsPriv:     epsPriv,
			sampleProp:  sampleProp,
		}, nil
	}
	return &bob{
		receiver:    opts.Receiver,
		sideChannel: pf,
		reconciler:  rec,
		rand:        opts.Rand,
		qBytes:      qBytes,
		epsPriv:     epsPriv,
		sampleProp:  sampleProp,
	}, nil
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
	Reconcile(x bitarray.Dense, s *Stats) (reconcileResult, error)
}
