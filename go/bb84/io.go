package bb84

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/alan-christopher/bb84/go/bb84/bitarray"
	"google.golang.org/protobuf/proto"
)

// A protoFramer reads and writes framed protocol buffers to the wire.
// The structure of the frame is trivial:  proto-length | proto | mac
//
// MACs are computed by applying a secret Toeplitz matrix to create a hash, then
// applying a one-time pad to the hash to allow for unconditional security. See
// also, https://arxiv.org/abs/1603.08387.
type protoFramer struct {
	rw     io.ReadWriter
	secret io.Reader
	t      toeplitz
}

func (p *protoFramer) Write(m proto.Message) error {
	marshalled, err := proto.Marshal(m)
	if err != nil {
		return err
	}
	if err := binary.Write(p.rw, binary.LittleEndian, int32(len(marshalled))); err != nil {
		return err
	}
	if _, err := p.rw.Write(marshalled); err != nil {
		return err
	}
	mac, err := p.buildMAC(marshalled)
	if err != nil {
		return err
	}
	if _, err := p.rw.Write(mac); err != nil {
		return err
	}
	return nil
}

func (p *protoFramer) Read(m proto.Message) error {
	var mLen int32
	if err := binary.Read(p.rw, binary.LittleEndian, &mLen); err != nil {
		return err
	}
	marshalled := make([]byte, mLen)
	if _, err := io.ReadFull(p.rw, marshalled); err != nil {
		return err
	}
	// TODO: avoid magic number
	// TODO: don't assume byte alignment
	mac := make([]byte, p.t.m/8)
	if _, err := io.ReadFull(p.rw, mac); err != nil {
		return err
	}
	emac, err := p.buildMAC(marshalled)
	if err != nil {
		return err
	}
	if !bytes.Equal(mac, emac) {
		return fmt.Errorf("invalid mac: got %v, expected %v", mac, emac)
	}

	return proto.Unmarshal(marshalled, m)
}

func (p *protoFramer) buildMAC(msg []byte) ([]byte, error) {
	// TODO: avoid magic number
	p.t.n = len(msg) * 8
	hash, err := p.t.Mul(bitarray.NewDense(msg, -1))
	if err != nil {
		return nil, err
	}
	otp := make([]byte, hash.ByteSize())
	if _, err := p.secret.Read(otp); err != nil {
		return nil, err
	}
	mac := hash.XOr(bitarray.NewDense(otp, -1))
	return mac.Data(), nil
}
