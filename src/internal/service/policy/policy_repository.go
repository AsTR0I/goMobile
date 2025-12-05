package policy

import (
	"net"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

type Policy struct {
	ID           int
	State        int
	Description  string
	Priority     int
	NumA         *regexp.Regexp
	NumB         *regexp.Regexp
	NumC         *regexp.Regexp
	SrcIP        []*net.IPNet
	SbcIP        []*net.IPNet
	PeriodStart  int64
	PeriodStop   int64
	Target       string
	MatchCounter int32
}

type PolicyRepository struct {
	mutex    sync.RWMutex
	policies []Policy
	version  string
	lastLoad time.Time
}

func NewPolicyRepository() *PolicyRepository {
	return &PolicyRepository{}
}

func (r *PolicyRepository) SetPolicies(policies []Policy, version string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.policies = policies
	r.version = version
	r.lastLoad = time.Now()
}

func (r *PolicyRepository) GetPolicies() []Policy {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.policies
}

func (r *PolicyRepository) GetVersion() string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.version
}

func (r *PolicyRepository) GetLastLoadTime() time.Time {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.lastLoad
}

func (r *PolicyRepository) FindBestPolicy(numA, numB, numC string, unixTime int64, srcIP, sbcIP, callID string) *Policy {
	start := time.Now()

	type result struct {
		policy   *Policy
		index    int
		priority int
	}

	results := make(chan result, len(r.policies))
	var wg sync.WaitGroup

	for i, p := range r.policies {
		wg.Add(1)
		go func(idx int, pol Policy) {
			defer wg.Done()

			if unixTime < pol.PeriodStart || unixTime > pol.PeriodStop {
				return
			}
			if !ipInRange(srcIP, pol.SrcIP, callID) || !ipInRange(sbcIP, pol.SbcIP, callID) {
				return
			}
			if !pol.NumA.MatchString(numA) || !pol.NumB.MatchString(numB) || !pol.NumC.MatchString(numC) {
				return
			}

			results <- result{
				policy:   &pol,
				index:    idx,
				priority: pol.Priority,
			}
		}(i, p)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var best *Policy
	maxPriority := -1

	for res := range results {
		polPtr := &r.policies[res.index]
		if res.priority > maxPriority {
			maxPriority = res.priority
			best = polPtr
		}
	}

	if best != nil {
		atomic.AddInt32(&best.MatchCounter, 1)
		elapsed := float64(time.Since(start).Microseconds()) / 1000.0
		logrus.Infof("Call-ID: %s — Best policy ID %d found with priority %d, search time %.3f ms", callID, best.ID, best.Priority, elapsed)
	}

	return best
}

func ipInRange(ipStr string, ranges []*net.IPNet, callID string) bool {
	start := time.Now()

	ip := net.ParseIP(ipStr)
	if ip == nil {
		logrus.Infof("Call-ID: %s — Invalid IP: %s", callID, ipStr)
		return false
	}

	for _, ipNet := range ranges {
		logrus.Debugf("Call-ID: %s — Checking if IP %s is in range %s", callID, ipStr, ipNet.String())

		if ipNet.Contains(ip) {
			logrus.Debugf("Call-ID: %s — IP %s matched range %s", callID, ipStr, ipNet.String())

			elapsed := float64(time.Since(start).Microseconds()) / 1000.0
			logrus.Debugf("Call-ID: %s — ipInRange() search time: %.3f ms", callID, elapsed)

			return true
		}
	}

	elapsed := float64(time.Since(start).Microseconds()) / 1000.0
	logrus.Debugf("Call-ID: %s — IP %s did not match any provided ranges. ipInRange() search time: %.3f ms", callID, ipStr, elapsed)

	return false
}
