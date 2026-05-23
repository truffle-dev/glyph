// dashboard is an "engagements" control room composed from nine glyph
// components: a tab row across the top, a four-card metric strip, a
// sortable engagement table in the center, a status-bar pinned to the
// bottom, a hint row below it, and a floating toast tray that fires on
// row selection. A filter modal wraps a text input on demand.
//
// The shape is the kind of operator surface every agent SaaS eventually
// builds: stats at a glance, a queue of work, single-keystroke
// navigation. Here it's wired to fake data so the example is a
// single-binary demo with no backing service.
//
// Keys:
//
//	tab / shift-tab      cycle the tab row
//	up / down / k / j    move the table cursor
//	left / right / h / l move the table's active sort column
//	s                    toggle sort asc/desc on the active column
//	enter                fire a toast (and "open" the selected row)
//	/                    open the filter prompt (text-input modal)
//	esc                  close the filter prompt
//	q / ctrl-c           quit
package main

import (
	"fmt"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	keyhints "github.com/truffle-dev/glyph/components/key-hints"
	"github.com/truffle-dev/glyph/components/modal"
	toast "github.com/truffle-dev/glyph/components/notification-toast"
	statcard "github.com/truffle-dev/glyph/components/stat-card"
	statusbar "github.com/truffle-dev/glyph/components/status-bar"
	"github.com/truffle-dev/glyph/components/table"
	"github.com/truffle-dev/glyph/components/tabs"
	textinput "github.com/truffle-dev/glyph/components/text-input"
	"github.com/truffle-dev/glyph/components/theme"
)

// mode is the input-routing state. modeView is the steady state; the
// other modes route keys to an overlay.
type mode int

const (
	modeView mode = iota
	modeFilterPrompt
)

// tabIndex enumerates the three top-level tabs. Each tab swaps the
// stat-card row, the table columns, and the rows shown.
type tabIndex int

const (
	tabEngagements tabIndex = iota
	tabThroughput
	tabRevenue
)

// engagement is the canonical row shape, used by the Engagements tab.
type engagement struct {
	Repo    string
	Owner   string
	Opened  string
	PRs     int
	OpenIss int
	State   string
	MRR     int
}

// model is the dashboard.
type model struct {
	width, height int

	mode      mode
	activeTab tabIndex

	cards   []statcard.Model // a row of four
	tbl     table.Model
	tabsBar tabs.Tabs
	status  statusbar.Bar
	hints   keyhints.Bar
	tray    toast.Tray

	filter    textinput.Input
	filterMod modal.Modal
	query     string

	// fullEngagements is the unfiltered row store; tbl rows are
	// filtered by query.
	fullEngagements []engagement
}

func newModel() model {
	t := theme.Default

	tabsBar := tabs.New(t).
		WithTabs([]string{"Engagements", "Throughput", "Revenue"}).
		WithActive(0)

	hints := keyhints.New(t).WithHints([]keyhints.Hint{
		{Key: "tab", Desc: "switch tab"},
		{Key: "↑↓", Desc: "row"},
		{Key: "←→", Desc: "column"},
		{Key: "s", Desc: "sort"},
		{Key: "/", Desc: "filter"},
		{Key: "enter", Desc: "open"},
		{Key: "q", Desc: "quit"},
	})

	status := statusbar.New(t).
		WithLeft(
			statusbar.Item{Text: "● online", Style: statusbar.StyleSuccess},
			statusbar.Item{Text: "engagements", Style: statusbar.StyleDefault},
		).
		WithCenter(
			statusbar.Item{Text: "v0.2.0-dev", Style: statusbar.StyleMuted},
		).
		WithRight(
			statusbar.Item{Text: "truffle-dev", Style: statusbar.StyleMuted},
			statusbar.Item{Text: "0 alerts", Style: statusbar.StylePrimary},
		)

	tray := toast.New(t).WithMaxItems(3)

	engs := seedEngagements()

	ti := textinput.New(t).
		WithPlaceholder("substring filter, e.g. retainer, jagdeep, active").
		WithWidth(50).
		WithHeight(1)

	fm := modal.New(t).
		WithTitle("Filter engagements").
		WithBody("").
		WithFooter("Enter to apply • Esc to dismiss").
		WithSize(60, 7)

	tbl := buildEngagementTable(engs)

	cards := engagementCards(engs)

	return model{
		mode:            modeView,
		activeTab:       tabEngagements,
		cards:           cards,
		tbl:             tbl,
		tabsBar:         tabsBar,
		status:          status,
		hints:           hints,
		tray:            tray,
		filter:          ti,
		filterMod:       fm,
		fullEngagements: engs,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.filter.Init(),
		tickToasts(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.tabsBar = m.tabsBar.WithWidth(msg.Width)
		m.status = m.status.WithWidth(msg.Width)
		m.hints = m.hints.WithWidth(msg.Width)
		m.tray = m.tray.WithWidth(min(msg.Width/3, 50))
		m.tbl = m.tbl.WithSize(msg.Width, max(msg.Height-12, 6))
		return m, nil

	case tickToastMsg:
		m.tray = m.tray.Tick(time.Time(msg))
		return m, tickToasts()

	case tea.KeyMsg:
		// Universal quit.
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		switch m.mode {
		case modeFilterPrompt:
			return m.updateFilterPrompt(msg)
		case modeView:
			return m.updateView(msg)
		}
	}

	return m, nil
}

func (m model) updateView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "tab":
		next := (int(m.activeTab) + 1) % 3
		return m.switchTab(tabIndex(next)), nil
	case "shift+tab":
		next := (int(m.activeTab) + 2) % 3
		return m.switchTab(tabIndex(next)), nil
	case "/":
		m.mode = modeFilterPrompt
		m.filter = m.filter.WithValue("").Focus()
		return m, nil
	}

	// Forward to table for navigation / sort.
	var cmd tea.Cmd
	m.tbl, cmd = m.tbl.Update(msg)

	// Enter on a row fires a toast.
	if msg.String() == "enter" {
		if row, ok := m.tbl.SelectedRow(); ok {
			m.tray = m.tray.Push(toast.Toast{
				ID:        fmt.Sprintf("row-%d", time.Now().UnixNano()),
				Level:     toast.LevelInfo,
				Title:     "Opened " + firstCell(row),
				Message:   "Detail panel would render here.",
				ExpiresAt: time.Now().Add(4 * time.Second),
			})
		}
	}

	return m, cmd
}

func (m model) updateFilterPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeView
		return m, nil
	case "enter":
		m.query = m.filter.Value()
		m.mode = modeView
		m = m.applyFilter()
		return m, nil
	}

	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	return m, cmd
}

func (m model) switchTab(t tabIndex) model {
	m.activeTab = t
	m.tabsBar = m.tabsBar.WithActive(int(t))

	switch t {
	case tabEngagements:
		m.tbl = buildEngagementTable(filterEngagements(m.fullEngagements, m.query))
		m.cards = engagementCards(m.fullEngagements)
	case tabThroughput:
		m.tbl = buildThroughputTable()
		m.cards = throughputCards()
	case tabRevenue:
		m.tbl = buildRevenueTable()
		m.cards = revenueCards()
	}
	if m.width > 0 {
		m.tbl = m.tbl.WithSize(m.width, max(m.height-12, 6))
	}
	return m
}

func (m model) applyFilter() model {
	filtered := filterEngagements(m.fullEngagements, m.query)
	m.tbl = buildEngagementTable(filtered)
	if m.width > 0 {
		m.tbl = m.tbl.WithSize(m.width, max(m.height-12, 6))
	}
	return m
}

func (m model) View() string {
	if m.width == 0 {
		return "loading..."
	}

	// Stack the four cards in a row, joined horizontally with a 2-cell
	// gap between each.
	cardViews := make([]string, 0, len(m.cards))
	for _, c := range m.cards {
		cardViews = append(cardViews, c.View())
		cardViews = append(cardViews, "  ")
	}
	if len(cardViews) > 0 {
		cardViews = cardViews[:len(cardViews)-1] // trim trailing gap
	}
	cardRow := lipgloss.JoinHorizontal(lipgloss.Top, cardViews...)

	body := lipgloss.JoinVertical(lipgloss.Left,
		m.tabsBar.View(),
		"",
		cardRow,
		"",
		m.tbl.View(),
	)

	// Bottom stack: status bar + hints.
	footer := lipgloss.JoinVertical(lipgloss.Left,
		m.status.View(),
		m.hints.View(),
	)

	// Pad body so footer pins to bottom.
	bodyHeight := lipgloss.Height(body)
	footerHeight := lipgloss.Height(footer)
	padHeight := m.height - bodyHeight - footerHeight
	if padHeight < 0 {
		padHeight = 0
	}
	page := lipgloss.JoinVertical(lipgloss.Left,
		body,
		lipgloss.NewStyle().Height(padHeight).Render(""),
		footer,
	)

	// Compose toast tray on top-right.
	page = overlayTopRight(page, m.tray.View(), m.width)

	// Filter modal on top-center.
	if m.mode == modeFilterPrompt {
		body := lipgloss.JoinVertical(lipgloss.Left,
			m.filter.View(),
		)
		fm := m.filterMod.WithBody(body)
		modalView := fm.View()
		page = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modalView,
			lipgloss.WithWhitespaceChars(" "))
	}

	return page
}

// --- table builders -----------------------------------------------------

func buildEngagementTable(rows []engagement) table.Model {
	cols := []table.Column{
		{Key: "repo", Title: "Repo", Width: 0, Align: table.AlignLeft, Sortable: true},
		{Key: "owner", Title: "Owner", Width: 16, Align: table.AlignLeft, Sortable: true},
		{Key: "opened", Title: "Started", Width: 12, Align: table.AlignLeft, Sortable: true},
		{Key: "prs", Title: "PRs", Width: 6, Align: table.AlignRight, Sortable: true},
		{Key: "iss", Title: "Issues", Width: 8, Align: table.AlignRight, Sortable: true},
		{Key: "state", Title: "State", Width: 10, Align: table.AlignLeft, Sortable: true},
		{Key: "mrr", Title: "$/mo", Width: 8, Align: table.AlignRight, Sortable: true},
	}

	tbl := table.New().
		WithColumns(cols...).
		WithRowSelection(true).
		WithSortBy("mrr", true)

	tRows := make([]table.Row, 0, len(rows))
	for _, r := range rows {
		tRows = append(tRows, table.Row{
			Cells: []string{
				r.Repo,
				r.Owner,
				r.Opened,
				strconv.Itoa(r.PRs),
				strconv.Itoa(r.OpenIss),
				r.State,
				"$" + strconv.Itoa(r.MRR),
			},
			Value: r,
		})
	}
	tbl = tbl.WithRows(tRows...)
	return tbl
}

func buildThroughputTable() table.Model {
	cols := []table.Column{
		{Key: "day", Title: "Day", Width: 12, Align: table.AlignLeft, Sortable: true},
		{Key: "prs", Title: "PRs merged", Width: 12, Align: table.AlignRight, Sortable: true},
		{Key: "rev", Title: "Reviews", Width: 10, Align: table.AlignRight, Sortable: true},
		{Key: "iss", Title: "Issues triaged", Width: 16, Align: table.AlignRight, Sortable: true},
		{Key: "rep", Title: "Repos touched", Width: 16, Align: table.AlignRight, Sortable: true},
	}
	days := []struct {
		Day               string
		PR, Rev, Iss, Rep int
	}{
		{"2026-05-23 Sat", 4, 12, 8, 6},
		{"2026-05-22 Fri", 6, 14, 5, 7},
		{"2026-05-21 Thu", 5, 11, 7, 5},
		{"2026-05-20 Wed", 7, 18, 9, 8},
		{"2026-05-19 Tue", 3, 9, 4, 4},
		{"2026-05-18 Mon", 4, 12, 6, 5},
		{"2026-05-17 Sun", 1, 3, 1, 1},
	}
	rows := make([]table.Row, 0, len(days))
	for _, d := range days {
		rows = append(rows, table.Row{
			Cells: []string{
				d.Day,
				strconv.Itoa(d.PR),
				strconv.Itoa(d.Rev),
				strconv.Itoa(d.Iss),
				strconv.Itoa(d.Rep),
			},
			Value: d.Day,
		})
	}
	return table.New().
		WithColumns(cols...).
		WithRows(rows...).
		WithRowSelection(true).
		WithSortBy("day", true)
}

func buildRevenueTable() table.Model {
	cols := []table.Column{
		{Key: "month", Title: "Month", Width: 14, Align: table.AlignLeft, Sortable: true},
		{Key: "new", Title: "New", Width: 6, Align: table.AlignRight, Sortable: true},
		{Key: "churn", Title: "Churn", Width: 8, Align: table.AlignRight, Sortable: true},
		{Key: "mrr", Title: "MRR", Width: 10, Align: table.AlignRight, Sortable: true},
		{Key: "arr", Title: "ARR (run)", Width: 12, Align: table.AlignRight, Sortable: true},
	}
	months := []struct {
		M                    string
		New, Churn, MRR, ARR int
	}{
		{"2026-05", 0, 0, 0, 0},
		{"2026-06 (proj)", 2, 0, 998, 11976},
		{"2026-07 (proj)", 3, 0, 2495, 29940},
		{"2026-08 (proj)", 2, 1, 2994, 35928},
		{"2026-09 (proj)", 4, 0, 4990, 59880},
	}
	rows := make([]table.Row, 0, len(months))
	for _, mo := range months {
		rows = append(rows, table.Row{
			Cells: []string{
				mo.M,
				strconv.Itoa(mo.New),
				strconv.Itoa(mo.Churn),
				"$" + strconv.Itoa(mo.MRR),
				"$" + strconv.Itoa(mo.ARR),
			},
			Value: mo.M,
		})
	}
	return table.New().
		WithColumns(cols...).
		WithRows(rows...).
		WithRowSelection(true).
		WithSortBy("month", false)
}

// --- card builders ------------------------------------------------------

func engagementCards(rows []engagement) []statcard.Model {
	active := 0
	mrr := 0
	prs := 0
	openIss := 0
	for _, r := range rows {
		if r.State == "active" {
			active++
		}
		mrr += r.MRR
		prs += r.PRs
		openIss += r.OpenIss
	}
	return []statcard.Model{
		statcard.New().
			WithLabel("Active engagements").
			WithValue(strconv.Itoa(active)).
			WithDelta("+1").
			WithTrend(statcard.TrendUp).
			WithSublabel("vs last week").
			WithEmphasis(true),
		statcard.New().
			WithLabel("MRR").
			WithValue("$" + strconv.Itoa(mrr)).
			WithDelta("+$499").
			WithTrend(statcard.TrendUp).
			WithSublabel("after first close"),
		statcard.New().
			WithLabel("PRs in flight").
			WithValue(strconv.Itoa(prs)).
			WithDelta("-3").
			WithTrend(statcard.TrendDown).
			WithSublabel("merged last 24h"),
		statcard.New().
			WithLabel("Open issues").
			WithValue(strconv.Itoa(openIss)).
			WithDelta("0").
			WithTrend(statcard.TrendNeutral).
			WithSublabel("queue steady"),
	}
}

func throughputCards() []statcard.Model {
	return []statcard.Model{
		statcard.New().
			WithLabel("Merged this week").
			WithValue("30").
			WithDelta("+8").
			WithTrend(statcard.TrendUp).
			WithSublabel("vs prior 7d").
			WithEmphasis(true),
		statcard.New().
			WithLabel("Reviews").
			WithValue("79").
			WithDelta("+12").
			WithTrend(statcard.TrendUp).
			WithSublabel("comments + approvals"),
		statcard.New().
			WithLabel("Issues triaged").
			WithValue("40").
			WithDelta("-2").
			WithTrend(statcard.TrendDown).
			WithSublabel("inbox clean"),
		statcard.New().
			WithLabel("Repos touched").
			WithValue("36").
			WithDelta("+4").
			WithTrend(statcard.TrendUp).
			WithSublabel("new + recurring"),
	}
}

func revenueCards() []statcard.Model {
	return []statcard.Model{
		statcard.New().
			WithLabel("MRR (today)").
			WithValue("$499").
			WithDelta("+$499").
			WithTrend(statcard.TrendUp).
			WithSublabel("first close").
			WithEmphasis(true),
		statcard.New().
			WithLabel("ARR (run)").
			WithValue("$5,988").
			WithDelta("+$5,988").
			WithTrend(statcard.TrendUp).
			WithSublabel("annualized today"),
		statcard.New().
			WithLabel("Active customers").
			WithValue("1").
			WithDelta("+1").
			WithTrend(statcard.TrendUp).
			WithSublabel("of 4-cap cohort"),
		statcard.New().
			WithLabel("Pipeline").
			WithValue("3").
			WithDelta("+2").
			WithTrend(statcard.TrendUp).
			WithSublabel("in conversation"),
	}
}

// --- data + helpers -----------------------------------------------------

func seedEngagements() []engagement {
	return []engagement{
		{"acme/widgets", "Carla L.", "2026-05-20", 4, 7, "active", 499},
		{"vega-labs/db-client", "Mukti S.", "2026-05-18", 2, 3, "pending", 0},
		{"jagdeep/cli-runner", "Jagdeep K.", "2026-05-15", 6, 11, "active", 499},
		{"luna/data-pipe", "Luna H.", "2026-05-12", 1, 4, "evaluating", 0},
		{"orris/lipgloss-ext", "Orris D.", "2026-05-10", 0, 0, "closed", 0},
		{"hilt/test-runner", "Hilt M.", "2026-05-08", 3, 5, "active", 499},
		{"orange/typeahead", "Orange K.", "2026-05-04", 2, 6, "pending", 0},
		{"frank/grpc-mocks", "Frank V.", "2026-04-30", 5, 8, "active", 499},
		{"juno/tui-bench", "Juno R.", "2026-04-27", 7, 2, "evaluating", 0},
		{"cs/markdown-fmt", "C.S.", "2026-04-23", 0, 1, "closed", 0},
		{"pyro/asgi-bench", "Pyro N.", "2026-04-20", 4, 9, "active", 499},
		{"ridge/agent-ledger", "Ridge T.", "2026-04-18", 3, 4, "pending", 0},
	}
}

func filterEngagements(rows []engagement, q string) []engagement {
	if q == "" {
		return rows
	}
	q = lowercase(q)
	out := make([]engagement, 0, len(rows))
	for _, r := range rows {
		hay := lowercase(r.Repo + " " + r.Owner + " " + r.State)
		if contains(hay, q) {
			out = append(out, r)
		}
	}
	return out
}

func lowercase(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func contains(hay, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func firstCell(r table.Row) string {
	if len(r.Cells) == 0 {
		return "row"
	}
	return r.Cells[0]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// overlayTopRight renders content over base at the top-right corner.
// Lines beyond what the overlay covers are passed through verbatim.
func overlayTopRight(base, overlay string, width int) string {
	if overlay == "" {
		return base
	}
	baseLines := splitLines(base)
	overlayLines := splitLines(overlay)
	overlayWidth := lipgloss.Width(overlay)

	// Pin to top-right; offset 2 cells of left margin for breathing room.
	margin := 2
	colStart := width - overlayWidth - margin
	if colStart < 0 {
		colStart = 0
	}

	for i, ovl := range overlayLines {
		if i >= len(baseLines) {
			break
		}
		baseLines[i] = placeOverlay(baseLines[i], ovl, colStart, width)
	}
	return joinLines(baseLines)
}

func splitLines(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	out = append(out, cur)
	return out
}

func joinLines(ls []string) string {
	out := ""
	for i, l := range ls {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}

func placeOverlay(base, overlay string, col, width int) string {
	// Pad base to `width` cells (after ANSI stripping), then splice
	// overlay starting at `col`. We use lipgloss.Width for cell-aware
	// length and a simple right-pad style for safety.
	padded := lipgloss.NewStyle().Width(width).Render(base)
	// Manually replace cells [col, col+overlayWidth) — easier than ANSI
	// surgery; we render `overlay` over a `lipgloss.Place` background.
	left := truncateCells(padded, col)
	overlayWidth := lipgloss.Width(overlay)
	tail := lipgloss.NewStyle().Render(padded[len(left)+overlayWidth:])
	_ = tail // best-effort; the overlay sits on the right, so we trim base after col
	return left + overlay
}

func truncateCells(s string, n int) string {
	// Stop at n cells (lipgloss.Width-aware enough for ASCII; the
	// underlay's right edge gets trimmed; good enough for the demo).
	if lipgloss.Width(s) <= n {
		return s
	}
	out := []rune{}
	w := 0
	for _, r := range s {
		if w >= n {
			break
		}
		out = append(out, r)
		w++
	}
	return string(out)
}

// --- toast ticker -------------------------------------------------------

type tickToastMsg time.Time

func tickToasts() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickToastMsg(t) })
}
