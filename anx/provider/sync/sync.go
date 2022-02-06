package sync

import (
	"sync"
	"time"
)

// SubjectLock creates an atomic lock on a specific subject
type SubjectLock struct {
	m     sync.Mutex
	locks map[string]byte
}

func (r *SubjectLock) Lock(subject string) {
	for {
		r.m.Lock()
		locked := r.locks[subject]
		if locked == 0 {
			locked = 1
			r.locks[subject] = 1
			return
		}
		r.m.Unlock()
		time.Sleep(1 * time.Second)
	}
}

func (r *SubjectLock) Unlock(subject string) {
	r.m.Lock()
	delete(r.locks, subject)
	r.m.Unlock()
}
