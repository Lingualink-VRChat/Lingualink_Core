package processing

import (
	"errors"
	"sync"
	"time"
)

var ErrStatusNotFound = errors.New("processing status not found")

// ProcessingStatus represents the current status of a request that is being processed.
type ProcessingStatus struct {
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	Message   string `json:"message,omitempty"`
	UpdatedAt int64  `json:"updated_at"`
}

// StatusStore provides a storage interface for ProcessingStatus lookups.
type StatusStore interface {
	Get(requestID string) (*ProcessingStatus, error)
	Set(requestID string, status *ProcessingStatus) error
}

// InMemoryStatusStore stores ProcessingStatus entries in memory with TTL.
type InMemoryStatusStore struct {
	store sync.Map
	ttl   time.Duration
}

type statusEntry struct {
	status    *ProcessingStatus
	expiresAt time.Time
}

// NewInMemoryStatusStore creates an in-memory status store with a TTL.
func NewInMemoryStatusStore(ttl time.Duration) *InMemoryStatusStore {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return &InMemoryStatusStore{ttl: ttl}
}

func (s *InMemoryStatusStore) Get(requestID string) (*ProcessingStatus, error) {
	if requestID == "" {
		return nil, ErrStatusNotFound
	}

	v, ok := s.store.Load(requestID)
	if !ok {
		return nil, ErrStatusNotFound
	}
	entry, ok := v.(statusEntry)
	if !ok || entry.status == nil {
		s.store.Delete(requestID)
		return nil, ErrStatusNotFound
	}

	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		s.store.Delete(requestID)
		return nil, ErrStatusNotFound
	}

	cp := *entry.status
	return &cp, nil
}

func (s *InMemoryStatusStore) Set(requestID string, status *ProcessingStatus) error {
	if requestID == "" {
		return errors.New("requestID is required")
	}
	if status == nil {
		return errors.New("status is required")
	}

	cp := *status
	cp.RequestID = requestID
	cp.UpdatedAt = time.Now().Unix()

	s.store.Store(requestID, statusEntry{
		status:    &cp,
		expiresAt: time.Now().Add(s.ttl),
	})
	return nil
}
