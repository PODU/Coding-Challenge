package provider

import (
	"context"
	"fmt"
	"sync"

	"socialmedia/internal/model"
)

const MockMaxBody = 5000

// Mock records posts instead of publishing them. It can be told to fail, to
// test retries.
type Mock struct {
	mu sync.Mutex

	FailTimes  int  // fail the first FailTimes calls, then succeed
	FailAlways bool // fail every call (retryable)
	FailAuth   bool // fail every call with ErrAuth

	calls   []model.Post
	counter int
}

func NewMock() *Mock { return &Mock{} }

func (*Mock) Name() string { return "mock" }

func (*Mock) Validate(p model.Post) error {
	if len(p.Body) > MockMaxBody {
		return fmt.Errorf("%w: body exceeds %d characters", model.ErrValidation, MockMaxBody)
	}
	return nil
}

// Publish returns "mock-<post id>", so publishing a post twice gives the same ID.
func (m *Mock) Publish(ctx context.Context, p model.Post) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.counter++
	if m.FailAuth {
		return "", fmt.Errorf("%w: mock auth rejected", ErrAuth)
	}
	if m.FailAlways || m.counter <= m.FailTimes {
		return "", fmt.Errorf("mock transient failure (call %d)", m.counter)
	}

	m.calls = append(m.calls, p)
	return "mock-" + p.ID, nil
}

// Calls returns the posts published successfully so far.
func (m *Mock) Calls() []model.Post {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]model.Post, len(m.calls))
	copy(out, m.calls)
	return out
}

func (m *Mock) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// AttemptCount counts every Publish call, failed or not.
func (m *Mock) AttemptCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.counter
}
