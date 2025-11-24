package logic

import (
	"context"
	"encoding/json"
	"gomobile/internal/service/db"
	"gomobile/internal/service/fnm"
	"gomobile/internal/service/policy"
	"gomobile/internal/types"
	"log"
	"regexp"
	"strings"
)

type BusinessLogic struct {
	policies *policy.PolicyRepository
	fnm      *fnm.FnmRepository
	db       db.Storage
}

func NewBusinessLogic(
	p *policy.PolicyRepository,
	f *fnm.FnmRepository,
	db db.Storage,
) *BusinessLogic {
	return &BusinessLogic{
		policies: p,
		fnm:      f,
		db:       db,
	}
}

func (b *BusinessLogic) FindPolicyResult(numA, numB, numC, srcIP, sbcIP, callID string, unixTime int64) types.BusinessLogicResult {
	// ctx := context.Background()

	// var node string

	simA, err := b.db.GetFullSimDataByDid(context.Background(), numA)
	if err != nil {
		log.Printf("Error fetching SIM A data for DID %s: %v", numA, err)
	}

	simB, err := b.db.GetFullSimDataByDid(context.Background(), numB)
	if err != nil {
		log.Printf("Error fetching SIM B data for DID %s: %v", numB, err)
	}

	bestPolicy := b.policies.FindBestPolicy(numA, numB, numC, unixTime, srcIP, sbcIP, callID, simA, simB)
	if bestPolicy == nil {
		return types.BusinessLogicResult{
			Target: "Bad Gateway",
			Reason: "Policies not found",
			ID:     0,
		}
	}

	target := bestPolicy.Target

	target = replaceIgnoreCase(target, "%A%", numA)
	target = replaceIgnoreCase(target, "%B%", numB)
	target = replaceIgnoreCase(target, "%C%", numC)

	if simB != nil && simB.Account != nil && simB.Account.VirtualPbx != nil {
		target = replaceIgnoreCase(target, "%PBX_VOICE%", simB.Account.VirtualPbx.VoiceNumber)
		target = replaceIgnoreCase(target, "%PBX_ACCESS%", simB.Account.VirtualPbx.AccessCode)
	}

	node := ""
	if simB != nil && simB.Account != nil && simB.Account.NodeID != nil {
		node = strings.ToUpper(*simB.Account.NodeID)
	}
	target = replaceIgnoreCase(target, "%NODE_IP%", node)
	target = strings.Trim(target, `"'`)

	return types.BusinessLogicResult{
		Target:   target,
		Reason:   "",
		Priority: bestPolicy.Priority,
		ID:       bestPolicy.ID,
	}
}

func pretty(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func replaceIgnoreCase(target, placeholder, value string) string {
	if value == "" {
		return target
	}
	re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(placeholder))
	return re.ReplaceAllString(target, value)
}
