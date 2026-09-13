package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const vibeOutputLimit = 32 * 1024 * 1024
const vibeRowLimit = 100_000

var errVibeLimit = errors.New("Git output exceeds the 32 MiB preview limit")

type vibeLine struct {
	kind     byte
	old, new int
	text     string
}
type vibeHunk struct {
	header   string
	old, new int
	lines    []vibeLine
}
type vibeFile struct {
	path, oldPath, blob, status, note string // paths are repository-relative
	hunks                             []vibeHunk
	added, deleted                    int
	previewBlocked                    bool
}
type vibeSnapshot struct {
	root, head, branch string
	files              []vibeFile
	fingerprint        [32]byte
}

// A bounded writer stops Git rather than accumulating arbitrarily large patches.
type vibeBuffer struct {
	buffer   bytes.Buffer
	limit    int
	exceeded bool
}

func (b *vibeBuffer) Len() int       { return b.buffer.Len() }
func (b *vibeBuffer) Bytes() []byte  { return b.buffer.Bytes() }
func (b *vibeBuffer) String() string { return b.buffer.String() }

func (b *vibeBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		b.exceeded = true
		return 0, errVibeLimit
	}
	return b.buffer.Write(p)
}
func vibeGit(ctx context.Context, root string, limit int, args ...string) ([]byte, error) {
	return vibeGitInput(ctx, root, limit, "", args...)
}
func vibeGitInput(ctx context.Context, root string, limit int, input string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, gitBinary(), append([]string{"--no-optional-locks", "--literal-pathspecs", "-c", "core.quotepath=true"}, args...)...)
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(input)
	cmd.WaitDelay = time.Second
	out := &vibeBuffer{limit: limit}
	diagnostic := &vibeBuffer{limit: 8192}
	cmd.Stdout = out
	cmd.Stderr = diagnostic
	err := cmd.Run()
	if out.exceeded {
		return nil, errVibeLimit
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, errVibeLimit) {
			return nil, errVibeLimit
		}
		return nil, fmt.Errorf("git %s: %w %s", args[0], err, strings.TrimSpace(diagnostic.String()))
	}
	return out.Bytes(), nil
}

func readVibe(ctx context.Context, root string) (vibeSnapshot, error) {
	s := vibeSnapshot{root: root}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	head, err := vibeGit(ctx, root, 8192, "rev-parse", "--verify", "HEAD")
	if err != nil {
		// Only a valid unborn branch uses an empty tree; repository errors
		// must retain the previous snapshot instead of presenting all additions.
		branch, e := vibeGit(ctx, root, 8192, "symbolic-ref", "--short", "HEAD")
		if e != nil {
			return s, err
		}
		_, e = vibeGit(ctx, root, 8192, "show-ref", "--verify", "refs/heads/"+strings.TrimSpace(string(branch)))
		if e == nil {
			return s, err
		}
		empty, e := vibeGit(ctx, root, 8192, "hash-object", "-t", "tree", "--stdin")
		if e != nil {
			return s, e
		}
		s.head = strings.TrimSpace(string(empty))
		s.branch = strings.TrimSpace(string(branch)) + " (no commits)"
	} else {
		s.head = strings.TrimSpace(string(head))
		branch, e := vibeGit(ctx, root, 8192, "symbolic-ref", "--short", "HEAD")
		if e == nil {
			s.branch = strings.TrimSpace(string(branch))
		} else {
			s.branch = "detached " + s.head[:min(8, len(s.head))]
		}
	}
	flags := []string{"--no-color", "--no-ext-diff", "--no-textconv", "--no-relative", "--find-renames"}
	args := append([]string{"diff", "--raw", "-z", "--no-abbrev"}, flags...)
	args = append(args, s.head, "--")
	raw, err := vibeGit(ctx, root, vibeOutputLimit, args...)
	if err != nil {
		return s, err
	}
	s.files, err = parseVibeRaw(raw)
	if err != nil {
		return s, err
	}
	if err := limitVibeFiles(ctx, root, s.files); err != nil {
		return s, err
	}
	// Unmerged index stages are deliberately summaries, not an ambiguous
	// combined diff whose old line numbers do not refer to a single version.
	conflicts, err := vibeGit(ctx, root, vibeOutputLimit, "ls-files", "--unmerged", "-z")
	if err != nil {
		return s, err
	}
	conflicted := map[string]bool{}
	for _, entry := range strings.Split(string(conflicts), "\x00") {
		if _, path, ok := strings.Cut(entry, "\t"); ok {
			conflicted[path] = true
		}
	}
	byPath := map[string]*vibeFile{}
	for i := range s.files {
		f := &s.files[i]
		byPath[f.path] = f
		if conflicted[f.path] {
			f.status = "U"
			f.note = "Unresolved conflict; F3 opens the working file"
		}
	}
	if len(s.files) > 0 {
		args = append([]string{"diff", "--patch", "--unified=3", "--src-prefix=a/", "--dst-prefix=b/"}, flags...)
		args = append(args, s.head, "--")
		patch, e := vibeGit(ctx, root, vibeOutputLimit, args...)
		if errors.Is(e, errVibeLimit) {
			for i := range s.files {
				s.files[i].note = "Patch exceeds 32 MiB; F3 opens the file"
			}
		} else if e != nil {
			return s, e
		} else {
			parseVibePatch(patch, byPath)
		}
	}
	untracked, err := vibeGit(ctx, root, vibeOutputLimit, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return s, err
	}
	remaining := vibeRowLimit
	remainingBytes := vibeOutputLimit
	for i := range s.files {
		for _, h := range s.files[i].hunks {
			remaining -= len(h.lines)
			for _, l := range h.lines {
				remainingBytes -= len(l.text) + 1
			}
		}
	}
	for _, path := range strings.Split(string(untracked), "\x00") {
		if path == "" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return s, err
		}
		f := vibeFile{path: path, status: "?"}
		if remaining <= 0 || remainingBytes <= 0 {
			f.note = "Preview line limit reached"
		} else {
			readVibeUntracked(root, &f, remaining, remainingBytes)
		}
		for _, h := range f.hunks {
			remaining -= len(h.lines)
			for _, l := range h.lines {
				remainingBytes -= len(l.text) + 1
			}
		}
		s.files = append(s.files, f)
	}
	// Include metadata as well as text, so mode changes and branch switches
	// refresh the tree even if their visible patch is empty.
	digest := sha256.New()
	fmt.Fprintf(digest, "%s\x00%s\x00", s.head, s.branch)
	digest.Write(raw)
	for _, f := range s.files {
		fmt.Fprintf(digest, "%#v\x00", f)
	}
	copy(s.fingerprint[:], digest.Sum(nil))
	return s, nil
}

func parseVibeRaw(raw []byte) ([]vibeFile, error) {
	fields := strings.Split(string(raw), "\x00")
	var files []vibeFile
	seen := map[string]bool{}
	for i := 0; i < len(fields) && fields[i] != ""; {
		meta := strings.Fields(fields[i])
		i++
		if len(meta) != 5 || !strings.HasPrefix(meta[0], ":") || i >= len(fields) {
			return nil, errors.New("invalid Git raw diff")
		}
		f := vibeFile{path: fields[i], oldPath: fields[i], blob: meta[2], status: meta[4][:1], note: "Binary or metadata-only change"}
		i++
		if f.status == "R" || f.status == "C" {
			if i >= len(fields) {
				return nil, errors.New("incomplete Git rename")
			}
			f.path = fields[i]
			i++
		}
		if strings.Trim(f.blob, "0") == "" {
			f.blob = ""
		}
		if strings.TrimPrefix(meta[0], ":") == "160000" || meta[1] == "160000" {
			f.note = "Submodule change"
			f.previewBlocked = true
		} else if strings.TrimPrefix(meta[0], ":") == "120000" || meta[1] == "120000" {
			f.note = "Symbolic link change"
			f.previewBlocked = true
		}
		if seen[f.path] {
			continue
		}
		seen[f.path] = true
		files = append(files, f)
	}
	return files, nil
}

func vibePatchPath(s string) string {
	if strings.HasPrefix(s, "\"") {
		if decoded, err := strconv.Unquote(s); err == nil {
			s = decoded
		}
	}
	if s == "/dev/null" {
		return ""
	}
	if strings.HasPrefix(s, "a/") || strings.HasPrefix(s, "b/") {
		s = s[2:]
	}
	return s
}

// Query baseline blob sizes in one process. The text threshold applies to
// source files, even when a large file has only a tiny patch.
func limitVibeFiles(ctx context.Context, root string, files []vibeFile) error {
	var input strings.Builder
	for _, f := range files {
		if f.blob != "" && !f.previewBlocked {
			input.WriteString(f.blob + "\n")
		}
	}
	sizes := map[string]int64{}
	if input.Len() > 0 {
		out, err := vibeGitInput(ctx, root, vibeOutputLimit, input.String(), "cat-file", "--batch-check=%(objectname) %(objectsize)")
		if err != nil {
			return err
		}
		for line := range strings.SplitSeq(string(out), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 2 {
				if size, e := strconv.ParseInt(fields[1], 10, 64); e == nil {
					sizes[fields[0]] = size
				}
			}
		}
	}
	for i := range files {
		f := &files[i]
		if f.previewBlocked {
			continue
		}
		large := sizes[f.blob] > compareTextLimit
		if info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(f.path))); err == nil {
			large = large || info.Size() > compareTextLimit
		}
		if large {
			f.previewBlocked = true
			f.note = "File exceeds 5 MiB preview limit"
		}
	}
	return nil
}
func parseVibePatch(patch []byte, files map[string]*vibeFile) {
	var f *vibeFile
	var h *vibeHunk
	oldPath := ""
	old, newLine, total, used := 0, 0, 0, 0
	for line := range strings.SplitSeq(string(patch), "\n") {
		if strings.HasPrefix(line, "diff --git ") || strings.HasPrefix(line, "diff --cc ") {
			f = nil
			h = nil
			oldPath = ""
			used = 0
			continue
		}
		if h == nil && strings.HasPrefix(line, "--- ") {
			oldPath = vibePatchPath(line[4:])
			continue
		}
		if h == nil && strings.HasPrefix(line, "+++ ") {
			path := vibePatchPath(line[4:])
			if path == "" {
				path = oldPath
			}
			f = files[path]
			if f != nil && (f.status == "U" || f.previewBlocked) {
				f = nil
			}
			continue
		}
		if f == nil {
			continue
		}
		used += len(line) + 1
		if used > compareTextLimit || total >= vibeRowLimit {
			f.hunks = nil
			f.added = 0
			f.deleted = 0
			f.note = "Preview limit reached; F3 opens the file"
			f = nil
			h = nil
			continue
		}
		if strings.HasPrefix(line, "@@ ") {
			parts := strings.Fields(line)
			if len(parts) < 4 {
				continue
			}
			a, _, _ := strings.Cut(strings.TrimPrefix(parts[1], "-"), ",")
			b, _, _ := strings.Cut(strings.TrimPrefix(parts[2], "+"), ",")
			var err error
			old, err = strconv.Atoi(a)
			if err != nil {
				continue
			}
			newLine, err = strconv.Atoi(b)
			if err != nil {
				continue
			}
			f.hunks = append(f.hunks, vibeHunk{header: line, old: old, new: newLine})
			h = &f.hunks[len(f.hunks)-1]
			f.note = ""
			continue
		}
		if h == nil || len(line) == 0 {
			continue
		}
		entry := vibeLine{kind: line[0], text: line[1:]}
		switch entry.kind {
		case ' ':
			entry.old = old
			entry.new = newLine
			old++
			newLine++
		case '-':
			entry.old = old
			old++
			f.deleted++
		case '+':
			entry.new = newLine
			newLine++
			f.added++
		case '\\':
			entry.text = line
			entry.kind = '!'
		default:
			continue
		}
		if !utf8.ValidString(entry.text) || strings.IndexByte(entry.text, 0) >= 0 {
			f.hunks = nil
			f.added = 0
			f.deleted = 0
			f.note = "Unsupported text encoding; F3 opens the file"
			f = nil
			h = nil
			continue
		}
		h.lines = append(h.lines, entry)
		total++
	}
}

func readVibeUntracked(root string, f *vibeFile, remaining, byteBudget int) {
	path := filepath.Join(root, filepath.FromSlash(f.path))
	info, err := os.Lstat(path)
	if err != nil {
		f.note = err.Error()
		return
	}
	if !info.Mode().IsRegular() {
		f.note = "Non-regular file"
		return
	}
	if info.Size() > compareTextLimit {
		f.note = "File exceeds 5 MiB preview limit"
		return
	}
	if info.Size() > int64(byteBudget) {
		f.note = "Total preview size limit reached"
		return
	}
	file, err := os.Open(path)
	if err != nil {
		f.note = err.Error()
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, int64(min(compareTextLimit, byteBudget)+1)))
	if err != nil {
		f.note = err.Error()
		return
	}
	if len(data) > compareTextLimit {
		f.note = "File exceeds 5 MiB preview limit"
		return
	}
	if len(data) > byteBudget {
		f.note = "Total preview size limit reached"
		return
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		f.note = "Binary or unsupported text encoding"
		return
	}
	breaks := bytes.Count(data, []byte{'\n'}) + bytes.Count(data, []byte{'\r'}) - bytes.Count(data, []byte{'\r', '\n'})
	if breaks >= remaining {
		f.note = "Preview line limit reached"
		return
	}
	if len(data) == 0 {
		f.note = "Empty untracked file"
		return
	}
	lines := splitCompareLines(data)
	h := vibeHunk{header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), new: 1}
	for i, l := range lines {
		h.lines = append(h.lines, vibeLine{kind: '+', new: i + 1, text: l.text})
	}
	f.added = len(lines)
	f.hunks = []vibeHunk{h}
}
