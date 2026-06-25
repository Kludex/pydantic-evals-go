package evals

import (
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

// RenderOptions configures [EvaluationReport.Render] and [EvaluationReport.Print].
type RenderOptions struct {
	IncludeInput          bool
	IncludeMetadata       bool
	IncludeExpectedOutput bool
	IncludeOutput         bool
	IncludeDurations      bool
	IncludeTotalDuration  bool
	IncludeAverages       bool
	IncludeReasons        bool

	// Title overrides the default "Evaluation Summary: <name>" title. An empty
	// Title uses the default; set OmitTitle to render no title at all.
	Title string
	// OmitTitle renders the table without a title row.
	OmitTitle bool
}

// DefaultRenderOptions returns the options used when none are supplied: durations
// and averages included, matching the Python defaults.
func DefaultRenderOptions() RenderOptions {
	return RenderOptions{IncludeDurations: true, IncludeAverages: true}
}

// stringFmt formats a value for display in a report cell.
func stringFmt(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case Scalar:
		return t.String()
	default:
		return sprintValue(v)
	}
}

// Render returns the report rendered as a box-drawing table string.
func (r *EvaluationReport[I, O, M]) Render(opts ...RenderOptions) string {
	o := DefaultRenderOptions()
	if len(opts) > 0 {
		o = opts[0]
	}

	includeScores := anyCaseHas(r.Cases, func(c ReportCase[I, O, M]) bool { return len(c.Scores) > 0 })
	includeLabels := anyCaseHas(r.Cases, func(c ReportCase[I, O, M]) bool { return len(c.Labels) > 0 })
	includeMetrics := anyCaseHas(r.Cases, func(c ReportCase[I, O, M]) bool { return len(c.Metrics) > 0 })
	includeAssertions := anyCaseHas(r.Cases, func(c ReportCase[I, O, M]) bool { return len(c.Assertions) > 0 })
	includeEvalFailures := anyCaseHas(r.Cases, func(c ReportCase[I, O, M]) bool { return len(c.EvaluatorFailures) > 0 })

	var headers []string
	headers = append(headers, "Case ID")
	if o.IncludeInput {
		headers = append(headers, "Inputs")
	}
	if o.IncludeMetadata {
		headers = append(headers, "Metadata")
	}
	if o.IncludeExpectedOutput {
		headers = append(headers, "Expected Output")
	}
	if o.IncludeOutput {
		headers = append(headers, "Outputs")
	}
	if includeScores {
		headers = append(headers, "Scores")
	}
	if includeLabels {
		headers = append(headers, "Labels")
	}
	if includeMetrics {
		headers = append(headers, "Metrics")
	}
	if includeAssertions {
		headers = append(headers, "Assertions")
	}
	if includeEvalFailures {
		headers = append(headers, "Evaluator Failures")
	}
	durationHeader := ""
	if o.IncludeDurations {
		if o.IncludeTotalDuration {
			durationHeader = "Durations"
		} else {
			durationHeader = "Duration"
		}
		headers = append(headers, durationHeader)
	}

	rc := reportColumns[I, O, M]{
		opts:                o,
		includeScores:       includeScores,
		includeLabels:       includeLabels,
		includeMetrics:      includeMetrics,
		includeAssertions:   includeAssertions,
		includeEvalFailures: includeEvalFailures,
	}

	var rows [][]string
	for _, c := range r.Cases {
		rows = append(rows, rc.caseRow(c))
	}
	if o.IncludeAverages {
		if avg := r.Averages(); avg != nil {
			rows = append(rows, rc.aggregateRow(*avg))
		}
	}

	title := ""
	if !o.OmitTitle {
		title = o.Title
		if title == "" {
			title = "Evaluation Summary: " + r.Name
		}
	}

	rightAlign := map[int]bool{}
	if o.IncludeDurations {
		rightAlign[len(headers)-1] = true
	}

	return renderTable(title, headers, rows, rightAlign)
}

// Print writes the rendered report to standard output.
func (r *EvaluationReport[I, O, M]) Print(opts ...RenderOptions) {
	io.WriteString(stdout, r.Render(opts...))
	io.WriteString(stdout, "\n")
}

type reportColumns[I, O, M any] struct {
	opts                RenderOptions
	includeScores       bool
	includeLabels       bool
	includeMetrics      bool
	includeAssertions   bool
	includeEvalFailures bool
}

func (rc reportColumns[I, O, M]) caseRow(c ReportCase[I, O, M]) []string {
	row := []string{c.Name}
	if rc.opts.IncludeInput {
		row = append(row, orDash(stringFmt(c.Inputs)))
	}
	if rc.opts.IncludeMetadata {
		row = append(row, orDash(metaString(c.Metadata, c.HasMetadata)))
	}
	if rc.opts.IncludeExpectedOutput {
		row = append(row, orDash(metaString(c.ExpectedOutput, c.HasExpectedOutput)))
	}
	if rc.opts.IncludeOutput {
		row = append(row, orDash(stringFmt(c.Output)))
	}
	if rc.includeScores {
		row = append(row, renderScoreResults(c.Scores, rc.opts.IncludeReasons))
	}
	if rc.includeLabels {
		row = append(row, renderLabelResults(c.Labels, rc.opts.IncludeReasons))
	}
	if rc.includeMetrics {
		row = append(row, renderMetrics(c.Metrics))
	}
	if rc.includeAssertions {
		row = append(row, renderAssertions(c.Assertions, rc.opts.IncludeReasons))
	}
	if rc.includeEvalFailures {
		row = append(row, renderEvalFailures(c.EvaluatorFailures))
	}
	if rc.opts.IncludeDurations {
		row = append(row, rc.durations(c.TaskDuration, c.TotalDuration))
	}
	return row
}

func (rc reportColumns[I, O, M]) aggregateRow(a ReportCaseAggregate) []string {
	row := []string{a.Name}
	if rc.opts.IncludeInput {
		row = append(row, "")
	}
	if rc.opts.IncludeMetadata {
		row = append(row, "")
	}
	if rc.opts.IncludeExpectedOutput {
		row = append(row, "")
	}
	if rc.opts.IncludeOutput {
		row = append(row, "")
	}
	if rc.includeScores {
		row = append(row, renderNumberMap(a.Scores))
	}
	if rc.includeLabels {
		row = append(row, renderLabelDist(a.Labels))
	}
	if rc.includeMetrics {
		row = append(row, renderNumberMap(a.Metrics))
	}
	if rc.includeAssertions {
		row = append(row, formatPercentage(*a.Assertions)+" ✔")
	}
	if rc.includeEvalFailures {
		row = append(row, "")
	}
	if rc.opts.IncludeDurations {
		row = append(row, rc.durations(a.TaskDuration, a.TotalDuration))
	}
	return row
}

func (rc reportColumns[I, O, M]) durations(task, total time.Duration) string {
	if !rc.opts.IncludeTotalDuration {
		return formatDuration(task)
	}
	return "task: " + formatDuration(task) + "\ntotal: " + formatDuration(total)
}

func renderScoreResults(m map[string]EvaluationResult, includeReasons bool) string {
	var lines []string
	for _, k := range sortedKeys(m) {
		v := m[k]
		line := k + ": " + formatNumber(scalarToFloat(v.Value), isIntScalar(v.Value))
		if includeReasons && v.Reason != "" {
			line += "\n  Reason: " + v.Reason + "\n"
		}
		lines = append(lines, line)
	}
	return orDash(strings.Join(lines, "\n"))
}

func renderLabelResults(m map[string]EvaluationResult, includeReasons bool) string {
	var lines []string
	for _, k := range sortedKeys(m) {
		v := m[k]
		line := k + ": " + v.Value.String()
		if includeReasons && v.Reason != "" {
			line += "\n  Reason: " + v.Reason + "\n"
		}
		lines = append(lines, line)
	}
	return orDash(strings.Join(lines, "\n"))
}

func renderMetrics(m map[string]float64) string {
	var lines []string
	for _, k := range sortedKeys(m) {
		lines = append(lines, k+": "+formatNumber(m[k], isWholeNumber(m[k])))
	}
	return orDash(strings.Join(lines, "\n"))
}

// renderNumberMap renders an aggregate's numeric map. Aggregate values are always
// averages (floats), so they use float formatting even when whole.
func renderNumberMap(m map[string]float64) string {
	var lines []string
	for _, k := range sortedKeys(m) {
		lines = append(lines, k+": "+formatNumber(m[k], false))
	}
	return orDash(strings.Join(lines, "\n"))
}

func renderLabelDist(m map[string]map[string]float64) string {
	var lines []string
	for _, k := range sortedKeys(m) {
		dist := m[k]
		var parts []string
		for _, label := range sortedKeys(dist) {
			parts = append(parts, label+": "+formatPercentage(dist[label]))
		}
		lines = append(lines, k+": "+strings.Join(parts, ", "))
	}
	return orDash(strings.Join(lines, "\n"))
}

func renderAssertions(m map[string]EvaluationResult, includeReasons bool) string {
	if len(m) == 0 {
		return "-"
	}
	var b strings.Builder
	for _, k := range sortedKeys(m) {
		a := m[k]
		mark := "✗"
		if bool(a.Value.(Bool)) {
			mark = "✔"
		}
		if includeReasons {
			b.WriteString(a.Name + ": " + mark + "\n")
			if a.Reason != "" {
				b.WriteString("  Reason: " + a.Reason + "\n\n")
			}
		} else {
			b.WriteString(mark)
		}
	}
	return b.String()
}

func renderEvalFailures(failures []EvaluatorFailure) string {
	if len(failures) == 0 {
		return "-"
	}
	var lines []string
	for _, f := range failures {
		line := f.Name
		if f.ErrorMessage != "" {
			line += ": " + f.ErrorMessage
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func metaString(v any, present bool) string {
	if !present {
		return ""
	}
	return stringFmt(v)
}

func anyCaseHas[I, O, M any](cases []ReportCase[I, O, M], pred func(ReportCase[I, O, M]) bool) bool {
	for _, c := range cases {
		if pred(c) {
			return true
		}
	}
	return false
}

func isIntScalar(s Scalar) bool {
	_, ok := s.(Int)
	return ok
}

func isWholeNumber(f float64) bool {
	return f == float64(int64(f))
}

// renderTable renders a box-drawing table matching rich's Table(show_lines=True)
// with heavy header borders and light inter-row separators.
func renderTable(title string, headers []string, rows [][]string, rightAlign map[int]bool) string {
	nCols := len(headers)
	widths := make([]int, nCols)
	for i, h := range headers {
		widths[i] = displayWidth(h)
	}
	splitRows := make([][][]string, len(rows))
	for ri, row := range rows {
		cells := make([][]string, nCols)
		for ci := 0; ci < nCols; ci++ {
			cell := ""
			if ci < len(row) {
				cell = row[ci]
			}
			lines := strings.Split(cell, "\n")
			cells[ci] = lines
			for _, line := range lines {
				if w := displayWidth(line); w > widths[ci] {
					widths[ci] = w
				}
			}
		}
		splitRows[ri] = cells
	}

	var b strings.Builder
	totalWidth := nCols*3 + 1
	for _, w := range widths {
		totalWidth += w
	}

	if title != "" {
		pad := (totalWidth - displayWidth(title)) / 2
		if pad < 0 {
			pad = 0
		}
		b.WriteString(strings.Repeat(" ", pad))
		b.WriteString(title)
		b.WriteString("\n")
	}

	writeBorder(&b, widths, "┏", "┳", "┓", "━")
	writeHeaderRow(&b, widths, headers, rightAlign)
	writeBorder(&b, widths, "┡", "╇", "┩", "━")

	for ri, cells := range splitRows {
		writeMultilineRow(&b, widths, cells, rightAlign)
		if ri < len(splitRows)-1 {
			writeBorder(&b, widths, "├", "┼", "┤", "─")
		}
	}
	writeBorder(&b, widths, "└", "┴", "┘", "─")

	return strings.TrimRight(b.String(), "\n")
}

func writeBorder(b *strings.Builder, widths []int, left, mid, right, fill string) {
	b.WriteString(left)
	for i, w := range widths {
		b.WriteString(strings.Repeat(fill, w+2))
		if i < len(widths)-1 {
			b.WriteString(mid)
		}
	}
	b.WriteString(right)
	b.WriteString("\n")
}

func writeHeaderRow(b *strings.Builder, widths []int, cells []string, rightAlign map[int]bool) {
	b.WriteString("┃")
	for i, w := range widths {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		b.WriteString(" ")
		b.WriteString(pad(cell, w, rightAlign[i]))
		b.WriteString(" ")
		b.WriteString("┃")
	}
	b.WriteString("\n")
}

func writeMultilineRow(b *strings.Builder, widths []int, cells [][]string, rightAlign map[int]bool) {
	height := 1
	for _, lines := range cells {
		if len(lines) > height {
			height = len(lines)
		}
	}
	for line := 0; line < height; line++ {
		b.WriteString("│")
		for i, w := range widths {
			text := ""
			if i < len(cells) && line < len(cells[i]) {
				text = cells[i][line]
			}
			b.WriteString(" ")
			b.WriteString(pad(text, w, rightAlign[i]))
			b.WriteString(" ")
			b.WriteString("│")
		}
		b.WriteString("\n")
	}
}

func pad(s string, width int, right bool) string {
	gap := width - displayWidth(s)
	if right {
		return strings.Repeat(" ", gap) + s
	}
	return s + strings.Repeat(" ", gap)
}

// displayWidth returns the number of monospace columns a string occupies.
// Non-ASCII runes are counted as width 1 (sufficient for the box characters and
// check marks used here).
func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}
