package fnm

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Fnm struct {
	Msisdn         string `json:"msisdn"`
	Iccid          string `json:"iccid"`
	TenantRaw      string `json:"tenant"`
	Tenant         Tenant
	InternalNumber string `json:"internal_number"`
}

type Tenant struct {
	Account struct {
		ID          json.Number `json:"id"`
		AccessCode  string      `json:"access_code"`
		Voicenumber string      `json:"voicenumber"`
		Pincode     string      `json:"pincode"`
	} `json:"account"`
	Service struct {
		Type string `json:"type"`
		Node string `json:"node"`
	} `json:"service"`
}

// FnmRepository хранит FNM записи в map по Msisdn
type FnmRepository struct {
	mutex    sync.RWMutex
	fnms     map[string]*Fnm
	version  string
	lastLoad time.Time
}

// NewFnmRepository создает новый репозиторий
func NewFnmRepository() *FnmRepository {
	return &FnmRepository{
		fnms: make(map[string]*Fnm),
	}
}

// SetFnms сохраняет FNM записи в map и обновляет версию и время
func (r *FnmRepository) SetFnms(fnms []Fnm, version string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	m := make(map[string]*Fnm, len(fnms))
	for i := range fnms {
		f := fnms[i]
		m[f.Msisdn] = &f
	}

	r.fnms = m
	r.version = version
	r.lastLoad = time.Now()

	logrus.Infof("FnmRepository updated: %d items, version=%s", len(fnms), version)
}

// GetFnms возвращает все FNM записи
func (r *FnmRepository) GetFnms() []Fnm {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	fnms := make([]Fnm, 0, len(r.fnms))
	for _, f := range r.fnms {
		fnms = append(fnms, *f)
	}
	return fnms
}

// GetVersion возвращает текущую версию FNM
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

func (r *FnmRepository) FindFnm(num, callID string) *Fnm {
	logrus.Infof("Call-ID: %s — Searching FNM num=%s", callID, num)

	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if len(r.fnms) == 0 {
		logrus.Warnf("Call-ID: %s — No FNM records loaded", callID)
		return nil
	}

	if f, ok := r.fnms[num]; ok {
		logrus.Infof(
			"Call-ID: %s — FNM match: msisdn=%s internal=%s",
			callID, f.Msisdn, f.InternalNumber,
		)
		return f
	}

	logrus.Warnf("Call-ID: %s — No FNM match for Num=%s", callID, num)
	return nil
}
