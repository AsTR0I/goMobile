package policy

import (
	"gomobile/internal/service/db"
	"net"
	"regexp"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Policy struct {
	ID          int
	State       int
	Description string
	Priority    int
	NumA        *regexp.Regexp
	NumB        *regexp.Regexp
	NumC        *regexp.Regexp
	SrcIP       []*net.IPNet
	SbcIP       []*net.IPNet
	PeriodStart int64
	PeriodStop  int64
	Target      string

	SrcType     string // "mcn"
	RequireSimA *bool  // nil=ignore, true=должен быть, false=не должен быть
	RequireSimB *bool
	OperatorB   string // "MCN"
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

func (r *PolicyRepository) FindBestPolicy(numA, numB, numC string, unixTime int64, srcIP, sbcIP, callID string, simA, simB *db.Simcard) *Policy {
	logrus.Infof("Call-ID: %s — Searching best policy for NumA=%s, NumB=%s, NumC=%s, Time=%d, SrcIP=%s, SbcIP=%s",
		callID, numA, numB, numC, unixTime, srcIP, sbcIP)

	logrus.Infof("Call-ID: %s — Total policies to evaluate: %d", callID, len(r.policies))
	logrus.Debugf("Call-ID: %s — Policies detail: %+v", callID, r.policies)
	for i, p := range r.policies {
		if unixTime < p.PeriodStart || unixTime > p.PeriodStop {
			logrus.Debugf("Call-ID: %s — Skipping policy ID %d: outside active period (%d - %d)", callID, p.ID, p.PeriodStart, p.PeriodStop)
			continue
		}

		if !p.NumA.MatchString(numA) || !p.NumB.MatchString(numB) || !p.NumC.MatchString(numC) || !ipInRange(srcIP, p.SrcIP, callID) || !ipInRange(sbcIP, p.SbcIP, callID) {
			logrus.Debugf("Call-ID: %s — Skipping policy ID %d", callID, p.ID)
			continue
		}

		logrus.Infof("Call-ID: %s — Found matching policy ID %d", callID, p.ID)
		return &r.policies[i]
	}

	logrus.Warnf("Call-ID: %s — No matching policy found", callID)
	return nil
}

func ipInRange(ipStr string, ranges []*net.IPNet, callID string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		logrus.Infof("Call-ID: %s — Invalid IP: %s", callID, ipStr)
		return false
	}

	for _, ipNet := range ranges {
		logrus.Debug("Call-ID: %s — Checking if IP %s is in range %s", callID, ipStr, ipNet.String())

		if ipNet.Contains(ip) {
			logrus.Debug("Call-ID: %s — IP %s matched range %s", callID, ipStr, ipNet.String())
			return true
		}
	}

	logrus.Debug("Call-ID: %s — IP %s did not match any provided ranges", callID, ipStr)
	return false
}
