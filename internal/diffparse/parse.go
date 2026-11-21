// Package diffparse reads unified git diffs.
package diffparse

import (
	"strconv"
	"strings"
)

// Hunk is one @@ region.
type Hunk struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	OldText  string
	NewText  string
}

// File is one path in a diff.
type File struct {
	OldPath string
	NewPath string
	Added   int
	Removed int
	Hunks   []Hunk
}

// Path is the path that exists after the change.
func (f File) Path() string {
	if f.NewPath == "" || f.NewPath == "/dev/null" {
		return f.OldPath
	}
	return f.NewPath
}

// Deleted reports a deletion.
func (f File) Deleted() bool {
	return f.NewPath == "/dev/null"
}

// AddedFile reports a new file.
func (f File) AddedFile() bool {
	return f.OldPath == "/dev/null"
}

// Parse parses `git diff -U0` output.
func Parse(unified string) ([]File, error) {
	lines := strings.Split(unified, "\n")
	var files []File
	var cur *File
	var hunk *Hunk
	flushHunk := func() {
		if cur == nil || hunk == nil {
			return
		}
		cur.Hunks = append(cur.Hunks, *hunk)
		hunk = nil
	}
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flushHunk()
			files = append(files, File{})
			cur = &files[len(files)-1]
		case strings.HasPrefix(line, "--- "):
			if cur != nil {
				cur.OldPath = cleanPath(strings.TrimPrefix(line, "--- "))
			}
		case strings.HasPrefix(line, "+++ "):
			if cur != nil {
				cur.NewPath = cleanPath(strings.TrimPrefix(line, "+++ "))
			}
		case strings.HasPrefix(line, "@@"):
			flushHunk()
			h, err := parseHunk(line)
			if err != nil {
				return nil, err
			}
			hunk = &h
		case strings.HasPrefix(line, `\`):
			continue
		case hunk != nil && strings.HasPrefix(line, "+"):
			hunk.NewText += line[1:] + "\n"
			if cur != nil {
				cur.Added++
			}
		case hunk != nil && strings.HasPrefix(line, "-"):
			hunk.OldText += line[1:] + "\n"
			if cur != nil {
				cur.Removed++
			}
		}
	}
	flushHunk()
	return files, nil
}

func cleanPath(raw string) string {
	raw = strings.TrimSpace(strings.Split(raw, "\t")[0])
	if raw == "/dev/null" {
		return raw
	}
	if strings.HasPrefix(raw, "a/") || strings.HasPrefix(raw, "b/") {
		return raw[2:]
	}
	return raw
}

func parseHunk(line string) (Hunk, error) {
	if !strings.HasPrefix(line, "@@") {
		return Hunk{}, errHunk(line)
	}
	rest := line[2:]
	second := strings.Index(rest, "@@")
	if second < 0 {
		return Hunk{}, errHunk(line)
	}
	body := strings.TrimSpace(rest[:second])
	parts := strings.Fields(body)
	if len(parts) < 2 {
		return Hunk{}, errHunk(line)
	}
	oldStart, oldLines, err := span(parts[0])
	if err != nil {
		return Hunk{}, err
	}
	newStart, newLines, err := span(parts[1])
	if err != nil {
		return Hunk{}, err
	}
	return Hunk{OldStart: oldStart, OldLines: oldLines, NewStart: newStart, NewLines: newLines}, nil
}

func span(s string) (int, int, error) {
	s = strings.TrimPrefix(s, "+")
	s = strings.TrimPrefix(s, "-")
	startS, countS, ok := strings.Cut(s, ",")
	start, err := strconv.Atoi(startS)
	if err != nil {
		return 0, 0, err
	}
	if !ok {
		if start == 0 {
			return 0, 0, nil
		}
		return start, 1, nil
	}
	count, err := strconv.Atoi(countS)
	if err != nil {
		return 0, 0, err
	}
	return start, count, nil
}

func errHunk(line string) error {
	return &parseError{line}
}

type parseError struct{ line string }

func (e *parseError) Error() string { return "bad hunk header: " + e.line }

// NewLines returns 1-based line numbers on the new side of a file.
func NewLines(f File) map[int]bool {
	out := map[int]bool{}
	for _, h := range f.Hunks {
		if h.NewText == "" {
			continue
		}
		rows := strings.Split(strings.TrimSuffix(h.NewText, "\n"), "\n")
		for i := range rows {
			out[h.NewStart+i] = true
		}
	}
	return out
}
