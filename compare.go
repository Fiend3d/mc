package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"unicode/utf8"

	"mc/internal/event"
)

const compareTextLimit = 5 * 1024 * 1024
const compareWorkLimit = 8_000_000

var errCompareBudget = errors.New("detailed comparison work limit reached")

// Comparison results belong to the worker until delivered, then to the UI.
type compareLine struct{ text, ending string }
type compareRow struct {
	left, right           int // one-based source lines; zero is an alignment gap
	changed               bool
	leftMarks, rightMarks []bool // changed rune positions
}
type compareResult struct {
	lines [2][]compareLine
	rows  []compareRow
	hunks []int
	sizes [2]int64
	equal bool
	note  string
	err   error
}
type comparison struct {
	paths             [2]string
	generation        uint64
	cancel            context.CancelFunc
	loading           bool
	result            compareResult
	start, horizontal int
}
type compareDoneMsg struct {
	generation uint64
	result     compareResult
}

func (c *comparison) stop() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}

func (m *model) openCompare() (event.Model, event.Cmd) {
	var paths [2]string
	for side, p := range m.panes {
		label := []string{"left", "right"}[side]
		if p == nil || !p.hasTabs() {
			return m, m.addMessage(msgWarning, "Compare: no file in the "+label+" pane")
		}
		t := p.tabs[p.currentTab]
		items, cursor := t.page.getItems(), t.getPageSettings().cursor
		if cursor < 0 || cursor >= len(items) || items[cursor].isDirectory() {
			return m, m.addMessage(msgWarning, "Compare: place the "+label+" cursor on a file")
		}
		paths[side] = items[cursor].getFullPath()
	}
	m.compare.paths = paths
	m.mode = compareMode
	return m, m.loadComparison()
}

func (m *model) loadComparison() event.Cmd {
	c := &m.compare
	c.stop()
	c.generation++
	c.loading = true
	c.start, c.horizontal = 0, 0
	c.result = compareResult{}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	paths, generation := c.paths, c.generation
	return func() event.Msg {
		defer cancel()
		return compareDoneMsg{generation, compareFiles(ctx, paths)}
	}
}

func (m *model) applyComparison(msg compareDoneMsg) {
	c := &m.compare
	if m.mode != compareMode || msg.generation != c.generation {
		return
	}
	c.stop()
	c.loading, c.result = false, msg.result
	if len(c.result.hunks) > 0 {
		c.start = c.result.hunks[0]
	}
}

func (m *model) compareRows() int {
	h := m.screenHeight
	if m.taskStripVisible() {
		h--
	}
	return max(1, h-4)
}
func (c *comparison) scroll(delta, height int) {
	// A hunk can be above the usual last full viewport. Scrolling from it
	// must stay monotonic rather than jumping backwards on a down key.
	limit := max(c.start, max(0, len(c.result.rows)-height))
	c.start = min(max(0, c.start+delta), limit)
}
func (m *model) handleCompare(key string) (event.Model, event.Cmd) {
	c := &m.compare
	switch key {
	case "esc", "q":
		c.stop()
		c.generation++
		c.result = compareResult{}
		c.loading = false
		m.mode = normalMode
	case "f5":
		return m, m.loadComparison()
	case "w":
		m.taskView = true
		m.taskCursor = max(0, len(m.taskList)-1)
	case "j", "down":
		c.scroll(1, m.compareRows())
	case "k", "up":
		c.scroll(-1, m.compareRows())
	case "pgdown":
		c.scroll(m.compareRows(), m.compareRows())
	case "pgup":
		c.scroll(-m.compareRows(), m.compareRows())
	case "home":
		c.start = 0
	case "end":
		c.start = max(0, len(c.result.rows)-m.compareRows())
	case "h", "left":
		c.horizontal = max(0, c.horizontal-4)
	case "l", "right":
		c.horizontal = min(compareTextLimit*4, c.horizontal+4)
	case "n", "p":
		if len(c.result.hunks) == 0 {
			break
		}
		// Hunk jumps keep their target at the top, even near the file end.
		index := c.hunkAtStart()
		if key == "n" {
			index++
		} else {
			index--
		}
		index = min(max(0, index), len(c.result.hunks)-1)
		c.start = c.result.hunks[index]
		// Hunk jumps can leave blank rows below, keeping the target at top.
	}
	return m, nil
}

func (c *comparison) hunkAtStart() int {
	index := -1
	for i, row := range c.result.hunks {
		if row > c.start {
			break
		}
		index = i
	}
	return index
}

// Read in bounded chunks so cancellation and size limits also apply to files
// that grow after Stat. Retain only the text prefix; byte equality is streamed.
func compareFiles(ctx context.Context, paths [2]string) (result compareResult) {
	var files [2]*os.File
	for i, path := range paths {
		if err := ctx.Err(); err != nil {
			result.err = err
			return
		}
		f, err := os.Open(path)
		if err != nil {
			result.err = err
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			result.err = err
			return
		}
		if !info.Mode().IsRegular() {
			result.err = fmt.Errorf("%s is not a regular file", path)
			return
		}
		files[i] = f
	}
	var data [2][]byte
	var buffers [2][64 * 1024]byte
	var done [2]bool
	result.equal = true
	for !done[0] || !done[1] {
		if err := ctx.Err(); err != nil {
			result.err = err
			return
		}
		var count [2]int
		for i := range files {
			if done[i] {
				continue
			}
			n, err := io.ReadFull(files[i], buffers[i][:])
			if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				result.err = err
				return
			}
			done[i] = err != nil
			count[i] = n
			result.sizes[i] += int64(n)
			if len(data[i]) <= compareTextLimit {
				keep := min(n, compareTextLimit+1-len(data[i]))
				data[i] = append(data[i], buffers[i][:keep]...)
			}
		}
		if !bytes.Equal(buffers[0][:count[0]], buffers[1][:count[1]]) {
			result.equal = false
		}
	}
	for _, d := range data {
		if len(d) > compareTextLimit {
			result.note = "Detailed text comparison is limited to 5 MiB per file."
			return
		}
		if bytes.IndexByte(d, 0) >= 0 || !utf8.Valid(d) {
			result.note = "Binary or unsupported encoding; byte comparison only."
			return
		}
	}
	return compareText(ctx, data, result)
}

func splitCompareLines(data []byte) []compareLine {
	var lines []compareLine
	text := strings.TrimPrefix(string(data), "\ufeff")
	for len(text) > 0 {
		pos := strings.IndexAny(text, "\r\n")
		if pos < 0 {
			lines = append(lines, compareLine{text, ""})
			break
		}
		end := pos + 1
		if text[pos] == '\r' && end < len(text) && text[end] == '\n' {
			end++
		}
		lines = append(lines, compareLine{text[:pos], text[pos:end]})
		text = text[end:]
	}
	return lines
}

func compareText(ctx context.Context, data [2][]byte, result compareResult) compareResult {
	// Bound row storage as well as bytes: millions of empty lines would
	// otherwise amplify a small input into hundreds of MiB of row metadata.
	for _, d := range data {
		breaks := bytes.Count(d, []byte{'\n'}) + bytes.Count(d, []byte{'\r'}) - bytes.Count(d, []byte{'\r', '\n'})
		if breaks > 100_000 {
			result.note = "Detailed comparison line limit reached; byte comparison only."
			return result
		}
	}
	for i := range data {
		result.lines[i] = splitCompareLines(data[i])
	}
	left, right := result.lines[0], result.lines[1]
	budget := compareWorkLimit
	ops, err := compareEdits(ctx, len(left), len(right), func(a, b int) bool { return left[a] == right[b] }, &budget)
	if err != nil {
		result.lines = [2][]compareLine{}
		if errors.Is(err, errCompareBudget) {
			result.note = "Detailed comparison work limit reached; byte comparison only."
		} else {
			result.err = err
		}
		return result
	}
	a, b := 0, 0
	for pos := 0; pos < len(ops); {
		if ops[pos] == '=' {
			a++
			b++
			result.rows = append(result.rows, compareRow{left: a, right: b})
			pos++
			continue
		}
		result.hunks = append(result.hunks, len(result.rows))
		startA, startB := a, b
		for pos < len(ops) && ops[pos] != '=' {
			if ops[pos] == '-' {
				a++
			} else {
				b++
			}
			pos++
		}
		for i := 0; i < max(a-startA, b-startB); i++ {
			row := compareRow{changed: true}
			if startA+i < a {
				row.left = startA + i + 1
			}
			if startB+i < b {
				row.right = startB + i + 1
			}
			if row.left > 0 && row.right > 0 {
				row.leftMarks, row.rightMarks = compareInline(ctx, left[row.left-1].text, right[row.right-1].text, &budget)
			}
			result.rows = append(result.rows, row)
		}
	}
	if bytes.HasPrefix(data[0], []byte("\xef\xbb\xbf")) != bytes.HasPrefix(data[1], []byte("\xef\xbb\xbf")) {
		result.note = "UTF-8 BOM differs."
	}
	if err := ctx.Err(); err != nil {
		result.err = err
	}
	return result
}

// Myers' shortest edit script. Compact diagonal traces cap memory separately
// from the work counter; every traversal, including equal snakes, spends work.
func compareEdits(ctx context.Context, n, m int, equal func(int, int) bool, budget *int) ([]byte, error) {
	var trace [][]int
	stored := 0
	for d := 0; d <= n+m; d++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		stored += 2*d + 3
		if stored > 1_000_000 {
			return nil, errCompareBudget
		}
		v := make([]int, 2*d+3)
		get := func(k int) int {
			if d == 0 {
				return 0
			}
			return trace[d-1][k+d]
		}
		for k := -d; k <= d; k += 2 {
			*budget--
			if *budget < 0 {
				return nil, errCompareBudget
			}
			x := 0
			if k == -d || (k != d && get(k-1) < get(k+1)) {
				x = get(k + 1)
			} else {
				x = get(k-1) + 1
			}
			y := x - k
			for x < n && y < m && equal(x, y) {
				x++
				y++
				*budget--
				if *budget < 0 {
					return nil, errCompareBudget
				}
				if *budget&4095 == 0 {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
				}
			}
			v[k+d+1] = x
			if x >= n && y >= m {
				var ops []byte
				for depth := d; depth > 0; depth-- {
					prev := trace[depth-1]
					k := x - y
					prevK := k - 1
					if k == -depth || (k != depth && prev[k+depth-1] < prev[k+depth+1]) {
						prevK = k + 1
					}
					px := prev[prevK+depth]
					py := px - prevK
					for x > px && y > py {
						ops = append(ops, '=')
						x--
						y--
					}
					if x == px {
						ops = append(ops, '+')
						y--
					} else {
						ops = append(ops, '-')
						x--
					}
				}
				for x > 0 && y > 0 {
					ops = append(ops, '=')
					x--
					y--
				}
				slices.Reverse(ops)
				return ops, nil
			}
		}
		trace = append(trace, v)
	}
	return nil, errCompareBudget
}

func compareInline(ctx context.Context, left, right string, budget *int) ([]bool, []bool) {
	a, b := []rune(left), []rune(right)
	ma, mb := make([]bool, len(a)), make([]bool, len(b))
	// Large individual lines still receive an accurate prefix/suffix highlight.
	start, endA, endB := 0, len(a), len(b)
	for start < endA && start < endB && a[start] == b[start] {
		start++
	}
	for endA > start && endB > start && a[endA-1] == b[endB-1] {
		endA--
		endB--
	}
	for i := start; i < endA; i++ {
		ma[i] = true
	}
	for i := start; i < endB; i++ {
		mb[i] = true
	}
	if endA-start+endB-start > 4096 || *budget <= 0 {
		return ma, mb
	}
	ops, err := compareEdits(ctx, endA-start, endB-start, func(i, j int) bool { return a[start+i] == b[start+j] }, budget)
	if err != nil {
		return ma, mb
	}
	i, j := start, start
	for _, op := range ops {
		switch op {
		case '=':
			ma[i], mb[j] = false, false
			i++
			j++
		case '-':
			i++
		case '+':
			j++
		}
	}
	return ma, mb
}
