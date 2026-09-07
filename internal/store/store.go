package store

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/relentlessworks/regexkit/internal/model"
)

type Store struct {
	mu       sync.RWMutex
	path     string
	patterns map[string]*model.Pattern
}

func New(path string) *Store {
	s := &Store{
		path:     path,
		patterns: make(map[string]*model.Pattern),
	}
	s.load()
	return s
}

func (s *Store) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var pats map[string]*model.Pattern
	if err := json.Unmarshal(data, &pats); err != nil {
		return
	}
	s.mu.Lock()
	s.patterns = pats
	s.mu.Unlock()
}

func (s *Store) save() error {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.patterns, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func (s *Store) SavePattern(p *model.Pattern) error {
	s.mu.Lock()
	s.patterns[p.Handle] = p
	s.mu.Unlock()
	return s.save()
}

func (s *Store) GetPattern(handle, workspace string) (*model.Pattern, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.patterns[handle]
	if !ok || p.Workspace != workspace {
		return nil, false
	}
	return p, true
}

func (s *Store) ListPatterns(workspace string) []*model.Pattern {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*model.Pattern
	for _, p := range s.patterns {
		if p.Workspace == workspace {
			result = append(result, p)
		}
	}
	return result
}

func (s *Store) DeletePattern(handle, workspace string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.patterns[handle]
	if !ok || p.Workspace != workspace {
		return false
	}
	delete(s.patterns, handle)
	go s.save()
	return true
}
