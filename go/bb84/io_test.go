package bb84

import (
	"bytes"
	"math/rand"
	"net"
	"testing"

	"github.com/alan-christopher/bb84/go/bb84/bitarray"
	"github.com/alan-christopher/bb84/go/generated/bb84pb"
	"google.golang.org/protobuf/proto"
)

func TestSendReceive(t *testing.T) {
	l, r := net.Pipe()
	otp := make([]byte, 1024)
	rand.Read(otp)
	diags := make([]byte, 1024)
	rand.Read(diags)
	alice := &protoFramer{
		rw:     l,
		secret: bytes.NewBuffer(otp),
		t:      toeplitz{diags: bitarray.NewDense(diags, -1), m: 40},
	}
	bob := &protoFramer{
		rw:     r,
		secret: bytes.NewBuffer(otp),
		t:      toeplitz{diags: bitarray.NewDense(diags, -1), m: 40},
	}
	msg := &bb84pb.BasisAnnouncement{
		Bases: &bb84pb.DenseBitArray{
			Bits: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
			Len:  70,
		},
		Dropped: &bb84pb.DenseBitArray{
			Bits: []byte{10, 11, 12, 13, 14, 15, 16, 17, 18, 19},
			Len:  70,
		},
	}
	msg2 := new(bb84pb.BasisAnnouncement)

	// net.Pipe() doesn't do any sort of buffering, so we perform these
	// operations asynchronously.
	wErr := make(chan error, 1)
	rErr := make(chan error, 1)
	go func() { wErr <- alice.Write(msg) }()
	go func() { rErr <- bob.Read(msg2) }()

	if err := <-wErr; err != nil {
		t.Fatalf("error writing message: %v", err)
	}
	if err := <-rErr; err != nil {
		t.Fatalf("error reading message: %v", err)
	}
	if !proto.Equal(msg2, msg) {
		t.Errorf("Message mangled in transit: got %v, want %v", msg2, msg)
	}
}

func TestMACVerification(t *testing.T) {
	l, r := net.Pipe()
	otp := make([]byte, 1024)
	otp2 := make([]byte, 1024)
	rand.Read(otp)
	rand.Read(otp2)
	diags := make([]byte, 1024)
	rand.Read(diags)
	alice := &protoFramer{
		rw:     l,
		secret: bytes.NewBuffer(otp),
		t:      toeplitz{diags: bitarray.NewDense(diags, -1), m: 40},
	}
	bob := &protoFramer{
		rw: r,
		// Note: otp2 != otp, so bob's MAC should disagree with alice's
		secret: bytes.NewBuffer(otp2),
		t:      toeplitz{diags: bitarray.NewDense(diags, -1), m: 40},
	}
	msg := &bb84pb.BasisAnnouncement{
		Bases: &bb84pb.DenseBitArray{
			Bits: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
			Len:  70,
		},
		Dropped: &bb84pb.DenseBitArray{
			Bits: []byte{10, 11, 12, 13, 14, 15, 16, 17, 18, 19},
			Len:  70,
		},
	}
	msg2 := new(bb84pb.BasisAnnouncement)

	// net.Pipe() doesn't do any sort of buffering, so we perform these
	// operations asynchronously.
	wErr := make(chan error, 1)
	rErr := make(chan error, 1)
	go func() { wErr <- alice.Write(msg) }()
	go func() { rErr <- bob.Read(msg2) }()

	if err := <-wErr; err != nil {
		t.Fatalf("Error writing message: %v", err)
	}
	if err := <-rErr; err == nil {
		t.Fatalf("Read of invalid MAC did not fail.")
	}
}
