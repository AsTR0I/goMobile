package fnm

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
)

type FnmLoader struct {
	repo *FnmRepository
}

func NewFnmLoader(repo *FnmRepository) *FnmLoader {
	return &FnmLoader{repo: repo}
}

func (l *FnmLoader) LoadLatestFromDir(dir string) error {
	logrus.Infof("Loading FNM from directory: %s", dir)

	files, err := filepath.Glob(filepath.Join(dir, "*.csv"))
	if err != nil {
		return fmt.Errorf("failed to list csv: %v", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no csv files in %s", dir)
	}

	sort.Strings(files)
	latestFile := files[len(files)-1]

	logrus.Infof("Latest CSV file: %s", latestFile)

	fnms, err := l.parseCSV(latestFile)
	if err != nil {
		return err
	}

	version := filepath.Base(latestFile)
	l.repo.SetFnms(fnms, version)

	logrus.Infof("Loaded %d FNM records from %s", len(fnms), latestFile)
	return nil
}

func (l *FnmLoader) parseCSV(filePath string) ([]Fnm, error) {
	logrus.Infof("Parsing CSV: %s", filePath)

	content, err := os.ReadFile(filePath)
	if err != nil {
		logrus.Errorf("Failed to read %s: %v", filePath, err)
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	var fnms []Fnm

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if i == 0 && strings.Contains(line, "id,did,nexthop") {
			continue
		}

		fields := strings.Split(line, ",")
		if len(fields) < 3 {
			logrus.Warnf("Skipping invalid line %d: %s", i+1, line)
			continue
		}

		id := strings.TrimSpace(fields[0])
		did := strings.TrimSpace(fields[1])
		nextHop := strings.TrimSpace(fields[2])

		fnms = append(fnms, Fnm{
			ID:      id,
			Did:     did,
			NextHop: nextHop,
		})

		logrus.Debugf("Loaded FNM: id=%s did=%s", id, did)
	}

	logrus.Infof("Finished parsing FNM. Total: %d", len(fnms))
	return fnms, nil
}
