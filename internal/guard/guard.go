// Package guard is the pre-tool hook entrypoint.
package guard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vaquarkhan/ProofSieve/internal/lockfile"
	"github.com/vaquarkhan/ProofSieve/internal/pathutil"
	"github.com/vaquarkhan/ProofSieve/internal/scope"
	"github.com/vaquarkhan/ProofSieve/internal/session"
)

// Options selects the repository and lock.
type Options struct {
	Repo     string
	LockPath string
	Now      time.Time
}

type hook struct {
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
	Cwd       string         `json:"cwd"`
	Command   string         `json:"command"`
}

// Run reads one hook payload and writes an allow or deny decision.
// It returns 0 or 2. Any failure denies the tool call.
func Run(r io.Reader, stdout, stderr io.Writer, opts Options) int {
	raw, err := io.ReadAll(r)
	if err != nil {
		return deny(stdout, stderr, "hook payload could not be read")
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	var h hook
	if err := json.Unmarshal(raw, &h); err != nil {
		return deny(stdout, stderr, "hook payload is not valid JSON")
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	repo := opts.Repo
	if repo == "" {
		repo = h.Cwd
	}
	if repo == "" {
		wd, err := getwd()
		if err != nil {
			return deny(stdout, stderr, "repository path is unknown")
		}
		repo = wd
	}
	lockPath := opts.LockPath
	if lockPath == "" {
		lockPath = filepath.Join(repo, ".proofsieve", "scope.lock.yaml")
	}
	lock, err := lockfile.Load(lockPath)
	if err != nil {
		return deny(stdout, stderr, "scope lock is missing or invalid")
	}
	command := h.Command
	if command == "" {
		command = str(h.ToolInput, "command")
	}
	if command != "" && !isWrite(h.ToolName) {
		if rule, msg, bad := shellBlocked(command, lock); bad {
			return deny(stdout, stderr, rule+": "+msg)
		}
		return allow(stdout)
	}
	path := first(h.ToolInput, "file_path", "path", "target_file")
	if path == "" {
		return deny(stdout, stderr, "hook payload has no file path")
	}
	rel, err := pathutil.Rel(repo, path)
	if err != nil {
		return deny(stdout, stderr, err.Error())
	}
	loc, newFile := churnFor(repo, rel, h)
	sessPath := filepath.Join(repo, ".proofsieve", "run", "session.json")
	var decision scope.Decision
	err = session.WithLock(sessPath, func() error {
		sess, err := session.Load(sessPath, lock.Hash, opts.Now)
		if err != nil {
			return err
		}
		files := sess.Count()
		if _, ok := sess.Files[rel]; ok {
			files--
		}
		projected := sess.Loc() + loc
		if str(h.ToolInput, "contents") != "" {
			projected = sess.Loc() - sess.Files[rel] + loc
		}
		decision = scope.Decide(lock, rel, files, projected, newFile && sess.Files[rel] == 0)
		if !decision.Allow {
			return nil
		}
		replace := str(h.ToolInput, "contents") != ""
		sess.Charge(rel, loc, replace)
		sess.Updated = opts.Now
		return sess.Save(sessPath)
	})
	if err != nil {
		return deny(stdout, stderr, "scope session could not be updated")
	}
	if !decision.Allow {
		return deny(stdout, stderr, decision.Rule+": "+decision.Message)
	}
	return allow(stdout)
}

var getwd = os.Getwd

func isWrite(name string) bool {
	switch strings.ToLower(name) {
	case "write", "edit", "delete", "strreplace", "applypatch":
		return true
	default:
		return false
	}
}

func churnFor(repo, rel string, h hook) (int, bool) {
	full := filepath.Join(repo, filepath.FromSlash(rel))
	oldBytes, err := os.ReadFile(full)
	newFile := err != nil
	old := string(oldBytes)
	if contents, ok := h.ToolInput["contents"]; ok {
		return lineChurn(old, fmt.Sprint(contents)), newFile
	}
	if _, ok := h.ToolInput["old_string"]; ok {
		return lineChurn(str(h.ToolInput, "old_string"), str(h.ToolInput, "new_string")), newFile
	}
	if strings.EqualFold(h.ToolName, "delete") {
		return lineChurn(old, ""), false
	}
	return 1, newFile
}

func lineChurn(old, newer string) int {
	ob, nb := bag(old), bag(newer)
	n := 0
	for line, count := range nb {
		if ob[line] < count {
			n += count - ob[line]
		}
	}
	for line, count := range ob {
		if nb[line] < count {
			n += count - ob[line]
		}
	}
	return n
}

func bag(s string) map[string]int {
	out := map[string]int{}
	if s == "" {
		return out
	}
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		out[line]++
	}
	return out
}

func shellBlocked(command string, lock lockfile.Lock) (string, string, bool) {
	text := strings.ReplaceAll(command, "\\", "/")
	if strings.Contains(text, ".proofsieve") {
		return "policy-path", "command mentions .proofsieve", true
	}
	for _, pattern := range lock.Blast.Denied {
		prefix := strings.TrimSuffix(strings.TrimSuffix(pattern, "/**"), "/*")
		if prefix == "" || prefix == "**" || strings.Contains(prefix, "*") {
			continue
		}
		if strings.Contains(text, prefix) {
			return "denied-path", "command mentions " + prefix, true
		}
	}
	for _, tok := range strings.Fields(text) {
		tok = strings.Trim(tok, "\"'`")
		if !looksPath(tok) {
			continue
		}
		dec := scope.Decide(lock, strings.TrimPrefix(tok, "./"), 0, 0, false)
		if !dec.Allow && dec.Rule != "max-files" && dec.Rule != "max-loc" {
			return dec.Rule, dec.Message, true
		}
	}
	return "", "", false
}

func looksPath(tok string) bool {
	if tok == "" || tok == "." || tok == ".." {
		return false
	}
	return strings.Contains(tok, "/") || strings.Contains(filepath.Ext(tok), ".")
}

func str(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func first(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if s := str(m, key); s != "" {
			return s
		}
	}
	return ""
}

func allow(stdout io.Writer) int {
	write(stdout, "allow", "")
	return 0
}

func deny(stdout, stderr io.Writer, msg string) int {
	write(stdout, "deny", msg)
	fmt.Fprintln(stderr, msg)
	return 2
}

func write(w io.Writer, perm, msg string) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"permission":    perm,
		"user_message":  msg,
		"agent_message": msg,
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       perm,
			"permissionDecisionReason": msg,
		},
	})
}
