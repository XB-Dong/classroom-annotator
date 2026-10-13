package help

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var ErrInvalidQuestion = errors.New("invalid help question")
var ErrLimited = errors.New("help limit reached")

type Reference struct {
	SectionID string `json:"sectionId"`
	Title     string `json:"title"`
	Anchor    string `json:"anchor"`
}
type Answer struct {
	Answer          string      `json:"answer"`
	References      []Reference `json:"references"`
	DocumentVersion string      `json:"documentVersion"`
	DocumentSHA256  string      `json:"documentSHA256"`
}
type usage struct {
	start  time.Time
	count  int
	active bool
}
type Service struct {
	Document Document
	Provider Provider
	mu       sync.Mutex
	users    map[string]usage
	slots    chan struct{}
}

func NewService(d Document, p Provider) *Service {
	return &Service{Document: d, Provider: p, users: map[string]usage{}, slots: make(chan struct{}, 2)}
}
func (s *Service) Ask(ctx context.Context, userID, q string) (Answer, error) {
	q = strings.TrimSpace(q)
	if userID == "" || q == "" || len(q) > 4096 || utf8.RuneCountInString(q) > 1000 {
		return Answer{}, ErrInvalidQuestion
	}
	if s.Provider == nil {
		return Answer{}, ErrUnavailable
	}
	s.mu.Lock()
	now := time.Now()
	u := s.users[userID]
	if !u.active && now.Sub(u.start) >= time.Hour {
		u = usage{start: now}
	}
	if u.active || u.count >= 10 {
		s.mu.Unlock()
		return Answer{}, ErrLimited
	}
	if len(s.users) >= 4096 {
		for id, v := range s.users {
			if !v.active && now.Sub(v.start) >= time.Hour {
				delete(s.users, id)
			}
		}
		if _, ok := s.users[userID]; !ok && len(s.users) >= 4096 {
			s.mu.Unlock()
			return Answer{}, ErrLimited
		}
	}
	select {
	case s.slots <- struct{}{}:
	default:
		s.mu.Unlock()
		return Answer{}, ErrLimited
	}
	u.active = true
	u.count++
	s.users[userID] = u
	s.mu.Unlock()
	defer func() {
		<-s.slots
		s.mu.Lock()
		u := s.users[userID]
		u.active = false
		s.users[userID] = u
		s.mu.Unlock()
	}()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	raw, e := s.Provider.Answer(ctx, s.Document, q)
	if e != nil {
		return Answer{}, e
	}
	if ctx.Err() != nil {
		return Answer{}, ErrUnavailable
	}
	if strings.TrimSpace(raw.Answer) == "" || utf8.RuneCountInString(raw.Answer) > 1500 || len(raw.References) == 0 || len(raw.References) > len(s.Document.Sections) {
		return Answer{}, ErrInvalidResponse
	}
	result := Answer{Answer: raw.Answer, DocumentVersion: s.Document.Version, DocumentSHA256: s.Document.SHA256, References: []Reference{}}
	seen := map[string]bool{}
	for _, id := range raw.References {
		var found *Section
		for i := range s.Document.Sections {
			if s.Document.Sections[i].ID == id {
				found = &s.Document.Sections[i]
				break
			}
		}
		if found == nil {
			return Answer{}, ErrInvalidResponse
		}
		if !seen[id] {
			result.References = append(result.References, Reference{id, found.Title, found.Anchor})
			seen[id] = true
		}
	}
	return result, nil
}
