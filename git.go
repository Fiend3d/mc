package main

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"mc/shutil"
)

// gitState is what git says about one entry in the listing. The values are
// ordered by severity: when several states roll up onto one directory, the
// lowest value wins, so a conflict is never hidden behind an untracked file.
type gitState byte

const (
	gitNone gitState = iota
	gitConflicted
	gitDeleted
	gitModified
	gitAdded
	gitRenamed
	gitUntracked
	gitIgnored
)

// gitTimeout caps a status read. A repository big enough to take longer is one
// where the markers are not worth blocking a goroutine over.
const gitTimeout = 5 * time.Second

// letter is the marker shown in the listing. Ignored entries have none: they
// are dimmed instead, and a column of "!" beside every build artefact is noise.
func (s gitState) letter() string {
	switch s {
	case gitConflicted:
		return "U"
	case gitDeleted:
		return "D"
	case gitModified:
		return "M"
	case gitAdded:
		return "A"
	case gitRenamed:
		return "R"
	case gitUntracked:
		return "?"
	}
	return " "
}

// name is the long form, for the pane footer.
func (s gitState) name() string {
	switch s {
	case gitConflicted:
		return "conflicted"
	case gitDeleted:
		return "deleted"
	case gitModified:
		return "modified"
	case gitAdded:
		return "added"
	case gitRenamed:
		return "renamed"
	case gitUntracked:
		return "untracked"
	case gitIgnored:
		return "ignored"
	}
	return ""
}

// gitEntry is one changed file, wherever in the repository it sits.
type gitEntry struct {
	path  string
	state gitState
}

// gitInfo is one directory's worth of status: where the repository is, what it
// is on, and the state of everything listed in that directory.
type gitInfo struct {
	root   string
	branch string
	ahead  int
	behind int

	// states is keyed by full path, already rolled up onto the entries of the
	// directory that was read.
	states map[string]gitState

	// entries is every changed file in the repository, in git's own order, and
	// is what the lists behind the tallies are made of.
	entries []gitEntry

	added     int
	modified  int
	untracked int
}

// gitTally groups the states the way the summary counts them, so a list opened
// from a tally holds exactly as many entries as the tally claims. Ignored
// entries belong to no tally and never reach a list.
func gitTally(state gitState) gitState {
	switch state {
	case gitAdded, gitRenamed:
		return gitAdded
	case gitModified, gitDeleted, gitConflicted:
		return gitModified
	case gitUntracked:
		return gitUntracked
	}
	return gitNone
}

// tallyName names a tally for a header or a message.
func tallyName(tally gitState) string {
	switch tally {
	case gitAdded:
		return "added"
	case gitModified:
		return "modified"
	case gitUntracked:
		return "untracked"
	}
	return "changed"
}

// changed lists the repository's entries belonging to one tally.
func (info *gitInfo) changed(tally gitState) []gitEntry {
	if info == nil {
		return nil
	}
	var out []gitEntry
	for _, e := range info.entries {
		if gitTally(e.state) == tally {
			out = append(out, e)
		}
	}
	return out
}

// gitBinary is the git executable, or "" when git is not on PATH. Everything
// else checks this first, so mc without git behaves exactly as it did before.
var gitBinary = sync.OnceValue(func() string {
	path, err := exec.LookPath("git")
	if err != nil {
		return ""
	}
	return path
})

// findRepoRoot walks up from dir looking for .git, which is a directory in a
// normal clone and a file in a worktree or submodule. Doing this in Go keeps
// mc from spawning git at all outside a repository -- the common case for
// C:\Windows, drive listings and network shares.
func findRepoRoot(dir string) string {
	if dir == "" || isUNCRoot(dir) {
		return ""
	}
	for current := filepath.Clean(dir); ; {
		if shutil.PathExists(filepath.Join(current, ".git")) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

// readGitInfo asks git about dir. The porcelain v1 format is stable by
// contract, -z removes all quoting questions, and --ignored in its default
// mode collapses a wholly ignored directory into a single record instead of
// listing everything inside it.
func readGitInfo(ctx context.Context, dir, root string) (*gitInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	// --no-optional-locks keeps the read from refreshing the index on disk.
	// Without it a status bumps .git's timestamp, mc's own watcher sees the
	// directory change, re-reads it and asks git again -- a loop that rebuilds
	// the listing under the pointer for as long as the tab is open.
	cmd := exec.CommandContext(ctx, gitBinary(),
		"--no-optional-locks", "-c", "core.quotepath=off",
		"status", "--porcelain=v1", "-z",
		"--branch", "--untracked-files=normal", "--ignored")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseGitStatus(out, root, dir), nil
}

// parseGitStatus turns porcelain output into the states of the entries listed
// in dir. Paths in the output are relative to the repository root and use
// forward slashes; anything deeper than dir is rolled up onto the directory
// inside dir that contains it.
func parseGitStatus(out []byte, root, dir string) *gitInfo {
	info := &gitInfo{root: root, states: map[string]gitState{}}
	records := strings.Split(string(bytes.TrimRight(out, "\x00")), "\x00")
	for i := 0; i < len(records); i++ {
		record := records[i]
		if record == "" {
			continue
		}
		if strings.HasPrefix(record, "## ") {
			info.readBranch(record[3:])
			continue
		}
		if len(record) < 4 {
			continue
		}
		// "XY path"; a rename also carries its original path in the next
		// record, which is of no interest here but must not be read as one.
		x, y, path := record[0], record[1], record[3:]
		if x == 'R' || y == 'R' {
			i++
		}
		state := recordState(x, y)
		if state == gitNone {
			continue
		}
		info.count(state)
		if full := absEntry(path, root); full != "" && state != gitIgnored {
			info.entries = append(info.entries, gitEntry{full, state})
		}
		if target := entryInDir(path, root, dir); target != "" {
			if old, ok := info.states[target]; !ok || state < old {
				info.states[target] = state
			}
		}
	}
	return info
}

// recordState reduces a two-letter status code to one state. Unmerged paths
// are any code with a U in it, plus the AA and DD pairs.
func recordState(x, y byte) gitState {
	switch {
	case x == '!' && y == '!':
		return gitIgnored
	case x == '?' && y == '?':
		return gitUntracked
	case x == 'U' || y == 'U' || (x == 'A' && y == 'A') || (x == 'D' && y == 'D'):
		return gitConflicted
	}
	// The index is reported first, so staged work outranks the same letter in
	// the work tree; either column alone is enough to mark the entry.
	for _, c := range []byte{x, y} {
		switch c {
		case 'D':
			return gitDeleted
		case 'M', 'T':
			return gitModified
		case 'A':
			return gitAdded
		case 'R', 'C':
			return gitRenamed
		}
	}
	return gitNone
}

// absEntry is a record's path as the rest of mc spells paths: absolute, with
// the separators this platform uses, and without the slash git puts on a
// directory it has collapsed.
func absEntry(path, root string) string {
	return filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(path, "/")))
}

// entryInDir maps a repository-relative path onto the entry of dir that holds
// it: the path itself when it sits directly inside dir, otherwise the
// directory below dir that contains it. Paths outside dir map to nothing.
func entryInDir(path, root, dir string) string {
	full := absEntry(path, root)
	rest, ok := relativeTo(full, dir)
	if !ok {
		return ""
	}
	if i := strings.IndexByte(rest, filepath.Separator); i >= 0 {
		rest = rest[:i]
	}
	return filepath.Join(dir, rest)
}

// relativeTo reports path's remainder under dir, case-insensitively as the
// rest of mc compares paths. An exact match is not an entry of dir.
func relativeTo(path, dir string) (string, bool) {
	path, dir = filepath.Clean(path), filepath.Clean(dir)
	if !strings.HasSuffix(dir, string(filepath.Separator)) {
		dir += string(filepath.Separator)
	}
	if len(path) <= len(dir) || !strings.EqualFold(path[:len(dir)], dir) {
		return "", false
	}
	return path[len(dir):], true
}

// readBranch reads the "## main...origin/main [ahead 1, behind 2]" header,
// which also has a form for a detached HEAD and one for a repository that has
// yet to be committed to.
func (info *gitInfo) readBranch(header string) {
	if strings.HasPrefix(header, "HEAD (no branch)") {
		info.branch = "HEAD"
		return
	}
	if name, ok := strings.CutPrefix(header, "No commits yet on "); ok {
		info.branch = strings.TrimSpace(name)
		return
	}
	branch, tracking, _ := strings.Cut(header, "...")
	info.branch = strings.TrimSpace(branch)
	_, counts, ok := strings.Cut(tracking, "[")
	if !ok {
		return
	}
	counts = strings.TrimSuffix(counts, "]")
	for _, part := range strings.Split(counts, ", ") {
		value, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(part, "ahead"), "behind")))
		if err != nil {
			continue
		}
		if strings.HasPrefix(part, "ahead") {
			info.ahead = value
		} else if strings.HasPrefix(part, "behind") {
			info.behind = value
		}
	}
}

func (info *gitInfo) count(state gitState) {
	switch gitTally(state) {
	case gitAdded:
		info.added++
	case gitModified:
		info.modified++
	case gitUntracked:
		info.untracked++
	}
}

// dirty reports whether the work tree holds anything but committed files.
func (info *gitInfo) dirty() bool {
	return info != nil && info.added+info.modified+info.untracked > 0
}

// gitSegment is one piece of the path row's summary. A segment with a tally
// stands for a list of files and can be pointed at and clicked; the branch and
// the arrows carry gitNone, because commits are not files.
type gitSegment struct {
	text  string
	tally gitState
}

// segments breaks the summary into its pieces. Rendering, hover and hit
// testing all walk this, so what can be clicked is defined in one place.
func (info *gitInfo) segments() []gitSegment {
	if info == nil {
		return nil
	}
	branch := info.branch
	if branch == "" {
		branch = "git"
	}
	if info.ahead > 0 {
		branch += "↑" + strconv.Itoa(info.ahead)
	}
	if info.behind > 0 {
		branch += "↓" + strconv.Itoa(info.behind)
	}
	segments := []gitSegment{{branch, gitNone}}
	for _, tally := range []struct {
		count int
		mark  string
		state gitState
	}{
		{info.added, "+", gitAdded},
		{info.modified, "~", gitModified},
		{info.untracked, "?", gitUntracked},
	} {
		if tally.count > 0 {
			segments = append(segments, gitSegment{" " + tally.mark + strconv.Itoa(tally.count), tally.state})
		}
	}
	return segments
}

// summary is the branch and the tallies as one string, for the pane's path row.
func (info *gitInfo) summary() string {
	s := ""
	for _, segment := range info.segments() {
		s += segment.text
	}
	return s
}
