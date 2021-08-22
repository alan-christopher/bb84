package bb84

import (
	"bytes"
	"math/rand"
	"net"
	"testing"

	"github.com/alan-christopher/bb84/go/bb84/bitmap"
	"github.com/alan-christopher/bb84/go/bb84/photon"
)

// A convenience struct for pumping the return values from NegotiateKey into a
// channel.
type negotiationResult struct {
	key   bitmap.Dense
	stats Stats
	err   error
}

func TestWinnowedNegotation(t *testing.T) {
	l, r := net.Pipe()
	pa := PulseAttrs{}
	pa.MuLo, pa.MuMed, pa.MuHi = 0.05, 0.1, 0.3
	pa.ProbLo, pa.ProbMed, pa.ProbHi = 0.4, 0.3, 0.3
	sender, receiver := photon.NewSimulatedChannel(
		0.5,                            // pMain
		pa.MuLo,                        // muLo
		pa.MuMed,                       // muMed
		pa.MuHi,                        // muHi
		pa.ProbLo,                      // pLo
		pa.ProbMed,                     // pMed
		pa.ProbHi,                      // pHi
		rand.New(rand.NewSource(1234)), // sendrand
		rand.New(rand.NewSource(5678)), // receiveRand
	)
	otp := make([]byte, 1<<23)
	rand.Read(otp)
	a, err := NewPeer(PeerOpts{
		Sender:           sender,
		ClassicalChannel: l,
		Rand:             rand.New(rand.NewSource(42)),
		Secret:           bytes.NewBuffer(otp),
		WinnowOpts: &WinnowOpts{
			Iters:    []int{3, 3, 3, 4, 6, 7, 7, 7},
			SyncRand: rand.New(rand.NewSource(17)),
		},
		PulseAttrs: pa,
	})
	if err != nil {
		t.Fatalf("Building Alice: %v", err)
	}
	b, err := NewPeer(PeerOpts{
		Receiver:         receiver,
		ClassicalChannel: r,
		Rand:             rand.New(rand.NewSource(1337)),
		Secret:           bytes.NewBuffer(otp),
		WinnowOpts: &WinnowOpts{
			Iters:    []int{3, 3, 3, 4, 6, 7, 7, 7},
			SyncRand: rand.New(rand.NewSource(17)),
		},
		PulseAttrs: pa,
	})
	if err != nil {
		t.Fatalf("Building Bob: %v", err)
	}
	batchBits := DefaultMeasurementBatchBytes * 8
	legitErrs := bitmap.NewDense(nil, batchBits)
	for i := 0; i < batchBits/20; i++ {
		legitErrs.Flip(i)
	}
	legitErrs.Shuffle(rand.New(rand.NewSource(99)))
	receiver.Errors = legitErrs.Data()

	aResCh := make(chan negotiationResult, 1)
	bResCh := make(chan negotiationResult, 1)
	go func() {
		k, s, err := a.NegotiateKey()
		aResCh <- negotiationResult{k, s, err}
	}()
	go func() {
		k, s, err := b.NegotiateKey()
		bResCh <- negotiationResult{k, s, err}
	}()

	var aRes, bRes negotiationResult
	select {
	case res := <-aResCh:
		aRes = res
		if aRes.err == nil {
			bRes = <-bResCh
		}
	case res := <-bResCh:
		bRes = res
		if bRes.err == nil {
			aRes = <-aResCh
		}
	}

	if aRes.err != nil {
		t.Fatalf("Alice error: %v", aRes.err)
	}
	if bRes.err != nil {
		t.Fatalf("Bob error: %v", bRes.err)
	}
	if !bytes.Equal(aRes.key.Data(), bRes.key.Data()) {
		t.Errorf("Alice and Bob disagree on keys: (%v, %v)", aRes.key, bRes.key)
	}
	if aRes.key.Size() != bRes.key.Size() {
		t.Errorf("Alice and Bob have different key lengths: %d != %d", aRes.key.Size(), bRes.key.Size())
	}
	if aRes.key.Size() == 0 {
		t.Errorf("Alice arrived at an empty key")
	}
	if bRes.key.Size() == 0 {
		t.Errorf("Bob arrived at an empty key")
	}
}
