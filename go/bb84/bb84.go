// Package bb84 provides utilities for negotiating a shared secret using the
// BB84 protocol.
package bb84

import (
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"

	"github.com/alan-christopher/bb84/go/bb84/bitmap"
	"github.com/alan-christopher/bb84/go/bb84/photon"
)

var (
	DefaultMeasurementBatchBytes = 1 << 14
	DefaultMainBlockSize         = int(1e5)
	DefaultTestBlockSize         = int(1e5)
	DefaultEpsilon               = 1e-12
)

// Stats packages together a collection of potentially interesting metrics
// pertaining to a BB84 key negotiation.
type Stats struct {
	Pulses           int
	QBits            int
	QBER             float64
	MessagesSent     int
	MessagesReceived int
	BytesRead        int
	BytesSent        int
}

// TODO: make Peer embed io.Reader, expose Stats via a secondary method, and
// make bitmap an internal lib.
// A Peer represents one of the two legitimate participants in a BB84 key
// exchange.
type Peer interface {
	// NegotiateKey performs one round of BB84 key exchange, including
	// "post-processing" steps, e.g.  error correction and privacy
	// amplification.
	NegotiateKey() (bitmap.Dense, Stats, error)
}

// A PeerOpts packages together the arguments necessary to construct a new Peer. Many of the fields
// of a PeerOpts do *not* have reasonable defaults, and leaving those fields to zero-initialize will
// result in NewPeer returning an error.
type PeerOpts struct {
	// Sender/Receiver handles photon transmission. Exactly one must be non-nil.
	Sender   photon.Sender
	Receiver photon.Receiver

	// ClassicalChannel provides a channel for classical communications. Must be
	// non-nil.
	ClassicalChannel io.ReadWriter

	// Rand provides a source of randomness, e.g. for salting hashes. This may
	// reasonably use pRNG for experimental and/or testing purposes, but for
	// unconditional security it should be truly random. Must be non-nil.
	Rand *rand.Rand

	// Secret provides a bootstrap secret shared between Alice and Bob for
	// authentication. Must be non-nil.
	Secret io.Reader

	// MeasurementBatchBytes specifies the number of bytes worth of qubit
	// measurements to batch together before performing a basis announcement.
	//
	// Defaults to DefaultMeasurementBatchBytes.
	MeasurementBatchBytes int

	// MainBlockSize specifies the minimum number of qubit measurements in the
	// "main" basis to accumulate before moving on to error correction, privacy
	// amplificiation, etc.
	//
	// Defaults to DefaultMainBlockSize.
	MainBlockSize int

	// TestBlockSize specifies the minimum number of qubit measurements in the
	// "test" basis to accumulate beforem oving on to error correction, privacy
	// amplificiation, etc.
	//
	// Defaults to DefaultTestBlockSize.
	TestBlockSize int

	// EpsilonAuth specifies the probability that we are willing to accept that
	// Eve can forge a message. Each classical message exchanged spends
	// log_2(1/EpsilonAuth) bits of Secret, rounded up to the nearest byte.
	//
	// Defaults to DefaultEpsilon.
	EpsilonAuth float64

	// EpsilonCorrect specifies the maximum acceptable probability that Alice
	// and Bob wind up with different secrets at the end of key negotiation.
	//
	// Defaults to DefaultEpsilon.
	EpsilonCorrect float64

	// EpsilonPrivacy specifies the statistical distance from uniform we are willing
	// to tolerate our final extracted key being, conditioned on the information made
	// available during the public phases of the protocol.
	//
	// Defaults to DefaultEpsilon.
	EpsilonPrivacy float64

	// PulseAttrs provide information about the attenuated laser pulses used to
	// carry information between Alice and Bob. Must be provided.
	PulseAttrs PulseAttrs

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

// PulseAttrs provide information about the attenuated laser pulses used to
// carry information between Alice and Bob. We assume a decoy-state setup with
// three total states.
type PulseAttrs struct {
	// MuLo, MuMed, and MuHi specify the mean photons per pulse of the low,
	// medium, and high intensity pulse states, respectively. It is required
	// that:
	//
	// (0 <= MuLo < MuMed < MuHi) and (MuLo + MuMed < MuHi)
	MuLo, MuMed, MuHi float64

	// ProbLo, ProbMed, and ProbHi describe the underlying probability that any
	// given pulse will be prepared at low, medium, or high intensities. It is
	// required that the three of them sum to one.
	ProbLo, ProbMed, ProbHi float64
}

// NewPeer returns a new Peer, configured in accordance with opts, or an error
// if the options are nonsensical.
func NewPeer(opts PeerOpts) (Peer, error) {
	if err := checkOpts(opts); err != nil {
		return nil, err
	}
	nX := opts.MainBlockSize
	if nX == 0 {
		nX = DefaultMainBlockSize
	}
	nZ := opts.TestBlockSize
	if nZ == 0 {
		nZ = DefaultTestBlockSize
	}
	epsAuth := opts.EpsilonAuth
	if epsAuth == 0 {
		epsAuth = DefaultEpsilon
	}
	epsPriv := opts.EpsilonAuth
	if epsPriv == 0 {
		epsPriv = DefaultEpsilon
	}
	epsCorrect := opts.EpsilonCorrect
	if epsCorrect == 0 {
		epsCorrect = DefaultEpsilon
	}
	batchBytes := opts.MeasurementBatchBytes
	if batchBytes == 0 {
		batchBytes = DefaultMeasurementBatchBytes
	}

	diags := make([]byte, max(5*(batchBytes+4), 2*(nX+4))+40+8)
	if _, err := io.ReadFull(opts.Secret, diags); err != nil {
		return nil, err
	}
	pf := &protoFramer{
		rw:     opts.ClassicalChannel,
		secret: opts.Secret,
		t: toeplitz{
			diags: bitmap.NewDense(diags, -1),
			m:     int(math.Ceil(math.Log2(1 / epsAuth))),
		},
	}
	rec := winnower{
		channel: pf,
		rand:    opts.WinnowOpts.SyncRand,
		iters:   opts.WinnowOpts.Iters,
		isAlice: opts.Sender != nil,
	}
	if opts.Sender == nil {
		return &bob{
			receiver:       opts.Receiver,
			sideChannel:    pf,
			reconciler:     rec,
			measBatchBytes: batchBytes,
			rand:           opts.Rand,
			epsPriv:        epsPriv,
			epsCorrect:     epsCorrect,
			pulseAttrs:     opts.PulseAttrs,
			nX:             nX,
			nZ:             nZ,
		}, nil
	}
	return &alice{
		sender:         opts.Sender,
		sideChannel:    pf,
		reconciler:     rec,
		measBatchBytes: batchBytes,
		rand:           opts.Rand,
		epsPriv:        epsPriv,
		epsCorrect:     epsCorrect,
		pulseAttrs:     opts.PulseAttrs,
		nX:             nX,
		nZ:             nZ,
	}, nil
}

func checkOpts(opts PeerOpts) error {
	if (opts.Sender == nil) == (opts.Receiver == nil) {
		return errors.New("exactly one of {Sender, Receiver} must be specified")
	}
	if opts.ClassicalChannel == nil {
		return errors.New("must provide ClassicalChannel")
	}
	if opts.Rand == nil {
		return errors.New("must provide Rand")
	}
	if opts.Secret == nil {
		return errors.New("must provide Secret")
	}
	// Only option for reconciliation at the moment is winnow.
	if opts.WinnowOpts == nil {
		return errors.New("must provide reconciliation options")
	}
	lo, med, hi := opts.PulseAttrs.MuLo, opts.PulseAttrs.MuMed, opts.PulseAttrs.MuHi
	if 0 > lo || lo >= med || med >= hi {
		return fmt.Errorf("pulse intensities must satisfy 0 <= lo < med < hi, but !(0 <= %f < %f < %f)",
			lo, med, hi)
	}
	if lo+med >= hi {
		return fmt.Errorf("pulse intensities must satisfy lo + med < hi, but !(%f + %f < %f)",
			lo, med, hi)
	}
	p1, p2, p3 := opts.PulseAttrs.ProbLo, opts.PulseAttrs.ProbMed, opts.PulseAttrs.ProbHi
	if math.Abs(p1+p2+p3-1) > 1e-9 {
		return fmt.Errorf("decoy state proportions must sum to one, %f+%f+%f=%f",
			p1, p2, p3, p1+p2+p3)
	}
	return nil
}

type reconcileResult struct {
	xHat       bitmap.Dense
	bitsLeaked int
}

type reconciler interface {
	// Reconcile performs "error correction" on x, so that this reconciler and
	// its sibling compute the same xHat with high probability. Note that the
	// reconciler interface does not guarantee that all modifications to x occur
	// on one side of the channel.
	Reconcile(x bitmap.Dense, s *Stats) (reconcileResult, error)
}
