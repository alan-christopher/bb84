package bb84

import (
	"errors"
	"fmt"
	"math"
	"math/rand"

	"github.com/alan-christopher/bb84/go/bb84/bitmap"
	"github.com/alan-christopher/bb84/go/bb84/photon"
	"github.com/alan-christopher/bb84/go/generated/bb84pb"
	"gonum.org/v1/gonum/stat/distuv"
)

// An alice represents the first BB84 participant.
type alice struct {
	sender         photon.Sender
	sideChannel    *protoFramer
	rand           *rand.Rand
	reconciler     reconciler
	measBatchBytes int
	epsPriv        float64
	epsCorrect     float64
	sampleProp     float64
	pulseAttrs     PulseAttrs
	nX             int
	nZ             int
}

// A bob represents the second BB84 participant.
type bob struct {
	receiver       photon.Receiver
	sideChannel    *protoFramer
	rand           *rand.Rand
	reconciler     reconciler
	measBatchBytes int
	epsPriv        float64
	epsCorrect     float64
	sampleProp     float64
	pulseAttrs     PulseAttrs
	nX             int
	nZ             int
}

type measurements struct {
	all, lo, med, hi bitmap.Dense
}

func (m *measurements) Append(o measurements) {
	m.all.Append(o.all)
	m.lo.Append(o.lo)
	m.med.Append(o.med)
	m.hi.Append(o.hi)
}

// NegotiateKey implements the Peer interface.
func (a *alice) NegotiateKey() (key bitmap.Dense, stats Stats, err error) {
	var main, test, errors measurements
	for main.all.Size() < a.nX || test.all.Size() < a.nZ {
		bits, bases, lo, med, hi, err := a.sendQBits()
		stats.Pulses += bits.Size()
		if err != nil {
			return bitmap.Empty(), stats, err
		}
		m, t, e, err := a.sift(bits, bases, lo, med, hi, &stats)
		if err != nil {
			return bitmap.Empty(), stats, err
		}
		stats.QBits += m.all.Size() + t.all.Size()
		main.Append(m)
		test.Append(t)
		errors.Append(e)
	}
	keyLen := calcSafeKeyLen(main, test, errors, a.pulseAttrs, a.epsPriv, a.epsCorrect, &stats)
	recRes, err := a.reconciler.Reconcile(main.all, &stats)
	if err != nil {
		return
	}
	if keyLen < recRes.bitsLeaked {
		err = fmt.Errorf("cannot make safe key: safe len == %d, ec loss == %d", keyLen, recRes.bitsLeaked)
		return
	}
	keyLen -= recRes.bitsLeaked
	// A reconciler that does privacy maintenance may have reduced our remaining
	// bits down below the safe key len.
	if keyLen > recRes.xHat.Size() {
		keyLen = recRes.xHat.Size()
	}
	seed, err := a.ecFinished(recRes.xHat, keyLen, &stats)
	if err != nil {
		return
	}
	key, err = hash(seed, recRes.xHat, keyLen)
	if err != nil {
		return
	}
	return
}

// NegotiateKey implements the Peer interface.
func (b *bob) NegotiateKey() (key bitmap.Dense, stats Stats, err error) {
	var main, test, errors measurements
	for main.all.Size() < b.nX || test.all.Size() < b.nZ {
		// TODO: In a realistic setup with non-ideal photon sources the vast
		//   majority of our pulses will be dropped, so we can reduce bandwidth
		//   by encoding dropped a sparse matrix of detected pulses.
		bits, bases, dropped, err := b.receiveQBits()
		stats.Pulses += bits.Size()
		if err != nil {
			return bitmap.Empty(), stats, err
		}
		m, t, e, err := b.sift(bits, bases, dropped, &stats)
		if err != nil {
			return bitmap.Empty(), stats, err
		}
		stats.QBits += m.all.Size() + t.all.Size()
		main.Append(m)
		test.Append(t)
		errors.Append(e)
	}
	keyLen := calcSafeKeyLen(main, test, errors, b.pulseAttrs, b.epsPriv, b.epsCorrect, &stats)
	recRes, err := b.reconciler.Reconcile(main.all, &stats)
	if err != nil {
		return
	}
	if keyLen < recRes.bitsLeaked {
		err = fmt.Errorf("cannot make safe key: safe len == %d, ec loss == %d", keyLen, recRes.bitsLeaked)
		return
	}
	keyLen -= recRes.bitsLeaked
	// A reconciler that does privacy maintenance may have reduced our remaining
	// bits down below the safe key len.
	if keyLen > recRes.xHat.Size() {
		keyLen = recRes.xHat.Size()
	}
	seed, err := b.ecFinished(recRes.xHat, &stats)
	if err != nil {
		return
	}
	key, err = hash(seed, recRes.xHat, keyLen)
	if err != nil {
		return
	}
	return
}

func (a *alice) sendQBits() (bits, bases, lo, med, hi bitmap.Dense, err error) {
	bitsArr, basesArr, loArr, medArr, hiArr, err := a.sender.Next(a.measBatchBytes)
	if err != nil {
		err = fmt.Errorf("sending qubits: %w", err)
		return
	}
	bits = bitmap.NewDense(bitsArr, -1)
	bases = bitmap.NewDense(basesArr, -1)
	lo = bitmap.NewDense(loArr, -1)
	med = bitmap.NewDense(medArr, -1)
	hi = bitmap.NewDense(hiArr, -1)
	return
}

func (b *bob) receiveQBits() (bits, bases, dropped bitmap.Dense, err error) {
	bitsArr, basesArr, droppedArr, err := b.receiver.Next(b.measBatchBytes)
	if err != nil {
		err = fmt.Errorf("receiving qubits: %w", err)
		return
	}
	bits = bitmap.NewDense(bitsArr, -1)
	bases = bitmap.NewDense(basesArr, -1)
	dropped = bitmap.NewDense(droppedArr, -1)
	return
}

func (a *alice) sift(bits, bases, lo, med, hi bitmap.Dense, s *Stats) (main, test, errors measurements, err error) {
	bba := new(bb84pb.BasisAnnouncement)
	if err = a.sideChannel.Read(bba, s); err != nil {
		err = fmt.Errorf("receiving basis announcement: %w", err)
		return
	}
	bBases := bitmap.DenseFromProto(bba.Bases)
	bTest := bitmap.DenseFromProto(bba.TestBits)
	received := bitmap.Not(bitmap.DenseFromProto(bba.Dropped))
	bits = bitmap.Select(bits, received)
	bases = bitmap.Select(bases, received)
	lo = bitmap.Select(lo, received)
	med = bitmap.Select(med, received)
	hi = bitmap.Select(hi, received)
	z := bitmap.And(bits, bases)
	aba := &bb84pb.BasisAnnouncement{
		Bases:    bases.ToProto(),
		TestBits: z.ToProto(),
		Lo:       lo.ToProto(),
		Med:      med.ToProto(),
		Hi:       hi.ToProto(),
	}
	if err = a.sideChannel.Write(aba, s); err != nil {
		err = fmt.Errorf("announcing bases: %w", err)
		return
	}
	main, test, errors = sift(bits, bTest, bases, bBases, lo, med, hi)
	return
}

func (b *bob) sift(bits, bases, dropped bitmap.Dense,
	s *Stats) (main, test, errors measurements, err error) {
	received := bitmap.Not(dropped)
	bits = bitmap.Select(bits, received)
	bases = bitmap.Select(bases, received)
	z := bitmap.And(bits, bases)
	bba := &bb84pb.BasisAnnouncement{
		Bases:    bases.ToProto(),
		Dropped:  dropped.ToProto(),
		TestBits: z.ToProto(),
	}
	if err = b.sideChannel.Write(bba, s); err != nil {
		err = fmt.Errorf("sending basis announcement: %w", err)
		return
	}
	aba := new(bb84pb.BasisAnnouncement)
	if err = b.sideChannel.Read(aba, s); err != nil {
		err = fmt.Errorf("receiving basis announcement: %w", err)
		return
	}
	aBasis := bitmap.DenseFromProto(aba.Bases)
	aTest := bitmap.DenseFromProto(aba.TestBits)
	lo := bitmap.DenseFromProto(aba.Lo)
	med := bitmap.DenseFromProto(aba.Med)
	hi := bitmap.DenseFromProto(aba.Hi)
	main, test, errors = sift(bits, aTest, bases, aBasis, lo, med, hi)
	return main, test, errors, nil
}

func (a *alice) ecFinished(k bitmap.Dense, targetLen int, s *Stats) (bitmap.Dense, error) {
	verLen := int(math.Ceil(math.Log2(1 / a.epsCorrect)))
	needed := k.Size() + verLen - 1
	verSeed := make([]byte, bitmap.BytesFor(needed))
	a.rand.Read(verSeed)
	ver, err := hash(bitmap.NewDense(verSeed, -1), k, verLen)
	if err != nil {
		return bitmap.Empty(), err
	}
	needed = k.Size() + targetLen - 1
	extractSeed := make([]byte, bitmap.BytesFor(needed))
	a.rand.Read(extractSeed)
	err = a.sideChannel.Write(&bb84pb.ErrorCorrectionFinished{
		ExtractSeed: extractSeed,
		VerifySeed:  verSeed,
		VerifyHash:  ver.ToProto(),
	}, s)
	if err != nil {
		return bitmap.Empty(), err
	}
	m := &bb84pb.ErrorCorrectionFinished{}
	if err := a.sideChannel.Read(m, s); err != nil {
		return bitmap.Empty(), err
	}
	if !bitmap.Equal(ver, bitmap.DenseFromProto(m.VerifyHash)) {
		return bitmap.Empty(), errors.New("error correction failed verification")
	}
	return bitmap.NewDense(extractSeed, -1), nil
}

func (b *bob) ecFinished(k bitmap.Dense, s *Stats) (bitmap.Dense, error) {
	m := &bb84pb.ErrorCorrectionFinished{}
	if err := b.sideChannel.Read(m, s); err != nil {
		return bitmap.Empty(), fmt.Errorf("receiving ec finished: %w", err)
	}
	aVerHash := bitmap.DenseFromProto(m.VerifyHash)
	ver, err := hash(bitmap.NewDense(m.VerifySeed, -1), k, aVerHash.Size())
	if err != nil {
		return bitmap.Empty(), fmt.Errorf("computing verification hash: %w", err)
	}
	err = b.sideChannel.Write(&bb84pb.ErrorCorrectionFinished{
		VerifyHash: ver.ToProto(),
	}, s)
	if err != nil {
		return bitmap.Empty(), fmt.Errorf("sending ec finished message: %w", err)
	}
	if !bitmap.Equal(ver, aVerHash) {
		return bitmap.Empty(), errors.New("error correction failed verification")
	}
	return bitmap.NewDense(m.ExtractSeed, -1), nil
}

func sift(bits, otherTest, basis, otherBasis, lo, med, hi bitmap.Dense) (main, test, errors measurements) {
	mainMask := bitmap.And(bitmap.Not(basis), bitmap.Not(otherBasis))
	testMask := bitmap.And(basis, otherBasis)
	main = measurements{
		all: bitmap.Select(bits, mainMask),
		lo:  bitmap.Select(bits, bitmap.And(mainMask, lo)),
		med: bitmap.Select(bits, bitmap.And(mainMask, med)),
		hi:  bitmap.Select(bits, bitmap.And(mainMask, hi)),
	}
	test = measurements{
		all: bitmap.Select(bits, testMask),
		lo:  bitmap.Select(bits, bitmap.And(testMask, lo)),
		med: bitmap.Select(bits, bitmap.And(testMask, med)),
		hi:  bitmap.Select(bits, bitmap.And(testMask, hi)),
	}
	other := measurements{
		lo:  bitmap.Select(otherTest, bitmap.And(testMask, lo)),
		med: bitmap.Select(otherTest, bitmap.And(testMask, med)),
		hi:  bitmap.Select(otherTest, bitmap.And(testMask, hi)),
	}
	errors = measurements{
		lo:  bitmap.XOr(test.lo, other.lo),
		med: bitmap.XOr(test.med, other.med),
		hi:  bitmap.XOr(test.hi, other.hi),
	}
	return main, test, errors
}

func hash(seed bitmap.Dense, x bitmap.Dense, outlen int) (bitmap.Dense, error) {
	t := toeplitz{
		diags: seed,
		m:     outlen,
		n:     x.Size(),
	}
	return t.Mul(x)
}

// Computes $l + \lambda_{EC}$, as per
// https://journals.aps.org/pra/abstract/10.1103/PhysRevA.89.022307
func calcSafeKeyLen(main, test, errors measurements,
	pulseAttrs PulseAttrs,
	epsPriv, epsCorrect float64,
	stats *Stats) int {
	sX0 := estimateVacuumCount(main, pulseAttrs, epsPriv)
	sX1 := estimateSinglePhotonCount(main, pulseAttrs, epsPriv, sX0)
	phiX, mZ := estimatePhaseErrorRate(main, test, errors, pulseAttrs, epsPriv, sX1)
	l := sX0 + sX1 - sX1*binaryEntropy(phiX) - 6*math.Log2(21/epsPriv) - math.Log2(2/epsCorrect)
	stats.QBER = float64(mZ) / float64(test.all.Size())
	return int(math.Floor(l))
}

func estimatePhaseErrorRate(main, test, errors measurements,
	pa PulseAttrs, eps, sX1 float64) (phi float64, mZ int) {
	mu2, mu3 := pa.MuMed, pa.MuLo
	p2, p3 := pa.ProbMed, pa.ProbLo
	sZ0 := estimateVacuumCount(test, pa, eps)
	sZ1 := estimateSinglePhotonCount(test, pa, eps, sZ0)
	mZ1 := bitmap.CountOnes(errors.hi)
	mZ2 := bitmap.CountOnes(errors.med)
	mZ3 := bitmap.CountOnes(errors.lo)
	mZ = mZ1 + mZ2 + mZ3
	mZ2Plus := hoeffding(mu2, p2, eps, float64(mZ), float64(mZ2), 1)
	mZ3Minus := hoeffding(mu3, p3, eps, float64(mZ), float64(mZ3), -1)
	tau1 := probNPhotons(pa, 1)
	nuZ1 := tau1 * (mZ2Plus - mZ3Minus) / (mu2 - mu3)
	return nuZ1/sZ1 + gamma(eps, nuZ1/sZ1, sZ1, sX1), mZ
}

func estimateVacuumCount(meas measurements, pa PulseAttrs, eps float64) float64 {
	mu2, mu3 := pa.MuMed, pa.MuLo
	p2, p3 := pa.ProbMed, pa.ProbLo
	tau0 := probNPhotons(pa, 0)
	nX1 := meas.hi.Size()
	nX2 := meas.med.Size()
	nX3 := meas.lo.Size()
	nX := float64(nX1 + nX2 + nX3)
	nX3Minus := hoeffding(mu3, p3, eps, nX, float64(nX3), -1)
	nX2Plus := hoeffding(mu2, p2, eps, nX, float64(nX2), 1)
	ret := tau0 * (mu2*nX3Minus - mu3*nX2Plus) / (mu2 - mu3)
	// The preceding calculation gives a lower bound on the number of detection
	// events we had in response to 0-photon pulses, and negative values are
	// technically valid. However, 0 is also a valid bound, and in such cases,
	// it's tighter.
	if ret < 0 {
		return 0
	}
	return ret
}

func estimateSinglePhotonCount(meas measurements, pa PulseAttrs, eps, s0 float64) float64 {
	mu1, mu2, mu3 := pa.MuHi, pa.MuMed, pa.MuLo
	p1, p2, p3 := pa.ProbHi, pa.ProbMed, pa.ProbLo
	tau0 := probNPhotons(pa, 0)
	tau1 := probNPhotons(pa, 1)
	n1, n2, n3 := meas.hi.Size(), meas.med.Size(), meas.lo.Size()
	n := n1 + n2 + n3
	n2Minus := hoeffding(mu2, p2, eps, float64(n), float64(n2), -1)
	n3Plus := hoeffding(mu3, p3, eps, float64(n), float64(n3), 1)
	n1Plus := hoeffding(mu1, p1, eps, float64(n), float64(n1), 1)
	num := tau1 * mu1 * (n2Minus - n3Plus - (mu2*mu2-mu3*mu3)*(n1Plus-s0/tau0)/mu1/mu1)
	denom := mu1*(mu2-mu3) - mu2*mu2 + mu3*mu3
	return num / denom
}

func probNPhotons(pa PulseAttrs, n float64) float64 {
	hi := distuv.Poisson{Lambda: pa.MuHi}
	med := distuv.Poisson{Lambda: pa.MuMed}
	lo := distuv.Poisson{Lambda: pa.MuLo}
	return pa.ProbHi*hi.Prob(n) + pa.ProbMed*med.Prob(n) + pa.ProbLo*lo.Prob(n)
}

func binaryEntropy(x float64) float64 {
	return -x*math.Log2(x) - (1-x)*math.Log2(1-x)
}

func hoeffding(mu, p, eps, nX, nXk, sign float64) float64 {
	return (math.Exp(mu) / p) * (nXk + sign*math.Sqrt(nX*math.Log(21/eps)/2))
}

func gamma(a, b, c, d float64) float64 {
	term1 := (c + d) * (1 - b) * b / c / d / math.Log(2)
	term2 := (21 / a) * (21 / a) * (c + d) / (c * d * (1 - b) * b)
	return math.Sqrt(term1 * math.Log2(term2))
}
