package handlers

import (
	"sync"
	"time"
)

type checkinRecord struct {
	studentName string
	timestamp   time.Time
}

// RateLimiter prevents rapid sequential check-ins of different students from the same device.
type RateLimiter struct {
	mu      sync.Mutex
	records map[string][]checkinRecord // key: deviceKey (ip+fingerprint+deviceID)
}

func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{records: make(map[string][]checkinRecord)}
	// Cleanup old entries every 5 minutes
	go func() {
		for range time.Tick(5 * time.Minute) {
			rl.cleanup()
		}
	}()
	return rl
}

// DeviceKey builds a rate-limit key from device identity factors.
func DeviceKey(clientIP, fingerprint, deviceID string) string {
	key := clientIP
	if fingerprint != "" {
		key += "|" + fingerprint
	}
	if deviceID != "" {
		key += "|" + deviceID
	}
	return key
}

// Check returns an error message if the check-in should be rate-limited.
// Same student re-checking-in is always allowed.
func (rl *RateLimiter) Check(deviceKey, studentName, deviceType string) string {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cooldown := 2 * time.Minute
	if deviceType == "kiosk" {
		cooldown = 30 * time.Second
	}

	now := time.Now()
	cutoff := now.Add(-cooldown)

	records := rl.records[deviceKey]
	for _, r := range records {
		if r.timestamp.After(cutoff) && r.studentName != studentName {
			remaining := r.timestamp.Add(cooldown).Sub(now)
			secs := int(remaining.Seconds()) + 1
			return "Please wait " + itoa(secs) + " seconds before checking in another student"
		}
	}
	return ""
}

// Record logs a successful check-in for rate limiting.
func (rl *RateLimiter) Record(deviceKey, studentName string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.records[deviceKey] = append(rl.records[deviceKey], checkinRecord{
		studentName: studentName,
		timestamp:   time.Now(),
	})
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	for key, records := range rl.records {
		var fresh []checkinRecord
		for _, r := range records {
			if r.timestamp.After(cutoff) {
				fresh = append(fresh, r)
			}
		}
		if len(fresh) == 0 {
			delete(rl.records, key)
		} else {
			rl.records[key] = fresh
		}
	}
}

func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}
