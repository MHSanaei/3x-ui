package service

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// oneDayJobs holds the one-day commands (clean IP full test, DNS resolver test) a device on
// an ISP runs: in memory only, keyed by a random token, each keeping its latest reports.
type oneDayJobs[Req, Rep any] struct {
	mu   sync.Mutex
	jobs map[string]*oneDayJob[Req, Rep]
	seq  uint64
}

type oneDayJob[Req, Rep any] struct {
	req     Req
	expires time.Time
	seq     uint64 // creation order: expiry times tie on a coarse clock (Windows)
	reports []Rep
	scripts map[string]string
}

// add stores req for cleanIpCommandTTL; a full store drops its oldest job rather than refuse.
func (s *oneDayJobs[Req, Rep]) add(req Req) (string, time.Time, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	now := cleanIpNow()
	expires := now.Add(cleanIpCommandTTL)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.jobs == nil {
		s.jobs = map[string]*oneDayJob[Req, Rep]{}
	}
	for t, job := range s.jobs {
		if now.After(job.expires) {
			delete(s.jobs, t)
		}
	}
	for len(s.jobs) >= cleanIpMaxCommands {
		oldest := ""
		for t, job := range s.jobs {
			if oldest == "" || job.seq < s.jobs[oldest].seq {
				oldest = t
			}
		}
		delete(s.jobs, oldest)
	}
	s.seq++
	s.jobs[token] = &oneDayJob[Req, Rep]{req: req, expires: expires, seq: s.seq}
	return token, expires, nil
}

// jobFor needs s.mu held.
func (s *oneDayJobs[Req, Rep]) jobFor(token string) (*oneDayJob[Req, Rep], error) {
	job, ok := s.jobs[token]
	if !ok || cleanIpNow().After(job.expires) {
		return nil, errCleanIpExpired
	}
	return job, nil
}

func (s *oneDayJobs[Req, Rep]) request(token string) (Req, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, err := s.jobFor(token)
	if err != nil {
		var zero Req
		return zero, err
	}
	return job.req, nil
}

// setScripts fixes the scripts the job's commands hash; "" is the default variant.
func (s *oneDayJobs[Req, Rep]) setScripts(token string, scripts map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, err := s.jobFor(token)
	if err != nil {
		return err
	}
	job.scripts = scripts
	return nil
}

// script is the job's variant for key, else its default one.
func (s *oneDayJobs[Req, Rep]) script(token, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, err := s.jobFor(token)
	if err != nil {
		return "", err
	}
	if v, ok := job.scripts[key]; ok && key != "" {
		return v, nil
	}
	if v, ok := job.scripts[""]; ok {
		return v, nil
	}
	return "", errCleanIpExpired
}

func (s *oneDayJobs[Req, Rep]) reports(token string) ([]Rep, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, err := s.jobFor(token)
	if err != nil {
		return nil, err
	}
	return append([]Rep{}, job.reports...), nil
}

// addReport keeps the newest cleanIpMaxReports reports of the job.
func (s *oneDayJobs[Req, Rep]) addReport(token string, report Rep) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, err := s.jobFor(token)
	if err != nil {
		return err
	}
	job.reports = append(job.reports, report)
	if len(job.reports) > cleanIpMaxReports {
		job.reports = job.reports[len(job.reports)-cleanIpMaxReports:]
	}
	return nil
}
