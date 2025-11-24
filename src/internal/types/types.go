package types

import (
	"errors"
	"net"
	"regexp"
)

type RequestMeta struct {
	CallID string
	Method string // INVITE, OPTIONS, POST и т.д.
	Source string
	Body   string
}

type Responder interface {
	Respond(req RequestMeta, code int, reason string, body string)
}

type Logger interface {
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

type ConfigField struct {
	Key       string
	Label     string
	Color     func(a ...interface{}) string
	Indent    int
	IsNested  bool
	IsSpecial bool
}

type ConfigSection struct {
	Name     string
	Fields   []ConfigField
	IsNested bool
}

type ColorSet struct {
	White   func(a ...interface{}) string
	Green   func(a ...interface{}) string
	Yellow  func(a ...interface{}) string
	Blue    func(a ...interface{}) string
	Red     func(a ...interface{}) string
	Cyan    func(a ...interface{}) string
	Magenta func(a ...interface{}) string
	Debug   func(a ...interface{}) string
}

// Policy related types

var ErrPolicyNotFound = errors.New("policy not found")

type Policy struct {
	ID           int
	State        int
	Description  string
	Priority     int
	NumA         *regexp.Regexp
	NumB         *regexp.Regexp
	NumC         *regexp.Regexp
	PeriodStart  int64
	PeriodStop   int64
	SrcIP        []*net.IPNet
	SbcIP        []*net.IPNet
	Target       string
	MatchCounter int
}

type PolicyInput struct {
	NumA      string
	NumB      string
	NumC      string
	SrcIP     net.IP
	SbcIP     net.IP
	Timestamp int64
}

type BusinessLogicResult struct {
	Target   string
	Reason   string
	Priority int
	ID       int
}

func (p *Policy) Match(in PolicyInput) bool {

	if p.NumA != nil && !p.NumA.MatchString(in.NumA) {
		return false
	}
	if p.NumB != nil && !p.NumB.MatchString(in.NumB) {
		return false
	}
	if p.NumC != nil && !p.NumC.MatchString(in.NumC) {
		return false
	}

	if in.Timestamp < p.PeriodStart || in.Timestamp > p.PeriodStop {
		return false
	}

	if !matchIP(p.SrcIP, in.SrcIP) {
		return false
	}
	if !matchIP(p.SbcIP, in.SbcIP) {
		return false
	}

	return true
}

func matchIP(nets []*net.IPNet, ip net.IP) bool {
	if len(nets) == 0 {
		return true
	}
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
