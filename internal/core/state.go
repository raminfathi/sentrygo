package core

import (
	"log"
	"sync"
)

// SystemState manages the global Armed/Disarmed mode concurrently.
type SystemState struct {
	isArmed bool
	mu      sync.RWMutex
}

// NewSystemState creates a new instance, defaulting to disarmed (false).
func NewSystemState() *SystemState {
	return &SystemState{
		isArmed: false,
	}
}

// SetArmed changes the monitoring status of the system.
func (s *SystemState) SetArmed(status bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.isArmed = status

	if status {
		log.Println("System State: ARMED - Intrusion notifications are ENABLED.")
	} else {
		log.Println("System State: DISARMED - Intrusion notifications are DISABLED.")
	}
}

// IsArmed securely checks the current status before triggering alerts.
func (s *SystemState) IsArmed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isArmed
}
