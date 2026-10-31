package core

import "sync"

// SystemState وضعیت کلی سیستم امنیتی را مدیریت می‌کند
type SystemState struct {
	mu    sync.RWMutex
	armed bool
}

// NewSystemState سازنده اولیه برای وضعیت سیستم
func NewSystemState() *SystemState {
	return &SystemState{
		armed: false, // حالت پیش‌فرض
	}
}

// Arm سیستم را مسلح (آماده ارسال هشدار) می‌کند
func (s *SystemState) Arm() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.armed = true
}

// Disarm سیستم را غیرمسلح (توقف موقت هشدارها) می‌کند
func (s *SystemState) Disarm() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.armed = false
}

// IsArmed وضعیت فعلی سیستم را برمی‌گرداند
func (s *SystemState) IsArmed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.armed
}

func (s *SystemState) SetArmed(status bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.armed = status
}
