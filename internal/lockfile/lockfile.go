// Package lockfile reads and writes .proofsieve/scope.lock.yaml.
package lockfile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vaquarkhan/ProofSieve/internal/globset"
	"gopkg.in/yaml.v3"
)

var (
	yamlMarshal = yaml.Marshal
	readFile    = os.ReadFile
	walkDir     = filepath.WalkDir
)

// Lock is the per-change scope contract.
type Lock struct {
	Version    string        `yaml:"version"`
	SpecRef    string        `yaml:"spec_ref"`
	ChangeType string        `yaml:"change_type"`
	Owner      string        `yaml:"owner"`
	Blast      Blast         `yaml:"blast_radius"`
	Tests      TestIntegrity `yaml:"test_integrity"`
	Hash       string        `yaml:"-"`
}

// Blast is the file and line budget.
type Blast struct {
	MaxFiles int      `yaml:"max_files"`
	MaxLoc   int      `yaml:"max_loc_delta"`
	ChurnPct int      `yaml:"max_nonfunctional_churn_pct"`
	Allowed  []string `yaml:"allowed_paths"`
	Denied   []string `yaml:"denied_paths"`
}

// TestIntegrity names human-owned tests the agent must not rewrite.
type TestIntegrity struct {
	AllowDelete   bool         `yaml:"allow_test_deletion"`
	AllowSkip     bool         `yaml:"allow_skip_annotations"`
	AllowMutation bool         `yaml:"allow_test_mutation"`
	Locked        []LockedFile `yaml:"locked_files"`
}

// LockedFile is a path and its sha256 digest.
type LockedFile struct {
	Path   string `yaml:"path"`
	SHA256 string `yaml:"sha256"`
}

// Options selects inputs for Generate.
type Options struct {
	Repo       string
	Spec       string
	ChangeType string
	Owner      string
	MaxFiles   int
	MaxLoc     int
	ChurnPct   int
	Allow      []string
	Deny       []string
	LockGlobs  []string
}

// Load reads a lock from disk.
func Load(path string) (Lock, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Lock{}, err
	}
	return Parse(b)
}

// Parse validates lock YAML.
func Parse(b []byte) (Lock, error) {
	var lock Lock
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&lock); err != nil {
		return Lock{}, fmt.Errorf("scope lock: %w", err)
	}
	sum := sha256.Sum256(b)
	lock.Hash = hex.EncodeToString(sum[:])
	if err := lock.normalize(); err != nil {
		return Lock{}, err
	}
	return lock, nil
}

func (l *Lock) normalize() error {
	if l.Version != "1.0" {
		return fmt.Errorf("scope lock version %q is not supported", l.Version)
	}
	switch l.ChangeType {
	case "bugfix", "feature", "refactor":
	default:
		return fmt.Errorf("change_type must be bugfix, feature, or refactor")
	}
	if l.SpecRef == "" {
		return fmt.Errorf("spec_ref is required")
	}
	if l.Blast.MaxFiles < 1 || l.Blast.MaxLoc < 1 {
		return fmt.Errorf("max_files and max_loc_delta must be positive")
	}
	if l.Blast.ChurnPct == 0 {
		l.Blast.ChurnPct = 15
	}
	if l.Blast.ChurnPct < 0 || l.Blast.ChurnPct > 100 {
		return fmt.Errorf("max_nonfunctional_churn_pct must be from 1 to 100")
	}
	if len(l.Blast.Allowed) == 0 {
		return fmt.Errorf("allowed_paths is required")
	}
	if _, err := globset.Compile(l.Blast.Allowed); err != nil {
		return err
	}
	if _, err := globset.Compile(l.Blast.Denied); err != nil {
		return err
	}
	for _, f := range l.Tests.Locked {
		if f.Path == "" || len(f.SHA256) != 64 {
			return fmt.Errorf("locked file %q needs a sha256 digest", f.Path)
		}
		for _, c := range f.SHA256 {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				return fmt.Errorf("locked file %q digest must be lowercase hex", f.Path)
			}
		}
	}
	return nil
}

// Save writes the lock as YAML.
func Save(path string, lock Lock) error {
	if err := lock.normalize(); err != nil {
		return err
	}
	b, err := yamlMarshal(lock)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// Generate builds a lock and hashes files selected by LockGlobs.
func Generate(opts Options) (Lock, error) {
	if opts.MaxFiles == 0 {
		opts.MaxFiles = 6
	}
	if opts.MaxLoc == 0 {
		opts.MaxLoc = 250
	}
	if opts.ChurnPct == 0 {
		opts.ChurnPct = 15
	}
	if opts.ChangeType == "" {
		opts.ChangeType = "bugfix"
	}
	deny := append([]string{}, opts.Deny...)
	if !contains(deny, ".proofsieve/**") {
		deny = append([]string{".proofsieve/**"}, deny...)
	}
	lock := Lock{
		Version: "1.0", SpecRef: opts.Spec, ChangeType: opts.ChangeType, Owner: opts.Owner,
		Blast: Blast{MaxFiles: opts.MaxFiles, MaxLoc: opts.MaxLoc, ChurnPct: opts.ChurnPct, Allowed: opts.Allow, Denied: deny},
	}
	locked, err := hashGlobs(opts.Repo, opts.LockGlobs)
	if err != nil {
		return Lock{}, err
	}
	lock.Tests.Locked = locked
	if err := lock.normalize(); err != nil {
		return Lock{}, err
	}
	return lock, nil
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func hashGlobs(repo string, globs []string) ([]LockedFile, error) {
	if len(globs) == 0 {
		return nil, nil
	}
	set, err := globset.Compile(globs)
	if err != nil {
		return nil, err
	}
	var found []LockedFile
	err = walkDir(repo, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "node_modules" || base == ".venv" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(repo, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !set.Match(rel) {
			return nil
		}
		sum, err := hashFile(path)
		if err != nil {
			return err
		}
		found = append(found, LockedFile{Path: rel, SHA256: sum})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("lock globs matched no files")
	}
	return found, nil
}

func hashFile(path string) (string, error) {
	b, err := readFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// Digest computes the sha256 of a file.
func Digest(path string) (string, error) {
	return hashFile(path)
}

// PolicyPaths are the files an agent must not rewrite.
func PolicyPaths() []string {
	return []string{".proofsieve/scope.lock.yaml", ".proofsieve/policy.yaml", ".proofsieve/ignore.json"}
}

// IsPolicyPath reports whether rel is inside the control-plane directory.
func IsPolicyPath(rel string) bool {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "./")
	return rel == ".proofsieve" || strings.HasPrefix(rel, ".proofsieve/")
}
