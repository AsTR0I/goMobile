package policy

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

type PolicyLoader struct {
	repo *PolicyRepository
}

func NewPolicyLoader(repo *PolicyRepository) *PolicyLoader {
	return &PolicyLoader{repo: repo}
}

func (l *PolicyLoader) LoadLatestFromDir(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.csv"))
	if err != nil {
		return fmt.Errorf("failed to list csv files: %v", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no csv files found in %s", dir)
	}

	sort.Strings(files)
	latestFile := files[len(files)-1]
	policies, err := l.parseCSV(latestFile)
	if err != nil {
		return fmt.Errorf("failed to parse csv %s: %v", latestFile, err)
	}

	l.repo.SetPolicies(policies, filepath.Base(latestFile))
	return nil
}

func (l *PolicyLoader) parseCSV(filePath string) ([]Policy, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	var policies []Policy

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if i == 0 || line == "" {
			continue
		}

		fields := strings.Split(line, ";")
		if len(fields) < 15 {
			continue
		}

		id, _ := strconv.Atoi(fields[0])
		state, _ := strconv.Atoi(fields[1])
		priority, _ := strconv.Atoi(fields[2])
		description := fields[3]

		numA, _ := regexp.Compile(fields[4])
		numB, _ := regexp.Compile(fields[5])
		numC, _ := regexp.Compile(fields[6])

		periodStart, _ := strconv.ParseInt(fields[7], 10, 64)
		periodStop, _ := strconv.ParseInt(fields[8], 10, 64)

		srcIP := parseIPRanges(fields[9])
		sbcIP := parseIPRanges(fields[10])

		srcType := fields[11]

		var requireSimA, requireSimB *bool
		if fields[12] != "" {
			b, _ := strconv.Atoi(fields[12])
			bb := b != 0
			requireSimA = &bb
		}
		if fields[13] != "" {
			b, _ := strconv.Atoi(fields[13])
			bb := b != 0
			requireSimB = &bb
		}

		operatorB := ""
		if len(fields) > 14 {
			operatorB = fields[14]
		}

		target := ""
		if len(fields) > 15 {
			target = strings.Join(fields[15:], ";")
		}

		policies = append(policies, Policy{
			ID:          id,
			State:       state,
			Description: description,
			Priority:    priority,
			NumA:        numA,
			NumB:        numB,
			NumC:        numC,
			SrcIP:       srcIP,
			SbcIP:       sbcIP,
			PeriodStart: periodStart,
			PeriodStop:  periodStop,
			Target:      target,
			SrcType:     srcType,
			RequireSimA: requireSimA,
			RequireSimB: requireSimB,
			OperatorB:   operatorB,
		})
	}

	return policies, nil
}

func compileRegexp(pattern string) (*regexp.Regexp, error) {
	return regexp.Compile(pattern)
}

// parseIPRanges парсит CIDR диапазоны, добавляет /32 если нужно
func parseIPRanges(field string) []*net.IPNet {
	var ranges []*net.IPNet
	cidrs := strings.Split(field, "|")
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		if !strings.Contains(cidr, "/") {
			cidr += "/32"
		}
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			logrus.Warnf("Invalid CIDR '%s': %v", cidr, err)
			continue
		}
		ranges = append(ranges, ipnet)
	}
	return ranges
}
