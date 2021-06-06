package bb84

import (
	"bytes"
	"math/rand"
	"net"
	"testing"

	"github.com/alan-christopher/bb84/go/bb84/bitarray"
	"github.com/alan-christopher/bb84/go/bb84/photon"
)

// A convenience struct for pumping the return values from NegotiateKey into a
// channel.
type negotiationResult struct {
	key  bitarray.Dense
	qber float64
	err  error
}

func TestWinnowedNegotation(t *testing.T) {
	l, r := net.Pipe()
	sender, receiver := photon.NewSimulatedChannel(0)
	otp := make([]byte, 1<<20)
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
	})
	if err != nil {
		t.Fatalf("Building Bob: %v", err)
	}
	legitErrs := bitarray.NewDense(nil, DefaultQBytes)
	for i := 0; i < DefaultQBytes*8/20; i++ {
		legitErrs.Set(i, true)
	}
	legitErrs.Shuffle(rand.New(rand.NewSource(99)))
	receiver.Errors = legitErrs.Data()

	aResCh := make(chan negotiationResult, 1)
	bResCh := make(chan negotiationResult, 1)
	go func() {
		k, qber, err := a.NegotiateKey()
		aResCh <- negotiationResult{k, qber, err}
	}()
	go func() {
		k, qber, err := b.NegotiateKey()
		bResCh <- negotiationResult{k, qber, err}
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
		t.Errorf("Alice and Bob disagree on keys: (%b, %b)", aRes.key, bRes.key)
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
