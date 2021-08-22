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
	"strings"

	"github.com/alan-christopher/bb84/go/bb84"
	"github.com/alan-christopher/bb84/go/bb84/bitmap"
	"github.com/alan-christopher/bb84/go/bb84/photon"
	flag "github.com/spf13/pflag"
)

var (
	qBatch = flag.IntSlice("qBatch", []int{bb84.DefaultMeasurementBatchBytes},
		"The bytes of raw quantum information to exchange per round of basis announcement.")
	nX    = flag.IntSlice("nX", []int{int(1e5)}, "The minimum number of main basis bits to accumulate before EC/priv amp/etc.")
	nZ    = flag.IntSlice("nZ", []int{int(5e4)}, "The minimum number of test basis bits to accumulate before EC/priv amp/etc.")
	pX    = flag.Float64Slice("pX", []float64{0.7}, "The probability of sending a bit in the main basis.")
	muLo  = flag.Float64Slice("muLo", []float64{0.05}, "The mean photons per pulse of the low intensity preparation.")
	muMed = flag.Float64Slice("muMed", []float64{0.1}, "The mean photons per pulse of the medium intensity preparation.")
	muHi  = flag.Float64Slice("muHi", []float64{0.3}, "The mean photons per pulse of the high intensity preparation.")
	pLo   = flag.Float64Slice("pLo", []float64{0.34}, "The proportion of low intensity photon pulses.")
	pMed  = flag.Float64Slice("pMed", []float64{0.33}, "The proportion of medium intensity photon pulses.")
	pHi   = flag.Float64Slice("pHi", []float64{0.33}, "The proportion of high intensity photon pulses.")
	qber  = flag.Float64Slice("qber", []float64{0.01}, "The qbers to observe when bases align.")
)

var (
	inputs = []string{"qBatch", "nX", "nZ", "pX", "muLo", "muMed", "muHi", "pLo", "pMed", "pHi", "qber"}
	// TODO: consider using reflection to pull this out of the Experiment data
	//   type.
	columns = []string{"QBatchBytes", "NX", "NZ", "PX", "MuLo", "MuMed", "MuHi",
		"PLo", "PMed", "PHi", "QBER", "Pulses", "QBits", "EmpiricalQBER", "KeyBits",
		"AliceMessages", "BobMessages", "AliceClassicalBytes", "BobClassicalBytes",
		"Succeeded"}
)

// An Experiment packages together the result of benchmarking a single
// parameterization for easy formatting.
type Experiment struct {
	// Fields corresponding to experiment parameters
	QBatchBytes       int
	NX, NZ            int
	PX                float64
	MuLo, MuMed, MuHi float64
	PLo, PMed, PHi    float64
	QBER              float64

	// Fields corresponding to experiment results
	Pulses              int
	QBits               int
	EmpiricalQBER       float64
	KeyBits             int
	AliceMessages       int
	BobMessages         int
	AliceClassicalBytes int
	BobClassicalBytes   int
	Succeeded           bool
}

func main() {
	flag.Parse()
	fmt.Println(header())
	tmpl := template.Must(template.New("line").Parse(lineTmpl()))
	var args [][]interface{}
	for _, inp := range inputs {
		args = append(args, lookupInput(inp))
	}
	applyCartesian(func(args []interface{}) {
		exp := &Experiment{
			QBatchBytes: args[inpIndex("qBatch")].(int),
			NX:          args[inpIndex("nX")].(int),
			NZ:          args[inpIndex("nZ")].(int),
			PX:          args[inpIndex("pX")].(float64),
			MuLo:        args[inpIndex("muLo")].(float64),
			MuMed:       args[inpIndex("muMed")].(float64),
			MuHi:        args[inpIndex("muHi")].(float64),
			PLo:         args[inpIndex("pLo")].(float64),
			PMed:        args[inpIndex("pMed")].(float64),
			PHi:         args[inpIndex("pHi")].(float64),
			QBER:        args[inpIndex("qber")].(float64),
		}
		if err := bench(exp); err != nil {
			log.Printf("Benching %v: %v", exp, err)
		}
		if err := tmpl.Execute(os.Stdout, exp); err != nil {
			log.Fatalf("BUG: could not fill in line template: %v", err)
		}
	}, args)
}

func inpIndex(v string) int {
	for i, inp := range inputs {
		if inp == v {
			return i
		}
	}
	return -1
}

func bench(exp *Experiment) error {
	l, r := net.Pipe()
	pa := bb84.PulseAttrs{}
	pa.MuLo, pa.MuMed, pa.MuHi = exp.MuLo, exp.MuMed, exp.MuHi
	pa.ProbLo, pa.ProbMed, pa.ProbHi = exp.PLo, exp.PMed, exp.PHi
	sender, receiver := photon.NewSimulatedChannel(
		exp.PX,                         // pMain
		pa.MuLo,                        // muLo
		pa.MuMed,                       // muMed
		pa.MuHi,                        // muHi
		pa.ProbLo,                      // pLo
		pa.ProbMed,                     // pMed
		pa.ProbHi,                      // pHi
		rand.New(rand.NewSource(1234)), // sendrand
		rand.New(rand.NewSource(5678)), // receiveRand
	)
	otp := make([]byte, 1<<23) // TODO: the amount of otp to create should be derived from experiment parameters
	rand.Read(otp)
	a, err := bb84.NewPeer(bb84.PeerOpts{
		Sender:           sender,
		ClassicalChannel: l,
		Rand:             rand.New(rand.NewSource(42)),
		Secret:           bytes.NewBuffer(otp),
		WinnowOpts: &bb84.WinnowOpts{
			Iters:    []int{3, 3, 3, 4, 6, 7, 7, 7},
			SyncRand: rand.New(rand.NewSource(17)),
		},
		PulseAttrs:            pa,
		MeasurementBatchBytes: exp.QBatchBytes,
		MainBlockSize:         exp.NX,
		TestBlockSize:         exp.NZ,
	})
	if err != nil {
		return err
	}
	b, err := bb84.NewPeer(bb84.PeerOpts{
		Receiver:         receiver,
		ClassicalChannel: r,
		Rand:             rand.New(rand.NewSource(1337)),
		Secret:           bytes.NewBuffer(otp),
		WinnowOpts: &bb84.WinnowOpts{
			Iters:    []int{3, 3, 3, 4, 6, 7, 7, 7},
			SyncRand: rand.New(rand.NewSource(17)),
		},
		PulseAttrs:            pa,
		MeasurementBatchBytes: exp.QBatchBytes,
		MainBlockSize:         exp.NX,
		TestBlockSize:         exp.NZ,
	})
	if err != nil {
		return err
	}
	batchBits := exp.QBatchBytes * 8
	legitErrs := bitmap.NewDense(nil, batchBits)
	for i := 0; i < int(float64(batchBits)*exp.QBER); i++ {
		legitErrs.Flip(i)
	}
	legitErrs.Shuffle(rand.New(rand.NewSource(99)))
	receiver.Errors = legitErrs.Data()

	go b.NegotiateKey()
	k, stats, err := a.NegotiateKey()
	exp.Pulses = stats.Pulses
	exp.QBits = stats.QBits
	exp.EmpiricalQBER = stats.QBER
	exp.KeyBits = k.Size()
	exp.AliceMessages = stats.MessagesSent
	exp.BobMessages = stats.MessagesSent
	exp.AliceClassicalBytes = stats.BytesSent
	exp.BobClassicalBytes = stats.BytesRead
	exp.Succeeded = err == nil
	return err
}

func header() string {
	return strings.Join(columns, ", ")
}

func lineTmpl() string {
	var els []string
	for _, c := range columns {
		els = append(els, "{{."+c+"}}")
	}
	return strings.Join(els, ", ") + "\n"
}

func lookupInput(name string) []interface{} {
	var r []interface{}
	if v, err := flag.CommandLine.GetIntSlice(name); err == nil {
		for _, val := range v {
			r = append(r, val)
		}
	} else if v, err := flag.CommandLine.GetFloat64Slice(name); err == nil {
		for _, val := range v {
			r = append(r, val)
		}
	} else {
		log.Fatalf("Unknown type for input %s", name)
	}
	return r
}

func applyCartesian(f func([]interface{}), args [][]interface{}) {
	for i := range args {
		if len(args[i]) == 1 {
			continue
		}
		l := make([][]interface{}, len(args))
		r := make([][]interface{}, len(args))
		copy(l, args)
		copy(r, args)
		l[i] = args[i][:1]
		r[i] = args[i][1:]
		applyCartesian(f, l)
		applyCartesian(f, r)
		return
	}
	x := make([]interface{}, 0, len(args))
	for _, a := range args {
		x = append(x, a[0])
	}
	f(x)
}
