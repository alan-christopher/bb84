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

func TestNoiselessNegotation(t *testing.T) {
	l, r := net.Pipe()
	sender, receiver := photon.NewSimulatedChannel(0)
	otp := make([]byte, 1024)
	rand.Read(otp)
	diags := make([]byte, 1024)
	rand.Read(diags)
	a := alice{
		sideChannel: &protoFramer{
			rw:     l,
			secret: bytes.NewBuffer(otp),
			t:      toeplitz{diags: bitarray.NewDense(diags, -1), m: 40},
		},
		sender: sender,
		random: rand.New(rand.NewSource(42)),
	}
	b := bob{
		sideChannel: &protoFramer{
			rw:     r,
			secret: bytes.NewBuffer(otp),
			t:      toeplitz{diags: bitarray.NewDense(diags, -1), m: 40},
		},
		receiver: receiver,
		random:   rand.New(rand.NewSource(1337)),
	}

	aResCh := make(chan negotiationResult, 1)
	bResCh := make(chan negotiationResult, 1)
	go func() {
		k, qber, err := a.NegotiateKey(4)
		aResCh <- negotiationResult{k, qber, err}
	}()
	go func() {
		k, qber, err := b.NegotiateKey(4)
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
	if aRes.qber != 0 || bRes.qber != 0 {
		t.Errorf("Expected 0 QBER from (alice, bob), got (%f, %f)", aRes.qber, bRes.qber)
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
