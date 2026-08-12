package identity

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const (
	defaultSessionTTL = time.Hour
	defaultPendingTTL = 10 * time.Minute
)

type pendingAuth struct {
	Verifier  string
	Nonce     string
	ExpiresAt time.Time
}

// Store is an in-memory session and pending-login store. It is process-local
// and is not a durable credential or policy store.
type Store struct {
	mu       sync.Mutex
	sessions map[string]Session
	pending  map[string]pendingAuth
	ttl      time.Duration
	now      func() time.Time
}

// NewStore constructs an in-memory store. ttl <= 0 uses a one-hour default.
func NewStore(ttl time.Duration, now func() time.Time) *Store {
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Store{
		sessions: make(map[string]Session),
		pending:  make(map[string]pendingAuth),
		ttl:      ttl,
		now:      now,
	}
}

// Create inserts a session and returns the opaque session id used as the cookie value.
func (s *Store) Create(principalID, issuer, subject, correlationID string) (Session, error) {
	id, err := randomID()
	if err != nil {
		return Session{}, err
	}
	if correlationID == "" {
		correlationID, err = randomID()
		if err != nil {
			return Session{}, err
		}
	}
	now := s.now()
	sess := Session{
		ID:              id,
		PrincipalID:     principalID,
		Issuer:          issuer,
		Subject:         subject,
		AuthenticatedAt: now,
		ExpiresAt:       now.Add(s.ttl),
		CorrelationID:   correlationID,
	}
	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()
	return sess, nil
}

// Get returns a fresh session for id.
func (s *Store) Get(id string) (*Session, bool) {
	if id == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return nil, false
	}
	if !sess.Fresh(s.now()) {
		delete(s.sessions, id)
		return nil, false
	}
	cp := sess
	return &cp, true
}

// Delete removes a session. Missing ids are ignored.
func (s *Store) Delete(id string) {
	if id == "" {
		return
	}
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

func (s *Store) putPending(state, verifier, nonce string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.sweepPendingLocked(now)
	s.pending[state] = pendingAuth{
		Verifier:  verifier,
		Nonce:     nonce,
		ExpiresAt: now.Add(defaultPendingTTL),
	}
	return nil
}

func (s *Store) takePending(state string) (pendingAuth, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.sweepPendingLocked(now)
	p, ok := s.pending[state]
	if !ok {
		return pendingAuth{}, false
	}
	delete(s.pending, state)
	return p, true
}

func (s *Store) pendingLen() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepPendingLocked(s.now())
	return len(s.pending)
}

func (s *Store) sweepPendingLocked(now time.Time) {
	for id, p := range s.pending {
		if !now.Before(p.ExpiresAt) {
			delete(s.pending, id)
		}
	}
}

func randomID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
