package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadSaveCharge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run", "session.json")
	now := time.Now()
	s, err := Load(path, "abc", now)
	if err != nil || s.Count() != 0 {
		t.Fatal(err)
	}
	s.Charge("a.py", 3, true)
	s.Charge("a.py", 1, false)
	s.Charge("b.py", 2, true)
	if s.Count() != 2 || s.Loc() != 6 {
		t.Fatalf("%+v", s)
	}
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path, "abc", now)
	if err != nil || got.Loc() != 6 {
		t.Fatal(err, got)
	}
	reset, err := Load(path, "other", now)
	if err != nil || reset.Count() != 0 {
		t.Fatal(reset, err)
	}
	old, err := Load(path, "abc", now.Add(13*time.Hour))
	if err != nil || old.Count() != 0 {
		t.Fatal(old, err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	broken, err := Load(path, "abc", now)
	if err != nil || broken.Count() != 0 {
		t.Fatal(broken, err)
	}
}

func TestWithLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	err := WithLock(path, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := WithLock(path, func() error { return os.ErrClosed }); err == nil {
		t.Fatal("wanted fn error")
	}
}

func TestChargeNilMap(t *testing.T) {
	s := &Session{}
	s.Charge("a", 1, false)
	if s.Loc() != 1 {
		t.Fatal(s)
	}
}

func TestErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir, "abc", time.Now()); err == nil {
		t.Fatal("directory read")
	}
	file := filepath.Join(dir, "notdir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WithLock(filepath.Join(file, "session.json"), func() error { return nil }); err == nil {
		t.Fatal("mkdir")
	}
	orig := lockAttempts
	lockAttempts = 1
	defer func() { lockAttempts = orig }()
	path := filepath.Join(dir, "held.json")
	if err := os.WriteFile(path+".lock", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WithLock(path, func() error { return nil }); err == nil {
		t.Fatal("held")
	}
	origMarshal := marshalIndent
	marshalIndent = func(any, string, string) ([]byte, error) { return nil, os.ErrInvalid }
	defer func() { marshalIndent = origMarshal }()
	if err := (Session{Files: map[string]int{}}).Save(filepath.Join(dir, "out.json")); err == nil {
		t.Fatal("marshal")
	}
}
