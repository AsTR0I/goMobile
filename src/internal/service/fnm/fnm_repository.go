package fnm

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Fnm struct {
	ID      string
	Did     string
	NextHop string
}

type FnmRepository struct {
	mutex    sync.RWMutex
	fnms     []Fnm
	version  string
	lastLoad time.Time
}

type FnmMatchResult struct {
	A *Fnm
	B *Fnm
}

func NewFnmRepository() *FnmRepository {
	return &FnmRepository{}
}

func (r *FnmRepository) SetFnms(fnms []Fnm, version string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.fnms = fnms
	r.version = version
	r.lastLoad = time.Now()
}

func (r *FnmRepository) GetFnms() []Fnm {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.fnms
}

func (r *FnmRepository) GetVersion() string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.version
}

func (r *FnmRepository) GetLastLoadTime() time.Time {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.lastLoad
}

func (r *FnmRepository) FindFnmMatches(numA, numB, callID string) FnmMatchResult {
	logrus.Infof("Call-ID: %s — Searching FNM (A=%s, B=%s)", callID, numA, numB)

	r.mutex.RLock()
	defer r.mutex.RUnlock()

	res := FnmMatchResult{}

	if len(r.fnms) == 0 {
		logrus.Warnf("Call-ID: %s — No FNM records loaded", callID)
		return res
	}

	for _, f := range r.fnms {
		if f.Did == numA {
			logrus.Infof("Call-ID: %s — FNM A match: id=%s did=%s nexthop=%s",
				callID, f.ID, f.Did, f.NextHop)
			res.A = &f
		}

		if f.Did == numB {
			logrus.Infof("Call-ID: %s — FNM B match: id=%s did=%s nexthop=%s",
				callID, f.ID, f.Did, f.NextHop)
			res.B = &f
		}
	}

	if res.A == nil {
		logrus.Warnf("Call-ID: %s — No FNM match for NumA=%s", callID, numA)
	}
	if res.B == nil {
		logrus.Warnf("Call-ID: %s — No FNM match for NumB=%s", callID, numB)
	}

	return res
}
