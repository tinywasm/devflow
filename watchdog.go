package devflow

import (
	"regexp"
	"strings"
	"sync"
	"time"
)

// Watchdog monitors test output for stalled tests.
type Watchdog struct {
	timeout    time.Duration
	onKill     func()
	running    map[string]time.Time
	culprits   []string
	mu         sync.Mutex
	stop       chan struct{}
	lineBuf    string
	runRe      *regexp.Regexp
	pauseRe    *regexp.Regexp
	contRe     *regexp.Regexp
	completeRe *regexp.Regexp
	killed     bool
}

// NewWatchdog creates a new watchdog.
func NewWatchdog(timeout time.Duration, onKill func()) *Watchdog {
	return &Watchdog{
		timeout:    timeout,
		onKill:     onKill,
		running:    make(map[string]time.Time),
		stop:       make(chan struct{}),
		runRe:      regexp.MustCompile(`=== RUN\s+(\S+)`),
		pauseRe:    regexp.MustCompile(`=== PAUSE\s+(\S+)`),
		contRe:     regexp.MustCompile(`=== CONT\s+(\S+)`),
		completeRe: regexp.MustCompile(`--- (?:PASS|FAIL|SKIP): (\S+)`),
	}
}

// Start begins the monitoring goroutine.
func (w *Watchdog) Start() {
	go func() {
		ticker := time.NewTicker(w.timeout / 4)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				w.check()
			case <-w.stop:
				return
			}
		}
	}()
}

// Stop halts the monitoring goroutine.
func (w *Watchdog) Stop() {
	close(w.stop)
}

// Add appends output to be parsed.
func (w *Watchdog) Add(s string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.lineBuf += s
	for {
		idx := strings.Index(w.lineBuf, "\n")
		if idx == -1 {
			break
		}
		line := w.lineBuf[:idx]
		w.lineBuf = w.lineBuf[idx+1:]
		w.processLine(line)
	}
}

func (w *Watchdog) processLine(line string) {
	if m := w.runRe.FindStringSubmatch(line); m != nil {
		w.running[m[1]] = time.Now()
	} else if m := w.pauseRe.FindStringSubmatch(line); m != nil {
		delete(w.running, m[1])
	} else if m := w.contRe.FindStringSubmatch(line); m != nil {
		w.running[m[1]] = time.Now()
	} else if m := w.completeRe.FindStringSubmatch(line); m != nil {
		delete(w.running, m[1])
	}
}

func (w *Watchdog) check() {
	w.mu.Lock()
	onKill := w.onKill
	killed := w.killed
	timeout := w.timeout
	running := make(map[string]time.Time)
	for k, v := range w.running {
		running[k] = v
	}
	w.mu.Unlock()

	if killed || onKill == nil {
		return
	}

	now := time.Now()
	for name, start := range running {
		if now.Sub(start) > timeout {
			w.mu.Lock()
			if w.killed { // double-check
				w.mu.Unlock()
				return
			}
			w.culprits = append(w.culprits, name)
			w.killed = true
			w.onKill = nil
			w.mu.Unlock()

			onKill()
			return
		}
	}
}

// Culprits returns the list of tests that were running when the watchdog fired.
func (w *Watchdog) Culprits() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.culprits
}
