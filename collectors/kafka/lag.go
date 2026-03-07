package kafka

import (
	"sync"
	"time"
)

// LagMonitor tracks consumer group lag and velocity.
type LagMonitor struct {
	threshold int64
	mu        sync.RWMutex
	groups    map[string]*GroupLagState
}

// GroupLagState tracks lag state for a consumer group.
type GroupLagState struct {
	GroupID        string                      `json:"group_id"`
	TotalLag       int64                       `json:"total_lag"`
	PartitionLags  map[int32]*PartitionLagState `json:"partition_lags"`
	Velocity       float64                     `json:"velocity"`       // lag change per second
	LastUpdated    time.Time                   `json:"last_updated"`
	AlertTriggered bool                        `json:"alert_triggered"`
}

// PartitionLagState tracks lag for a single partition.
type PartitionLagState struct {
	Partition     int32     `json:"partition"`
	CurrentOffset int64     `json:"current_offset"`
	EndOffset     int64     `json:"end_offset"`
	Lag           int64     `json:"lag"`
	Velocity      float64   `json:"velocity"`
	History       []lagPoint `json:"-"`
}

type lagPoint struct {
	lag       int64
	timestamp time.Time
}

// NewLagMonitor creates a new consumer lag monitor.
func NewLagMonitor(threshold int64) *LagMonitor {
	return &LagMonitor{
		threshold: threshold,
		groups:    make(map[string]*GroupLagState),
	}
}

// UpdateLag updates the lag for a consumer group partition.
func (m *LagMonitor) UpdateLag(groupID string, partition int32, currentOffset, endOffset int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.groups[groupID]
	if !ok {
		state = &GroupLagState{
			GroupID:       groupID,
			PartitionLags: make(map[int32]*PartitionLagState),
		}
		m.groups[groupID] = state
	}

	lag := endOffset - currentOffset
	if lag < 0 {
		lag = 0
	}

	pState, ok := state.PartitionLags[partition]
	if !ok {
		pState = &PartitionLagState{
			Partition: partition,
		}
		state.PartitionLags[partition] = pState
	}

	// Calculate velocity (rate of lag change).
	now := time.Now()
	if len(pState.History) > 0 {
		last := pState.History[len(pState.History)-1]
		elapsed := now.Sub(last.timestamp).Seconds()
		if elapsed > 0 {
			pState.Velocity = float64(lag-last.lag) / elapsed
		}
	}

	pState.CurrentOffset = currentOffset
	pState.EndOffset = endOffset
	pState.Lag = lag
	pState.History = append(pState.History, lagPoint{lag: lag, timestamp: now})

	// Keep only last 60 data points.
	if len(pState.History) > 60 {
		pState.History = pState.History[len(pState.History)-60:]
	}

	// Recalculate total lag and velocity for the group.
	var totalLag int64
	var totalVelocity float64
	for _, ps := range state.PartitionLags {
		totalLag += ps.Lag
		totalVelocity += ps.Velocity
	}
	state.TotalLag = totalLag
	state.Velocity = totalVelocity
	state.LastUpdated = now
	state.AlertTriggered = totalLag > m.threshold
}

// GetGroupLag returns the total lag for a consumer group.
func (m *LagMonitor) GetGroupLag(groupID string) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.groups[groupID]
	if !ok {
		return 0
	}
	return state.TotalLag
}

// GetGroupState returns the full lag state for a consumer group.
func (m *LagMonitor) GetGroupState(groupID string) *GroupLagState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.groups[groupID]
}

// AlertingGroups returns consumer groups exceeding the lag threshold.
func (m *LagMonitor) AlertingGroups() []*GroupLagState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var alerting []*GroupLagState
	for _, state := range m.groups {
		if state.AlertTriggered {
			alerting = append(alerting, state)
		}
	}
	return alerting
}
