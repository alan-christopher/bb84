// bench.go runs a round of BB84 key negotiation for each entry in the cartesion
// product of a collection of different tuning parameters, e.g. observed error
// rate and quantum bits exchanged, and outputs a CSV of relevant statistics for
// each different combination, e.g. messages exchanged and final key length.
package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net"
	"os"

	"github.com/alan-christopher/bb84/go/bb84"
	"github.com/alan-christopher/bb84/go/bb84/bitarray"
	"github.com/alan-christopher/bb84/go/bb84/photon"
	flag "github.com/spf13/pflag"
)

// TODO: support more dimensions for experimentation.
var (
	qbytes = flag.IntSlice("qbytes", []int{4096},
		"The bytes of raw quantum information to exchange during a round of key negotiation.")
	qbers = flag.Float64Slice("qbers", []float64{0.05},
		"The qbers to observe when bases align.")
)

const (
	header   = "QBits, QBER, KeySize, AliceMessages, BobMessages, AliceClassicalBytes, BobClassicalBytes"
	lineTmpl = "{{.QBits}}, {{.QBER}}, {{.KeySize}}, {{.AliceMessages}}, {{.BobMessages}}, {{.AliceClassicalBytes}}, {{.BobClassicalBytes}}\n"
)

// A Result packages together the result of benchmarking a single
// parameterization for easy formatting.
type Result struct {
	QBits               int
	QBER                float64
	KeySize             int
	AliceMessages       int
	BobMessages         int
	AliceClassicalBytes int
	BobClassicalBytes   int
}

func main() {
	flag.Parse()
	fmt.Println(header)
	tmpl := template.Must(template.New("line").Parse(lineTmpl))
	for _, qbyte := range *qbytes {
		for _, qber := range *qbers {
			r, err := bench(qbyte, qber)
			if err != nil {
				log.Fatalf("Benching (qbyte: %d, qber: %f): %v", qbyte, qber, err)
			}
			tmpl.Execute(os.Stdout, r)
		}
	}
}

func bench(qbytes int, qber float64) (Result, error) {
	l, r := net.Pipe()
	sender, receiver := photon.NewSimulatedChannel(0)
	otp := make([]byte, 1<<20) // TODO: this
	rand.Read(otp)
	a, err := bb84.NewPeer(bb84.PeerOpts{
		QBytes:           qbytes,
		Sender:           sender,
		ClassicalChannel: l,
		Rand:             rand.New(rand.NewSource(42)),
		Secret:           bytes.NewBuffer(otp),
		WinnowOpts: &bb84.WinnowOpts{
			Iters:    []int{3, 3, 3, 4, 6, 7, 7, 7},
			SyncRand: rand.New(rand.NewSource(17)),
		},
	})
	if err != nil {
		return Result{}, nil
	}
	b, err := bb84.NewPeer(bb84.PeerOpts{
		QBytes:           qbytes,
		Receiver:         receiver,
		ClassicalChannel: r,
		Rand:             rand.New(rand.NewSource(1337)),
		Secret:           bytes.NewBuffer(otp),
		WinnowOpts: &bb84.WinnowOpts{
			Iters:    []int{3, 3, 3, 4, 6, 7, 7, 7},
			SyncRand: rand.New(rand.NewSource(17)),
		},
	})
	if err != nil {
		return Result{}, nil
	}
	legitErrs := bitarray.NewDense(nil, qbytes*8)
	for i := 0; i < int(float64(qbytes*8)*qber); i++ {
		legitErrs.Set(i, true)
	}
	legitErrs.Shuffle(rand.New(rand.NewSource(99)))
	receiver.Errors = legitErrs.Data()

	go b.NegotiateKey()
	k, stats, err := a.NegotiateKey()
	return Result{
		QBits:               qbytes * 8,
		QBER:                stats.QBER,
		KeySize:             k.Size(),
		AliceMessages:       stats.MessagesSent,
		BobMessages:         stats.MessagesReceived,
		AliceClassicalBytes: stats.BytesSent,
		BobClassicalBytes:   stats.BytesRead,
	}, err
}
