package ratelimit

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var (
	//Errors
	ErrLimiterMaxCapacity = errors.New("Unable to service request - max capacity reached")
)

// LeakyBucketState keeps track of the current state of the Leaky Bucket
// backend. State is the accumulated timed intervals called steps. The number of
// steps are independently determined by a rate. For example, if the rate is 3
// seconds and the current step is 9 seconds, then there are 3 steps currently
// tracked in the state.
type LeakyBucketState interface {
	// Step returns the state's current timed interval step.
	Step() time.Duration

	// SetStep sets the state's current timed interval step.
	SetStep(time.Duration)
}

// leakBucketState implements the LeakBucketState interface and tracks the
// current time interval in memory.
type leakyBucketState struct {
	Current int64 // The current steps
}

func (state *leakyBucketState) Step() time.Duration {
	return time.Duration(state.Current)
}

func (state *leakyBucketState) SetStep(dur time.Duration) {
	atomic.StoreInt64(&state.Current, int64(dur))
}

// NewLeakyBucketState returns a new LeakBucketState.
func NewLeakyBucketState() LeakyBucketState {
	return &leakyBucketState{}
}

// LeakyBucketBackend represents an interface to a backend to a Leak Bucket. It
// manages the state of a single Leaky Bucket. This interface is generalized to
// be implemented in memory, to file, to database, whatever.
type LeakyBucketBackend interface {
	// State returns an interface to the state of the backend.
	State() LeakyBucketState

	// SetState sets the current state of the backend.
	SetState(state LeakyBucketState)
}

// leakyBucketMemoryBackend implements a LeakyBucketBackend in memory.
type leakyBucketMemoryBackend struct {
	BucketState LeakyBucketState // The backend bucket state
}

func (be *leakyBucketMemoryBackend) State() LeakyBucketState {
	return be.BucketState
}

func (be *leakyBucketMemoryBackend) SetState(state LeakyBucketState) {
	// XXX should this have syncing mechanisms?
	be.BucketState = state
}

// NewLeakyBucketBackend returns a new LeakyBucketBackend for tracking bucket
// state.
func NewLeakyBucketBackend() LeakyBucketBackend {
	return &leakyBucketMemoryBackend{
		BucketState: NewLeakyBucketState(),
	}
}

// LeakyBucketLimiter represents an interface to a rate limiter using the Leaky
// Bucket algorithm.
type LeakyBucketLimiter interface {
	// Next returns the next timed interval before whatever action is being
	// limited can be tried.
	Next() (time.Duration, error)
}

// leakyBucketLimiter implements the LeakyBucketLimiter interface. Tracking its
// own bucket backend, step capacity, and rate in time.
type leakyBucketLimiter struct {
	Backend  LeakyBucketBackend // Interface to the bucket backend
	Capacity int64              // Step capacity
	Lock     *sync.Mutex        // Lock for concurrency
	Rate     int64              // Timed action rate
}

// NewLeakyBucket returns a new LeakyBucketLimiter with the given step capacity
// and timed rate.
func NewLeakyBucket(capacity int64, rate int64) LeakyBucketLimiter {
	return &leakyBucketLimiter{
		Backend:  NewLeakyBucketBackend(),
		Capacity: capacity,
		Lock:     new(sync.Mutex),
		Rate:     rate,
	}
}

func (limiter *leakyBucketLimiter) Next() (time.Duration, error) {
	limiter.Lock.Lock()
	defer limiter.Lock.Unlock()
	state := limiter.Backend.State()
	step := int64(state.Step())
	now := time.Now().UnixNano()
	if now < step {
		// The current steps haven't been processed yet, therefore the
		// next step must wait for those steps to complete plus the rate
		// interval
		step += limiter.Rate
	} else {
		// The last step occurred a "long time ago", so set the next
		// step to now
		since := now - step
		step = now
		if since < limiter.Rate {
			// If the last step occurred less than the rate interval
			// ago, add the difference to the next step time
			step += limiter.Rate - since
		}
	}
	// Determine the time duration until the next step can be taken and add
	// it to the bucket state if step capacity has not been reached.
	// Otherwise the bucket has reached its capacity and allow this step to
	// "leak"
	next := step - now
	if (next / limiter.Rate) <= limiter.Capacity {
		state.SetStep(time.Duration(step))
		return time.Duration(next), nil
	}
	return time.Duration(next), ErrLimiterMaxCapacity
}
