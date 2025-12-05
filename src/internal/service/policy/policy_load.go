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

// ----- Доделать -----
// func (l *PolicyLoader) LoadFromAPI(url string, saveDir string) error {
// 	if err := l.ensureDir(saveDir); err != nil {
// 		return err
// 	}
// 	logrus.Infof("Loading Policy from API: %s", url)

// 	resp, err := http.Get(url)
// 	if err != nil {
// 		return fmt.Errorf("failed to GET api: %v", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != 200 {
// 		return fmt.Errorf("bad status: %v", resp.StatusCode)
// 	}

// 	body, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		return fmt.Errorf("failed to read body: %v", err)
// 	}

// 	var policies []Policy

// 	version := time.Now().Format("20060102_150405")
// 	filePath := filepath.Join(saveDir, fmt.Sprintf("%s.csv", version))

// 	if err := os.WriteFile(filePath, body, 0644); err != nil {
// 		logrus.Warnf("failed to save FNM JSON: %v", err)
// 	}

// 	for i := range policies {
// 	}

// 	l.repo.SetPolicies(policies, version)

// 	logrus.Infof("Loaded %d Policies from API", len(policies))
// 	return nil
// }

func (l *PolicyLoader) LoadLatestFromDir(dir string) error {
	if err := l.ensureDir(dir); err != nil {
		return err
	}
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
		if len(fields) < 11 {
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

		target := ""
		if len(fields) > 11 {
			target = strings.Join(fields[11:], ";")
		}

		policies = append(policies, Policy{
			ID:           id,
			State:        state,
			Description:  description,
			Priority:     priority,
			NumA:         numA,
			NumB:         numB,
			NumC:         numC,
			SrcIP:        srcIP,
			SbcIP:        sbcIP,
			PeriodStart:  periodStart,
			PeriodStop:   periodStop,
			Target:       target,
			MatchCounter: 0,
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

func (l *PolicyLoader) ensureDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		logrus.Infof("Directory %s does not exist, creating...", dir)
		if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
			return fmt.Errorf("failed to create directory %s: %v", dir, mkErr)
		}
	}
	return nil
}
