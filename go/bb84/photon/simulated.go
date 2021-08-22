package photon

import (
	"math"
	"math/rand"

	"github.com/alan-christopher/bb84/go/bb84/bitmap"
)

// NewSimulatedChannel creates a pair of (Sender, Receiver) structs simulating a
// Quantum channel. It is expected that each call to Send() will be mirrored by
// a call to Receive(). Expect errors if that is not the case, and for calls to
// Send() to hang if more than 1 of them are made before a Receive().
func NewSimulatedChannel(pMain, muLo, muMed, muHi, pLo, pMed, pHi float64,
	sendRand, receiveRand *rand.Rand) (*SimulatedSender, *SimulatedReceiver) {
	bits := make(chan bitmap.Dense, 1)
	bases := make(chan bitmap.Dense, 1)
	drops := make(chan bitmap.Dense, 1)
	ss := &SimulatedSender{
		bits:  bits,
		bases: bases,
		drops: drops,
		muLo:  muLo,
		muMed: muMed,
		muHi:  muHi,
		pLo:   pLo,
		pMed:  pMed,
		pHi:   pHi,
		pMain: pMain,
		rand:  sendRand,
	}
	sr := &SimulatedReceiver{
		bits:  bits,
		bases: bases,
		drops: drops,
		pMain: pMain,
		rand:  receiveRand,
	}
	return ss, sr
}

type SimulatedSender struct {
	bits  chan<- bitmap.Dense
	bases chan<- bitmap.Dense
	drops chan<- bitmap.Dense

	pMain             float64
	muLo, muMed, muHi float64
	pLo, pMed, pHi    float64

	rand *rand.Rand
}

type SimulatedReceiver struct {
	Errors []byte
	Drops  []byte

	pMain float64
	bits  <-chan bitmap.Dense
	bases <-chan bitmap.Dense
	drops <-chan bitmap.Dense
	rand  *rand.Rand
}

func (ss *SimulatedSender) Next(bytes int) (bits, bases, lo, med, hi []byte, err error) {
	bits = make([]byte, bytes)
	ss.rand.Read(bits)

	baBases := bitmap.Empty()
	baLo := bitmap.Empty()
	baMed := bitmap.Empty()
	baHi := bitmap.Empty()
	drops := bitmap.Empty()
	pZ := 1 - ss.pMain
	for i := 0; i < bytes*8; i++ {
		baBases.AppendBit(ss.rand.Float64() < pZ)

		r := ss.rand.Float64()
		isLo := r < ss.pLo
		isMed := !isLo && r < ss.pLo+ss.pMed
		isHi := !isLo && !isMed
		baLo.AppendBit(isLo)
		baMed.AppendBit(isMed)
		baHi.AppendBit(isHi)

		mu := ss.muLo
		if isMed {
			mu = ss.muMed
		} else if isHi {
			mu = ss.muHi
		}
		drops.AppendBit(ss.rand.Float64() < math.Exp(-mu))
	}
	bases = baBases.Data()
	lo = baLo.Data()
	med = baMed.Data()
	hi = baHi.Data()
	ss.bits <- bitmap.NewDense(bits, -1)
	ss.bases <- baBases
	ss.drops <- drops
	return
}

func (sr *SimulatedReceiver) Next(bytes int) (bits, bases, dropped []byte, err error) {
	sendBits := <-sr.bits
	sendBases := <-sr.bases
	drops := <-sr.drops

	receiveBases := bitmap.Empty()
	pZ := 1 - sr.pMain
	for i := 0; i < sendBits.Size(); i++ {
		receiveBases.AppendBit(sr.rand.Float64() < pZ)
	}

	synthErrs, err := sr.resize(bitmap.NewDense(sr.Errors, -1), bytes*8)
	if err != nil {
		return nil, nil, nil, err
	}
	synthDrops, err := sr.resize(bitmap.NewDense(sr.Drops, -1), bytes*8)
	if err != nil {
		return nil, nil, nil, err
	}
	buf := make([]byte, sendBits.SizeBytes())
	rand.Read(buf)
	flips := bitmap.NewDense(buf, -1)
	flips = bitmap.And(flips, bitmap.XOr(sendBases, receiveBases))
	flips = bitmap.Or(flips, synthErrs)
	drops = bitmap.Or(drops, synthDrops)
	return bitmap.XOr(flips, sendBits).Data(), receiveBases.Data(), drops.Data(), nil
}

func (sr *SimulatedReceiver) resize(r bitmap.Dense, s int) (bitmap.Dense, error) {
	if r.Size() < s {
		r2 := bitmap.NewDense(nil, s-r.Size())
		r.Append(r2)
		return r, nil
	}
	if r.Size() > s {

		return bitmap.Slice(r, 0, s)
	}
	return r, nil
}
