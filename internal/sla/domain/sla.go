// Package domain holds value types for the SLA bounded context.
package domain

import "time"

// Probe is a single self-probe outcome captured by the prober.
type Probe struct {
	ID        uint64
	TS        time.Time
	Region    string
	Target    string
	Success   bool
	LatencyMs uint32
	Err       string
}

// DailyRollup aggregates one day worth of probes for a target.
type DailyRollup struct {
	Day        time.Time
	Region     string
	Target     string
	SuccessCnt uint64
	FailCnt    uint64
	P50Ms      uint32
	P95Ms      uint32
	P99Ms      uint32
}

// UptimeFraction returns successful probes / total in [0,1]. Empty rollups
// return 1.0 so the dashboard does not show false-negative outages.
func (r DailyRollup) UptimeFraction() float64 {
	total := r.SuccessCnt + r.FailCnt
	if total == 0 {
		return 1.0
	}
	return float64(r.SuccessCnt) / float64(total)
}
