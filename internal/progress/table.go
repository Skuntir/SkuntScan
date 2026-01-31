package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/term"
)

type Table struct {
	w     io.Writer
	isTTY bool

	apex string
	rows []row

	contentWidth int
	printedLines int

	spinFrames []string
	spinIdx    int

	stopCh chan struct{}
	wg     sync.WaitGroup

	mu sync.Mutex
}

type row struct {
	tool    string
	status  string
	t       time.Time
	started time.Time
	dur     time.Duration
	exit    int
	spin    int
}

func New(w io.Writer, apex string, tools []string) *Table {
	isTTY := detectTTY(w)
	contentWidth := 78
	if isTTY {
		if width, ok := detectTermWidth(w); ok {
			contentWidth = clamp(width-4, 20, 78)
		}
	}
	t := &Table{
		w:            w,
		isTTY:        isTTY,
		apex:         apex,
		contentWidth: contentWidth,
		printedLines: 0,
		spinFrames:   []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		stopCh:       make(chan struct{}),
	}
	for _, tool := range tools {
		t.rows = append(t.rows, row{tool: tool, status: "queued"})
	}
	return t
}

func (t *Table) Print() {
	t.mu.Lock()
	defer t.mu.Unlock()

	fmt.Fprintln(t.w, t.top())
	fmt.Fprintln(t.w, t.titleLine())
	fmt.Fprintln(t.w, t.metaLine())
	fmt.Fprintln(t.w, t.sep())
	fmt.Fprintln(t.w, t.headerLine())
	fmt.Fprintln(t.w, t.sep())
	for i := range t.rows {
		fmt.Fprintln(t.w, t.rowLine(i, time.Now()))
	}
	fmt.Fprintln(t.w, t.bottom())

	t.printedLines = 7 + len(t.rows)
	if t.isTTY {
		t.wg.Add(1)
		go t.spinLoop()
	}
}

func (t *Table) MarkStart(i int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if i < 0 || i >= len(t.rows) {
		return
	}
	now := time.Now()
	t.rows[i].status = "running"
	t.rows[i].t = now
	t.rows[i].started = now
	t.rows[i].dur = 0
	t.rows[i].exit = 0
	t.updateLine(i)
}

func (t *Table) MarkDone(i int, exit int, dur time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if i < 0 || i >= len(t.rows) {
		return
	}
	if exit == 0 {
		t.rows[i].status = "success"
	} else {
		t.rows[i].status = "fail"
	}
	t.rows[i].dur = dur
	t.rows[i].exit = exit
	t.updateLine(i)
}

func (t *Table) MarkFail(i int, exit int, dur time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if i < 0 || i >= len(t.rows) {
		return
	}
	t.rows[i].status = "fail"
	t.rows[i].dur = dur
	t.rows[i].exit = exit
	t.updateLine(i)
}

func (t *Table) MarkSkipped(i int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if i < 0 || i >= len(t.rows) {
		return
	}
	t.rows[i].status = "skipped"
	t.rows[i].t = time.Now()
	t.rows[i].started = time.Time{}
	t.rows[i].dur = 0
	t.rows[i].exit = 0
	t.updateLine(i)
}

func (t *Table) Close() {
	if t.isTTY {
		close(t.stopCh)
		t.wg.Wait()
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	fmt.Fprintln(t.w)
}

func (t *Table) updateLine(i int) {
	line := t.rowLine(i, time.Now())
	if !t.isTTY {
		fmt.Fprintln(t.w, line)
		return
	}
	if t.printedLines == 0 {
		return
	}
	lineNo := 7 + i
	up := t.printedLines + 1 - lineNo
	if up < 0 {
		up = 0
	}
	if up == 0 {
		fmt.Fprintf(t.w, "\r\x1b[2K%s\r", line)
		return
	}
	fmt.Fprintf(t.w, "\x1b[%dA\r\x1b[2K%s\r\x1b[%dB\r", up, line, up)
}

func (t *Table) spinLoop() {
	defer t.wg.Done()
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-t.stopCh:
			return
		case now := <-ticker.C:
			t.mu.Lock()
			if len(t.rows) == 0 {
				t.mu.Unlock()
				continue
			}
			found := -1
			for off := 0; off < len(t.rows); off++ {
				i := (t.spinIdx + off) % len(t.rows)
				if t.rows[i].status == "running" && !t.rows[i].started.IsZero() {
					found = i
					t.spinIdx = (i + 1) % len(t.rows)
					break
				}
			}
			if found >= 0 {
				t.rows[found].spin = (t.rows[found].spin + 1) % len(t.spinFrames)
				t.rows[found].dur = now.Sub(t.rows[found].started)
				t.updateLine(found)
			}
			t.mu.Unlock()
		}
	}
}

func (t *Table) top() string {
	return "┌" + strings.Repeat("─", t.contentWidth+2) + "┐"
}

func (t *Table) sep() string {
	return "├" + strings.Repeat("─", t.contentWidth+2) + "┤"
}

func (t *Table) bottom() string {
	return "└" + strings.Repeat("─", t.contentWidth+2) + "┘"
}

func (t *Table) titleLine() string {
	left := "SkuntScan"
	right := "skuntir.com"
	return t.boxLine(twoCols(left, right, t.contentWidth))
}

func (t *Table) metaLine() string {
	left := "Target: " + t.apex
	right := "Author: Kernelstub"
	return t.boxLine(twoCols(left, right, t.contentWidth))
}

func (t *Table) headerLine() string {
	content := joinCols([]col{
		{Text: "START", Width: 12},
		{Text: "TOOL", Width: 14},
		{Text: "STATUS", Width: 12},
		{Text: "DURATION", Width: 10},
		{Text: "EXIT", Width: 4},
	})
	return t.boxLine(content)
}

func (t *Table) rowLine(i int, now time.Time) string {
	r := t.rows[i]

	timeCol := ""
	if !r.started.IsZero() {
		timeCol = r.started.Format("15:04:05.000")
	} else if !r.t.IsZero() {
		timeCol = r.t.Format("15:04:05.000")
	}

	statusCol := r.status
	durCol := "-"
	exitCol := "-"

	switch r.status {
	case "running":
		statusCol = "running"
		if !r.started.IsZero() {
			durCol = FormatDuration(now.Sub(r.started))
		}
	case "success", "fail":
		durCol = FormatDuration(r.dur)
		exitCol = fmt.Sprintf("%d", r.exit)
	case "queued":
	case "skipped":
		durCol = FormatDuration(0)
		exitCol = "0"
	}

	line := joinCols([]col{
		{Text: timeCol, Width: 12},
		{Text: r.tool, Width: 14},
		{Text: statusCol, Width: 12},
		{Text: durCol, Width: 10},
		{Text: exitCol, Width: 4},
	})
	return t.boxLine(line)
}

func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "0ms"
	}
	if d < time.Minute {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	m := int(d / time.Minute)
	s := int(d/time.Second) % 60
	return fmt.Sprintf("%dm%02ds", m, s)
}

func detectTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func detectTermWidth(w io.Writer) (int, bool) {
	f, ok := w.(*os.File)
	if !ok {
		return 0, false
	}
	width, _, err := term.GetSize(int(f.Fd()))
	if err != nil || width <= 0 {
		return 0, false
	}
	return width, true
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func trunc(s string, width int) string {
	in := strings.TrimRight(s, "\r\n")
	if width <= 0 || runeLen(in) <= width {
		return in
	}
	if width <= 1 {
		return truncRunes(in, width)
	}
	return truncRunes(in, width-1) + "…"
}

func (t *Table) boxLine(content string) string {
	c := content
	if runeLen(c) > t.contentWidth {
		c = trunc(c, t.contentWidth)
	}
	if runeLen(c) < t.contentWidth {
		c += strings.Repeat(" ", t.contentWidth-runeLen(c))
	}
	return "│ " + c + " │"
}

func twoCols(left, right string, width int) string {
	l := strings.TrimRight(left, "\r\n")
	r := strings.TrimRight(right, "\r\n")
	if runeLen(l)+1+runeLen(r) > width {
		space := width - 1 - runeLen(r)
		if space < 0 {
			space = 0
		}
		l = trunc(l, space)
	}
	gap := width - runeLen(l) - runeLen(r)
	if gap < 1 {
		gap = 1
	}
	return l + strings.Repeat(" ", gap) + r
}

type col struct {
	Text  string
	Width int
}

func joinCols(cols []col) string {
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		out = append(out, padRight(trunc(c.Text, c.Width), c.Width))
	}
	return strings.Join(out, " ")
}

func padRight(s string, width int) string {
	in := strings.TrimRight(s, "\r\n")
	n := runeLen(in)
	if n >= width {
		return in
	}
	return in + strings.Repeat(" ", width-n)
}

func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

func truncRunes(s string, width int) string {
	if width <= 0 {
		return ""
	}
	i := 0
	for width > 0 && i < len(s) {
		_, size := utf8.DecodeRuneInString(s[i:])
		if size == 0 {
			break
		}
		i += size
		width--
	}
	return s[:i]
}
