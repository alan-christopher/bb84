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
	key  []byte
	qber float64
	err  error
}

func TestWinnowedNegotation(t *testing.T) {
	l, r := net.Pipe()
	sender, receiver := photon.NewSimulatedChannel(0)
	otp := make([]byte, 4<<10)
	rand.Read(otp)
	diags := make([]byte, 1<<20)
	rand.Read(diags)
	aWire := &protoFramer{
		rw:     l,
		secret: bytes.NewBuffer(otp),
		t:      toeplitz{diags: bitarray.NewDense(diags, -1), m: 40},
	}
	bWire := &protoFramer{
		rw:     r,
		secret: bytes.NewBuffer(otp),
		t:      toeplitz{diags: bitarray.NewDense(diags, -1), m: 40},
	}
	a := alice{
		sideChannel: aWire,
		sender:      sender,
		random:      rand.New(rand.NewSource(42)),
		reconciler: winnower{
			channel: aWire,
			isBob:   false,
			rand:    rand.New(rand.NewSource(17)),
			iters:   []int{3, 3, 3, 4, 6, 7, 7, 7},
		},
	}
	b := bob{
		sideChannel: bWire,
		receiver:    receiver,
		random:      rand.New(rand.NewSource(1337)),
		reconciler: winnower{
			channel: bWire,
			isBob:   true,
			rand:    rand.New(rand.NewSource(17)),
			iters:   []int{3, 3, 3, 4, 6, 7, 7, 7},
		},
	}
	const qBytes = 4 << 10
	legitErrs := bitarray.NewDense(nil, qBytes)
	for i := 0; i < qBytes*8/20; i++ {
		legitErrs.Set(i, true)
	}
	legitErrs.Shuffle(rand.New(rand.NewSource(99)))
	receiver.Errors = legitErrs.Data()

	aResCh := make(chan negotiationResult, 1)
	bResCh := make(chan negotiationResult, 1)
	go func() {
		k, qber, err := a.NegotiateKey(qBytes)
		aResCh <- negotiationResult{k, qber, err}
	}()
	go func() {
		k, qber, err := b.NegotiateKey(qBytes)
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
	if !bytes.Equal(aRes.key, bRes.key) {
		t.Errorf("Alice and Bob disagree on keys: (%b, %b)", aRes.key, bRes.key)
	}
	if len(aRes.key) == 0 {
		t.Errorf("Alice arrived at an empty key")
	}
	if len(bRes.key) == 0 {
		t.Errorf("Bob arrived at an empty key")
	}
}
