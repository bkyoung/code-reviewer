package verify

import "sync"

// CostTracker tracks verification costs against a budget ceiling.
// Use this to enforce cost limits during batch verification.
// All methods are safe for concurrent use.
type CostTracker interface {
	// AddCost adds the given amount to the running total.
	AddCost(amount float64)

	// TotalCost returns the total accumulated cost.
	TotalCost() float64

	// ExceedsCeiling returns true if the total cost meets or exceeds the ceiling.
	ExceedsCeiling() bool

	// RemainingBudget returns the remaining budget before hitting the ceiling.
	// Returns 0 if the ceiling has been exceeded.
	RemainingBudget() float64
}

// costTracker is the default implementation of CostTracker.
// It is safe for concurrent use.
type costTracker struct {
	mu      sync.RWMutex
	total   float64
	ceiling float64
}

// NewCostTracker creates a new cost tracker with the given ceiling.
func NewCostTracker(ceiling float64) CostTracker {
	return &costTracker{
		total:   0,
		ceiling: ceiling,
	}
}

func (c *costTracker) AddCost(amount float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.total += amount
}

func (c *costTracker) TotalCost() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.total
}

func (c *costTracker) ExceedsCeiling() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.total >= c.ceiling
}

func (c *costTracker) RemainingBudget() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.total >= c.ceiling {
		return 0
	}
	return c.ceiling - c.total
}
