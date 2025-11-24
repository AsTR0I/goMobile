package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var mgr *manager

type manager struct {
	dir         string
	mu          sync.Mutex
	f           *os.File
	retention   int
	rotateTimer *time.Timer
	quit        chan struct{}
}

type customFormatter struct{}

func (f *customFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	t := entry.Time.Format("2006-01-02 15:04:05")
	level := strings.ToUpper(entry.Level.String())
	msg := entry.Message

	switch entry.Level {
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		level = color.RedString(level)
	case logrus.WarnLevel:
		level = color.YellowString(level)
	case logrus.InfoLevel:
		level = color.GreenString(level)
	case logrus.DebugLevel, logrus.TraceLevel:
		level = color.CyanString(level)
	}

	line := fmt.Sprintf("[%s] [%s] %s\n", t, level, msg)
	return []byte(line), nil
}

// Init initializes logging: daily file named YYYY-MM-DD.log and cleaner
func Init(dir string, retentionDays int) error {
	if dir == "" {
		dir = "logs"
	}
	if retentionDays <= 0 {
		retentionDays = 7
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	m := &manager{dir: dir, retention: retentionDays, quit: make(chan struct{})}
	if err := m.openForToday(); err != nil {
		return err
	}
	// set logrus output to file (stdout can be added by consumer)
	logrus.SetOutput(m.f)
	logrus.SetLevel(logrus.TraceLevel)
	logrus.SetFormatter(new(customFormatter))

	if viper.GetBool("flags.debug") {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	m.deleteOld()

	mgr = m
	go m.rotationLoop()
	go m.cleanerLoop()
	return nil
}

func (m *manager) openForToday() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.f != nil {
		m.f.Close()
	}
	name := time.Now().Format("2006-01-02") + ".log"
	path := filepath.Join(m.dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	m.f = f
	return nil
}

func (m *manager) rotationLoop() {
	for {
		// compute duration until next midnight
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		d := time.Until(next)
		timer := time.NewTimer(d)
		select {
		case <-m.quit:
			timer.Stop()
			return
		case <-timer.C:
			// rotate file
			m.openForToday()
			// update logrus writer to include new file
			logrus.SetOutput(m.f)
		}
	}
}

func (m *manager) cleanerLoop() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-m.quit:
			return
		case <-ticker.C:
			m.deleteOld()
		}
	}
}

func (m *manager) deleteOld() {
	files, err := os.ReadDir(m.dir)
	if err != nil {
		logrus.Errorf("failed to read log directory '%s': %v", m.dir, err)
		return
	}

	cutoff := time.Now().AddDate(0, 0, -m.retention).Format("2006-01-02")
	logrus.Infof("running log cleanup, retention: %d days, cutoff: %s", m.retention, cutoff)

	for _, fi := range files {
		if fi.IsDir() {
			continue
		}

		name := fi.Name()
		if !strings.HasSuffix(name, ".log") {
			continue
		}

		// имя файла в формате YYYY-MM-DD.log
		datePart := strings.TrimSuffix(name, ".log")

		if datePart < cutoff {
			fullPath := filepath.Join(m.dir, name)
			err := os.Remove(fullPath)
			if err != nil {
				logrus.Warnf("failed to delete old log file %s: %v", name, err)
				continue
			}
			logrus.Infof("deleted old log file: %s", name)
		} else {
			logrus.Debugf("keeping log file %s", name)
		}
	}
}

// Close stops background goroutines and closes file
func Close() error {
	if mgr == nil {
		return nil
	}
	close(mgr.quit)
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if mgr.f != nil {
		return mgr.f.Close()
	}
	return nil
}

// helper to log internal errors
func internalErrf(format string, a ...interface{}) {
	// if logrus already initialized, use it, otherwise fallback to fmt
	if logrus.StandardLogger() != nil {
		logrus.Errorf(format, a...)
		return
	}
	fmt.Printf(format+"\n", a...)
}
