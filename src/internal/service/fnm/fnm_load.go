package fnm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/sirupsen/logrus"
)

type FnmLoader struct {
	repo *FnmRepository
}

func NewFnmLoader(repo *FnmRepository) *FnmLoader {
	return &FnmLoader{repo: repo}
}

func (l *FnmLoader) LoadFromAPI(url string, saveDir string) error {
	if err := l.ensureDir(saveDir); err != nil {
		return err
	}
	logrus.Infof("Loading FNM from API: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to GET api: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("bad status: %v", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read body: %v", err)
	}

	var fnms []Fnm
	if err := json.Unmarshal(body, &fnms); err != nil {
		return fmt.Errorf("failed to unmarshal API response: %v", err)
	}

	version := time.Now().Format("20060102_150405")
	filePath := filepath.Join(saveDir, fmt.Sprintf("%s.json", version))

	if err := os.WriteFile(filePath, body, 0644); err != nil {
		logrus.Warnf("failed to save FNM JSON: %v", err)
	}

	for i := range fnms {
		if err := json.Unmarshal([]byte(fnms[i].TenantRaw), &fnms[i].Tenant); err != nil {
			logrus.Warnf("failed to parse tenant for msisdn=%s: %v", fnms[i].Msisdn, err)
		}
	}

	l.repo.SetFnms(fnms, version)

	logrus.Infof("Loaded %d FNM items from API", len(fnms))
	return nil
}

func (l *FnmLoader) LoadLatestFromDir(dir string) error {
	if err := l.ensureDir(dir); err != nil {
		return err
	}
	logrus.Infof("Loading FNM from directory: %s", dir)

	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to list json: %v", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no json files in %s", dir)
	}

	sort.Strings(files)
	latest := files[len(files)-1]

	logrus.Infof("Latest JSON: %s", latest)

	fnms, err := l.parseJSON(latest)
	if err != nil {
		return err
	}

	version := filepath.Base(latest)

	for i := range fnms {
		if err := json.Unmarshal([]byte(fnms[i].TenantRaw), &fnms[i].Tenant); err != nil {
			logrus.Warnf("failed to parse tenant for msisdn=%s: %v", fnms[i].Msisdn, err)
		}
	}

	l.repo.SetFnms(fnms, version)

	logrus.Infof("Loaded %d FNM items from file %s", len(fnms), latest)
	return nil
}

func (l *FnmLoader) parseJSON(path string) ([]Fnm, error) {
	logrus.Infof("Parsing JSON: %s", path)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file: %v", err)
	}

	var fnms []Fnm
	if err := json.Unmarshal(data, &fnms); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	logrus.Infof("Parsed %d FNM items from JSON", len(fnms))
	return fnms, nil
}

func (l *FnmLoader) ensureDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		logrus.Infof("Directory %s does not exist, creating...", dir)
		if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
			return fmt.Errorf("failed to create directory %s: %v", dir, mkErr)
		}
	}
	return nil
}

func (l *FnmLoader) parseTenants(fnms []Fnm) {
	for i := range fnms {
		if err := json.Unmarshal([]byte(fnms[i].TenantRaw), &fnms[i].Tenant); err != nil {
			logrus.Warnf("failed to parse tenant for msisdn=%s: %v", fnms[i].Msisdn, err)
		}
	}
}
