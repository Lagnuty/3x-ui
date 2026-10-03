package xray

import (
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v2/logger"
)

// NewLogWriter returns a new LogWriter for processing Xray log output.
func NewLogWriter() *LogWriter {
	return &LogWriter{startedAt: time.Now()}
}

// LogWriter processes and filters log output from the Xray process, handling crash detection and message filtering.
type LogWriter struct {
	mu          sync.Mutex
	lastLine    string
	startedAt   time.Time
	readyAt     time.Time
	cacheHits   int
	cacheMisses int
	geoErrors   []string
}

// StartupDiagnostics contains the core startup and geodata matcher signals
// observed on stdout/stderr. The MPH cache itself is owned by Xray.
type StartupDiagnostics struct {
	Ready             bool     `json:"ready"`
	StartupDurationMs int64    `json:"startupDurationMs"`
	CacheHits         int      `json:"cacheHits"`
	CacheMisses       int      `json:"cacheMisses"`
	GeoErrors         []string `json:"geoErrors"`
}

// ResetStartup starts a fresh diagnostic window before a new Xray process is launched.
func (lw *LogWriter) ResetStartup(startedAt time.Time) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	lw.lastLine = ""
	lw.startedAt = startedAt
	lw.readyAt = time.Time{}
	lw.cacheHits = 0
	lw.cacheMisses = 0
	lw.geoErrors = nil
}

// LastLine returns the latest unstructured Xray output safely.
func (lw *LogWriter) LastLine() string {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	return lw.lastLine
}

// Snapshot returns a copy of the current startup diagnostics.
func (lw *LogWriter) Snapshot() StartupDiagnostics {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	end := lw.readyAt
	if end.IsZero() {
		end = time.Now()
	}
	duration := int64(0)
	if !lw.startedAt.IsZero() {
		duration = end.Sub(lw.startedAt).Milliseconds()
	}
	return StartupDiagnostics{
		Ready:             !lw.readyAt.IsZero(),
		StartupDurationMs: duration,
		CacheHits:         lw.cacheHits,
		CacheMisses:       lw.cacheMisses,
		GeoErrors:         append([]string(nil), lw.geoErrors...),
	}
}

func (lw *LogWriter) observe(message string) {
	lower := strings.ToLower(message)
	if lw.readyAt.IsZero() && strings.Contains(lower, "xray ") && strings.Contains(lower, " started") {
		lw.readyAt = time.Now()
	}
	if strings.Contains(lower, "matcher cache hit") {
		lw.cacheHits++
	}
	if strings.Contains(lower, "matcher cache miss") {
		lw.cacheMisses++
	}
	geoRelated := strings.Contains(lower, "geosite") || strings.Contains(lower, "geoip") || strings.Contains(lower, "geodata")
	failed := strings.Contains(lower, "failed") || strings.Contains(lower, "error") ||
		strings.Contains(lower, "not found") || strings.Contains(lower, "no such file") || strings.Contains(lower, "invalid")
	if geoRelated && failed {
		for _, existing := range lw.geoErrors {
			if existing == message {
				return
			}
		}
		if len(message) > 500 {
			message = message[:500]
		}
		if len(lw.geoErrors) == 20 {
			lw.geoErrors = lw.geoErrors[1:]
		}
		lw.geoErrors = append(lw.geoErrors, message)
	}
}

// Write processes and filters log output from the Xray process, handling crash detection and message filtering.
func (lw *LogWriter) Write(m []byte) (n int, err error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	crashRegex := regexp.MustCompile(`(?i)(panic|exception|stack trace|fatal error)`)

	// Convert the data to a string
	message := strings.TrimSpace(string(m))
	msgLowerAll := strings.ToLower(message)

	// Suppress noisy Windows process-kill signal that surfaces as exit status 1
	if runtime.GOOS == "windows" && strings.Contains(msgLowerAll, "exit status 1") {
		return len(m), nil
	}

	// Check if the message contains a crash
	if crashRegex.MatchString(message) {
		lw.observe(message)
		logger.Debug("Core crash detected:\n", message)
		lw.lastLine = message
		err1 := writeCrashReport(m)
		if err1 != nil {
			logger.Error("Unable to write crash report:", err1)
		}
		return len(m), nil
	}

	regex := regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{6}) \[([^\]]+)\] (.+)$`)
	messages := strings.SplitSeq(message, "\n")

	for msg := range messages {
		lw.observe(msg)
		matches := regex.FindStringSubmatch(msg)

		if len(matches) > 3 {
			level := matches[2]
			msgBody := matches[3]
			msgBodyLower := strings.ToLower(msgBody)

			if strings.Contains(msgBodyLower, "tls handshake error") ||
				strings.Contains(msgBodyLower, "connection ends") {
				logger.Debug("XRAY: " + msgBody)
				lw.lastLine = ""
				continue
			}

			if strings.Contains(msgBodyLower, "failed") {
				logger.Error("XRAY: " + msgBody)
			} else {
				switch level {
				case "Debug":
					logger.Debug("XRAY: " + msgBody)
				case "Info":
					logger.Info("XRAY: " + msgBody)
				case "Warning":
					logger.Warning("XRAY: " + msgBody)
				case "Error":
					logger.Error("XRAY: " + msgBody)
				default:
					logger.Debug("XRAY: " + msg)
				}
			}
			lw.lastLine = ""
		} else if msg != "" {
			msgLower := strings.ToLower(msg)

			if strings.Contains(msgLower, "tls handshake error") ||
				strings.Contains(msgLower, "connection ends") {
				logger.Debug("XRAY: " + msg)
				lw.lastLine = msg
				continue
			}

			if strings.Contains(msgLower, "failed") {
				logger.Error("XRAY: " + msg)
			} else {
				logger.Debug("XRAY: " + msg)
			}
			lw.lastLine = msg
		}
	}

	return len(m), nil
}
