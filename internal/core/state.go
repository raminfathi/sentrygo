package core

import "sync"

type SystemState struct {
	mu        sync.RWMutex // برای جلوگیری از تداخل همزمان وب و تلگرام
	armed     bool         // فیلد اصلی وضعیت
	lastActor string       // فیلد جدید برای ثبت نام شخص
}

func NewSystemState() *SystemState {
	return &SystemState{
		armed: false,
	}
}

func (s *SystemState) Arm(source string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.armed = true
	s.lastActor = source
}

func (s *SystemState) Disarm(source string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.armed = false
	s.lastActor = source
}

func (s *SystemState) IsArmed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.armed
}

// تابع جدید برای خواندن نام آخرین شخصی که سیستم را تغییر داده
func (s *SystemState) GetLastActor() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastActor
}
