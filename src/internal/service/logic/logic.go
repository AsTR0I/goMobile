package logic

import (
	"fmt"
	"gomobile/internal/service/fnm"
	"gomobile/internal/service/policy"
	"gomobile/internal/types"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type BusinessLogic struct {
	policies *policy.PolicyRepository
	fnm      *fnm.FnmRepository
}

func NewBusinessLogic(
	p *policy.PolicyRepository,
	f *fnm.FnmRepository,
) *BusinessLogic {
	return &BusinessLogic{
		policies: p,
		fnm:      f,
	}
}

func (bl *BusinessLogic) FindPolicyResult(
	numA, numB, numC, srcIP, sbcIP, callID, ruri string, unixTime int64,
) types.BusinessLogicResult {

	startTotal := time.Now()
	logrus.Debugf("Call-ID: %s — Start FindPolicyResult", callID)

	// ───────────────────────────────────────────────────────────
	ts := time.Now()
	logrus.Infof("Call-ID: %s — Finding policy for numA=%s numB=%s numC=%s",
		callID, numA, numB, numC)

	bestPolicy := bl.policies.FindBestPolicy(numA, numB, numC, unixTime, srcIP, sbcIP, callID)
	logrus.Debugf("Call-ID: %s — Time FindBestPolicy: %.3fms", callID, ms(time.Since(ts)))

	if bestPolicy == nil {
		logrus.Warnf("Call-ID: %s — No matching policy found", callID)
		return types.BusinessLogicResult{
			Target: "Bad Gateway",
			Reason: "Policies not found",
			ID:     0,
		}
	}

	logrus.Infof("Call-ID: %s — Best policy found: ID=%d Target=%s Priority=%d",
		callID, bestPolicy.ID, bestPolicy.Target, bestPolicy.Priority)

	// ───────────────────────────────────────────────────────────
	ts = time.Now()
	msidnNumA := bl.fnm.FindFnm(numA, callID)
	logrus.Debugf("Call-ID: %s — Time FindFnm A: %.3fms", callID, ms(time.Since(ts)))

	if msidnNumA != nil {
		logrus.Infof("Call-ID: %s — FNM A: %s internal - %s",
			callID, numA, msidnNumA.InternalNumber)
	} else {
		logrus.Warnf("Call-ID: %s — FNM A not found for %s", callID, numA)
	}

	// ───────────────────────────────────────────────────────────
	ts = time.Now()
	msidnNumB := bl.fnm.FindFnm(numB, callID)
	logrus.Debugf("Call-ID: %s — Time FindFnm B: %.3fms", callID, ms(time.Since(ts)))

	if msidnNumB != nil {
		logrus.Infof("Call-ID: %s — FNM B: %s internal - %s",
			callID, numB, msidnNumB.InternalNumber)
	} else {
		logrus.Warnf("Call-ID: %s — FNM B not found for %s", callID, numB)
	}

	target := bestPolicy.Target
	logrus.Infof("Call-ID: %s — Initial target: %s", callID, target)

	// ───────────────────────────────────────────────────────────
	replaceOrFail := func(placeholder string, getter func() string) (string, *types.BusinessLogicResult) {
		tsInner := time.Now()

		if !strings.Contains(target, placeholder) {
			return target, nil
		}
		value := getter()
		if value == "" {
			logrus.Errorf("Call-ID: %s — Failed to resolve placeholder %s", callID, placeholder)
			return "", &types.BusinessLogicResult{
				Target: "Bad Gateway",
				Reason: fmt.Sprintf("Cannot resolve variable %s", placeholder),
				ID:     0,
			}
		}

		logrus.Infof("Call-ID: %s — Replacing placeholder %s with %s",
			callID, placeholder, value)

		newTarget := replaceIgnoreCase(target, placeholder, value)
		logrus.Debugf("Call-ID: %s — Time replace(%s): %.3fms",
			callID, placeholder, ms(time.Since(tsInner)))

		return newTarget, nil
	}

	var errResult *types.BusinessLogicResult

	// далее замеры каждого replace
	ts = time.Now()
	target, errResult = replaceOrFail("%a_int%", func() string {
		if msidnNumA == nil {
			return ""
		}
		return msidnNumA.InternalNumber
	})
	logrus.Debugf("Call-ID: %s — Time placeholder a_int: %.3fms", callID, ms(time.Since(ts)))
	if errResult != nil {
		return *errResult
	}

	ts = time.Now()
	target, errResult = replaceOrFail("%a%", func() string { return numA })
	logrus.Debugf("Call-ID: %s — Time placeholder a: %.3fms", callID, ms(time.Since(ts)))
	if errResult != nil {
		return *errResult
	}

	ts = time.Now()
	target, errResult = replaceOrFail("%b%", func() string { return numB })
	logrus.Debugf("Call-ID: %s — Time placeholder b: %.3fms", callID, ms(time.Since(ts)))
	if errResult != nil {
		return *errResult
	}

	ts = time.Now()
	target, errResult = replaceOrFail("%b_int%", func() string {
		if msidnNumB == nil {
			return ""
		}
		return msidnNumB.InternalNumber
	})
	logrus.Debugf("Call-ID: %s — Time placeholder b_int: %.3fms", callID, ms(time.Since(ts)))
	if errResult != nil {
		return *errResult
	}

	ts = time.Now()
	target, errResult = replaceOrFail("%shared_did%", func() string { return numB })
	logrus.Debugf("Call-ID: %s — Time placeholder shared_did: %.3fms", callID, ms(time.Since(ts)))
	if errResult != nil {
		return *errResult
	}

	ts = time.Now()
	target, errResult = replaceOrFail("%access_code%", func() string {
		if msidnNumB == nil && msidnNumA == nil {
			return ""
		}
		if len(numB) > 2 && len(numB) <= 5 {
			return msidnNumA.Tenant.Account.AccessCode
		}
		if msidnNumB != nil {
			return msidnNumB.Tenant.Account.AccessCode
		}
		return ""
	})
	logrus.Debugf("Call-ID: %s — Time placeholder access_code: %.3fms", callID, ms(time.Since(ts)))
	if errResult != nil {
		return *errResult
	}

	ts = time.Now()
	target, errResult = replaceOrFail("%node_ip%", func() string {
		if msidnNumB == nil && msidnNumA == nil {
			return ""
		}
		if len(numB) > 2 && len(numB) <= 5 {
			return getNodeIp(msidnNumA.Tenant.Service.Node, callID)
		}
		if msidnNumB != nil {
			return getNodeIp(msidnNumB.Tenant.Service.Node, callID)
		}
		return ""
	})
	logrus.Debugf("Call-ID: %s — Time placeholder node_ip: %.3fms", callID, ms(time.Since(ts)))
	if errResult != nil {
		return *errResult
	}

	ts = time.Now()
	target, errResult = replaceOrFail("%ruri%", func() string { return ruri })
	logrus.Debugf("Call-ID: %s — Time placeholder ruri: %.3fms", callID, ms(time.Since(ts)))
	if errResult != nil {
		return *errResult
	}

	logrus.Infof("Call-ID: %s — Final target: %s", callID, target)

	logrus.Debugf("Call-ID: %s — Total FindPolicyResult time: %3.fms",
		callID, ms(time.Since(startTotal)))

	target = strings.TrimSpace(target)

	return types.BusinessLogicResult{
		Target:   target,
		Reason:   "",
		Priority: bestPolicy.Priority,
		ID:       bestPolicy.ID,
	}
}

// old version replaceIgnoreCase using regexp
func replaceIgnoreCase(target, placeholder, value string) string {
	if value == "" {
		return target
	}
	re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(placeholder))
	return re.ReplaceAllString(target, value)
}

// func replaceIgnoreCase(target, placeholder, value string) string {
// 	if value == "" {
// 		return target
// 	}

// 	tl := strings.ToLower(target)
// 	pl := strings.ToLower(placeholder)

// 	var result strings.Builder
// 	result.Grow(len(target))

// 	i := 0
// 	for i < len(target) {
// 		if strings.HasPrefix(tl[i:], pl) {
// 			result.WriteString(value)
// 			i += len(placeholder)
// 		} else {
// 			result.WriteByte(target[i])
// 			i++
// 		}
// 	}

// 	return result.String()
// }

func getNodeIp(nodeIp string, callID string) string {
	if nodeIp == "" {
		logrus.Warnf("Call-ID: %s — Node IP is empty", callID)
	}
	return nodeIp + ".cocobri.ru"
}

func ms(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}
