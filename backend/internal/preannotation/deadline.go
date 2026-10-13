package preannotation

import "time"

func (s *Service) now() time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return time.Now()
}
func (s *Service) remaining(started time.Time) time.Duration {
	return max(120*time.Second-s.now().Sub(started), 0)
}
