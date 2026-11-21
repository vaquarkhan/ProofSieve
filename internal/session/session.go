// Package session tracks file and line budget for one agent turn.
package session

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

var lockAttempts = 50

var marshalIndent = json.MarshalIndent

// Session is the charged set of paths since the lock last changed.
type Session struct {
	LockHash string         `json:"lock_hash"`
	Files    map[string]int `json:"files"`
	Updated  time.Time      `json:"updated"`
}

// Load returns a session for lockHash, resetting when the lock or the age changes.
func Load(path, lockHash string, now time.Time) (Session, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fresh(lockHash, now), nil
		}
		return Session{}, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return fresh(lockHash, now), nil
	}
	if s.LockHash != lockHash || now.Sub(s.Updated) > 12*time.Hour || s.Files == nil {
		return fresh(lockHash, now), nil
	}
	return s, nil
}

func fresh(lockHash string, now time.Time) Session {
	return Session{LockHash: lockHash, Files: map[string]int{}, Updated: now}
}

// Save writes the session atomically.
func (s Session) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := marshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Count is the number of charged files.
func (s Session) Count() int { return len(s.Files) }

// Loc is the charged churn.
func (s Session) Loc() int {
	n := 0
	for _, v := range s.Files {
		n += v
	}
	return n
}

// Charge records loc for path. replace overwrites; otherwise it adds.
func (s *Session) Charge(path string, loc int, replace bool) {
	if s.Files == nil {
		s.Files = map[string]int{}
	}
	if replace {
		s.Files[path] = loc
		return
	}
	s.Files[path] += loc
}

// WithLock runs fn while holding an exclusive lock next to path.
func WithLock(path string, fn func() error) error {
	lock := path + ".lock"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var f *os.File
	var err error
	for i := 0; i < lockAttempts; i++ {
		f, err = os.OpenFile(lock, os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err != nil {
		return err
	}
	defer os.Remove(lock)
	defer f.Close()
	return fn()
}
