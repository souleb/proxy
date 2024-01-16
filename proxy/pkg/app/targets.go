package app

import (
	"encoding/json"
	"sync"
	"sync/atomic"
)

// Target is a target that the application is proxying to.
type Target struct {
	//Addr is the address of the target.
	Addr string

	// IsDead is a flag that indicates if the target is dead.
	IsDead bool

	// mu is a mutex that is used to protect the IsDead flag.
	mu sync.RWMutex
}

// SetDead sets the IsDead flag to true.
func (t *Target) SetDead() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.IsDead = true
}

// IsAlive returns true if the target is alive.
func (t *Target) IsAlive() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return !t.IsDead
}

// Targets is a list of targets that the application is proxying to.
type Targets struct {
	// Targets is a list of targets.
	Targets []*Target `json:"Targets"`

	// idx is an index that is used to round robin the targets.
	// we use an atomic int to make sure that the index is incremented
	// in a thread safe manner.
	idx atomic.Int32
}

// RoundRobin returns the next target in the list.
// It uses a round robin algorithm to select the next target.
func (t *Targets) RoundRobin() *Target {
	n := int(t.idx.Load())
	target := t.Targets[n%len(t.Targets)]
	t.idx.Add(1)
	return target
}

func (t *Targets) UnmarshalJSON(p []byte) error {
	var targets []string
	if err := json.Unmarshal(p, &targets); err != nil {
		return err
	}
	for _, target := range targets {
		t.Targets = append(t.Targets, &Target{Addr: target})
	}
	return nil
}
