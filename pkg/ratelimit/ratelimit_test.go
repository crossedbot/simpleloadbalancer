package ratelimit

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStateStep(t *testing.T) {
	expected := time.Second * 3
	state := &leakyBucketState{Current: int64(expected)}
	actual := state.Step()
	require.Equal(t, expected, actual)
}

func TestStateSetStep(t *testing.T) {
	expected := time.Second * 3
	state := &leakyBucketState{}
	state.SetStep(expected)
	actual := time.Duration(state.Current)
	require.Equal(t, expected, actual)
}

func TestBackendState(t *testing.T) {
	expected := &leakyBucketState{Current: int64(time.Second * 3)}
	be := &leakyBucketMemoryBackend{BucketState: expected}
	actual := be.State()
	require.Equal(t, expected.Current, int64(actual.Step()))
}

func TestBackendSetState(t *testing.T) {
	expected := &leakyBucketState{Current: int64(time.Second * 3)}
	be := &leakyBucketMemoryBackend{}
	be.SetState(expected)
	actual := be.BucketState
	require.Equal(t, expected.Current, int64(actual.Step()))
}

func TestLimiterNext(t *testing.T) {
	capacity := int64(3)
	rate := time.Second * 3
	state := &leakyBucketState{}
	be := &leakyBucketMemoryBackend{BucketState: state}
	limiter := &leakyBucketLimiter{
		Backend:  be,
		Capacity: capacity,
		Lock:     new(sync.Mutex),
		Rate:     int64(rate),
	}

	// Next step must wait for other steps and the next clock cycle
	// (time_now < current_step)
	step := time.Duration(time.Now().Add(time.Second * 3).UnixNano())
	state.SetStep(step)
	next, err := limiter.Next()
	require.Nil(t, err)
	// Time is approximate to current_step + clock_cycle (3s + 3s = 6s)
	require.Greater(t, next, time.Second*5)
	require.LessOrEqual(t, next, time.Second*6)

	// Zero steps in queue (current_step = now)
	step = time.Duration(0)
	state.SetStep(step)
	next, err = limiter.Next()
	require.Nil(t, err)
	// Next step should be immediate (0s)
	require.Equal(t, next, time.Duration(0))

	// The last step happened before the next clock cycle (time_since <
	// clock_cycle)
	step = time.Duration(time.Now().Add(-time.Second * 1).UnixNano())
	state.SetStep(step)
	next, err = limiter.Next()
	require.Nil(t, err)
	// Time is approximate to clock_cycle - time_since (3s - 1s = 2s)
	require.Greater(t, next, time.Second*1)
	require.LessOrEqual(t, next, time.Second*2)

	// Max capacity has been reached
	step = time.Duration(time.Now().Add(time.Second * 12).UnixNano())
	state.SetStep(step)
	next, err = limiter.Next()
	require.Equal(t, ErrLimiterMaxCapacity, err)
	require.Greater(t, next, time.Second*14)
	require.LessOrEqual(t, next, time.Second*15)
}
