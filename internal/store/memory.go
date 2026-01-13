package store

import (
	"sync"
)

type InMemoryStore struct {
	mu   sync.RWMutex
	data map[string]string
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		data: make(map[string]string),
	}
}

func (s *InMemoryStore) AddURL(shortCode string, url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[shortCode] = url
}

func (s *InMemoryStore) GetURL(shortCode string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	url, ok := s.data[shortCode]
	return url, ok
}
