// Command metrics-explorer is an SRE-style services dashboard composed
// from five v0.47.0 data-and-display components: table-virtualized,
// sparkline-chart, pagination-bar, timeline, and json-tree-view.
//
// The shape is the kind of operator surface every platform team
// eventually builds: a paginated list of services with an inline p99
// trend per row, plus a right panel that shows the selected service's
// recent rollout events on top and its current config below.
//
// Layout (60-column left + 38-column right; assumes ~120-col terminal):
//
//	┌ status-bar (mode, total services, page) ───────────────────────┐
//	├ table-virtualized (Name / Status / p99 / Errors)  │ timeline   │
//	│   - 47 fake services across 3 pages of 20         │            │
//	│   - p99 column renders a 14-cell sparkline-chart  │            │
//	├─────────────────────────────────────────────────  │ json-tree  │
//	├ pagination-bar (page x of y, total items)         │            │
//	├ key-hints ─────────────────────────────────────────────────────┤
//
// Keys:
//
//	up / down / k / j    move the table cursor
//	[ / ]                previous / next page
//	g / G                first / last page
//	enter                refresh the right panel from the cursor row
//	q / ctrl-c           quit
package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	jsontreeview "github.com/truffle-dev/glyph/components/json-tree-view"
	keyhints "github.com/truffle-dev/glyph/components/key-hints"
	paginationbar "github.com/truffle-dev/glyph/components/pagination-bar"
	sparklinechart "github.com/truffle-dev/glyph/components/sparkline-chart"
	statusbar "github.com/truffle-dev/glyph/components/status-bar"
	tablevirtualized "github.com/truffle-dev/glyph/components/table-virtualized"
	"github.com/truffle-dev/glyph/components/theme"
	"github.com/truffle-dev/glyph/components/timeline"
)

const (
	perPage     = 20
	totalSvcs   = 47
	sparkWidth  = 14
	leftWidth   = 60
	rightWidth  = 38
	tableHeight = 18
	rightHeight = 18
	winSeed     = 1748102400
)

type status int

const (
	statusOK status = iota
	statusWarn
	statusError
)

type service struct {
	name   string
	owner  string
	status status
	region string
	p99    []float64
	errors []float64
	events []timeline.Event
	config map[string]any
}

type model struct {
	th        theme.Theme
	services  []service
	page      int
	cursor    int
	table     tablevirtualized.Model
	bar       paginationbar.Model
	tl        timeline.Model
	jtv       jsontreeview.Model
	hints     keyhints.Bar
	statusbar statusbar.Bar
}

func newModel() model {
	r := rand.New(rand.NewSource(winSeed))
	svcs := generateServices(r, totalSvcs)
	th := theme.Default

	table := tablevirtualized.New().
		WithTheme(th).
		WithColumns(
			tablevirtualized.Column{Key: "name", Title: "Service", Width: 22},
			tablevirtualized.Column{Key: "status", Title: "Status", Width: 8},
			tablevirtualized.Column{Key: "p99", Title: "p99 (24h)", Width: sparkWidth + 8},
			tablevirtualized.Column{Key: "err", Title: "Errors", Width: 8, Align: tablevirtualized.AlignRight},
		).
		WithSize(leftWidth, tableHeight).
		WithRowSelection(true).
		WithHighlightCursor(true)

	bar := paginationbar.New().
		WithTheme(th).
		WithTotalItems(len(svcs)).
		WithPerPage(perPage).
		WithPage(0).
		WithWidth(leftWidth).
		WithPrefix("page ").
		WithItemsLabel("services")

	tl := timeline.New().
		WithTheme(th).
		WithSize(rightWidth, rightHeight/2).
		WithHighlightCursor(false)

	jtv := jsontreeview.New().
		WithTheme(th).
		WithSize(rightWidth, rightHeight/2).
		WithRootKey("config").
		WithHighlightCursor(false)

	hints := keyhints.New(th).WithHints([]keyhints.Hint{
		{Key: "↑/↓ j/k", Desc: "row"},
		{Key: "[ ]", Desc: "page"},
		{Key: "g G", Desc: "first/last"},
		{Key: "enter", Desc: "refresh detail"},
		{Key: "q", Desc: "quit"},
	})

	sb := statusbar.New(th)

	m := model{
		th:        th,
		services:  svcs,
		table:     table,
		bar:       bar,
		tl:        tl,
		jtv:       jtv,
		hints:     hints,
		statusbar: sb,
	}
	m = m.refreshTableRows()
	m = m.refreshRightPanel()
	m = m.refreshStatusBar()
	return m
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "[":
		if m.page > 0 {
			m.page--
			m.cursor = 0
			m.bar = m.bar.WithPage(m.page)
			m = m.refreshTableRows()
			m = m.refreshRightPanel()
			m = m.refreshStatusBar()
		}
		return m, nil
	case "]":
		last := m.lastPage()
		if m.page < last {
			m.page++
			m.cursor = 0
			m.bar = m.bar.WithPage(m.page)
			m = m.refreshTableRows()
			m = m.refreshRightPanel()
			m = m.refreshStatusBar()
		}
		return m, nil
	case "g":
		if m.page != 0 {
			m.page = 0
			m.cursor = 0
			m.bar = m.bar.WithPage(0)
			m = m.refreshTableRows()
			m = m.refreshRightPanel()
			m = m.refreshStatusBar()
		}
		return m, nil
	case "G":
		last := m.lastPage()
		if m.page != last {
			m.page = last
			m.cursor = 0
			m.bar = m.bar.WithPage(last)
			m = m.refreshTableRows()
			m = m.refreshRightPanel()
			m = m.refreshStatusBar()
		}
		return m, nil
	case "enter":
		m = m.refreshRightPanel()
		return m, nil
	}
	prev := m.table.Cursor()
	nt, cmd := m.table.Update(msg)
	m.table = nt
	if nt.Cursor() != prev {
		m.cursor = nt.Cursor()
		m = m.refreshRightPanel()
		m = m.refreshStatusBar()
	}
	return m, cmd
}

func (m model) View() string {
	left := lipgloss.JoinVertical(
		lipgloss.Left,
		m.table.View(),
		"",
		m.bar.View(),
	)
	right := lipgloss.JoinVertical(
		lipgloss.Left,
		titleLine("recent rollouts", m.th),
		m.tl.View(),
		"",
		titleLine("config", m.th),
		m.jtv.View(),
	)
	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(leftWidth+2).Render(left),
		lipgloss.NewStyle().Width(rightWidth).Render(right),
	)
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.statusbar.WithWidth(leftWidth+rightWidth+4).View(),
		"",
		body,
		"",
		m.hints.WithWidth(leftWidth+rightWidth+4).View(),
	)
}

func (m model) lastPage() int {
	total := len(m.services)
	if total == 0 {
		return 0
	}
	last := (total - 1) / perPage
	return last
}

func (m model) pageSlice() []service {
	start := m.page * perPage
	end := start + perPage
	if end > len(m.services) {
		end = len(m.services)
	}
	if start > len(m.services) {
		start = len(m.services)
	}
	return m.services[start:end]
}

func (m model) refreshTableRows() model {
	slice := m.pageSlice()
	rows := make([]tablevirtualized.Row, 0, len(slice))
	for i, s := range slice {
		spark := sparklinechart.New(m.th).
			WithValues(s.p99).
			WithWidth(sparkWidth).
			WithLatest(true).
			WithLatestFormat("%4.0f").
			WithLatestSuffix("ms").
			WithColor(p99Color(m.th, s)).
			View()
		rows = append(rows, tablevirtualized.Row{
			Cells: []string{
				s.name,
				statusBadge(s.status, m.th),
				spark,
				fmt.Sprintf("%d", int(s.errors[len(s.errors)-1])),
			},
			Value: m.page*perPage + i,
		})
	}
	m.table = m.table.WithRows(tablevirtualized.SliceProvider(rows))
	if m.cursor >= len(rows) {
		m.cursor = len(rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.table = m.table.WithSelectedRow(m.cursor)
	return m
}

func (m model) refreshRightPanel() model {
	slice := m.pageSlice()
	if len(slice) == 0 {
		return m
	}
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(slice) {
		idx = 0
	}
	s := slice[idx]
	m.tl = m.tl.WithEvents(s.events...).WithSelectedEvent(0)
	m.jtv = m.jtv.WithValue(s.config).WithExpandedDepth(2)
	return m
}

func (m model) refreshStatusBar() model {
	total := len(m.services)
	start := m.page*perPage + 1
	end := start + len(m.pageSlice()) - 1
	if total == 0 {
		start, end = 0, 0
	}
	cur := ""
	if slice := m.pageSlice(); len(slice) > 0 {
		idx := m.table.Cursor()
		if idx >= 0 && idx < len(slice) {
			cur = slice[idx].name
		}
	}
	m.statusbar = m.statusbar.
		WithLeft(
			statusbar.Item{Text: "metrics-explorer", Style: statusbar.StylePrimary},
			statusbar.Item{Text: fmt.Sprintf("%d services", total)},
		).
		WithCenter(
			statusbar.Item{Text: fmt.Sprintf("rows %d–%d of %d", start, end, total)},
		).
		WithRight(
			statusbar.Item{Text: cur, Style: statusbar.StylePrimary},
		)
	return m
}

func titleLine(s string, th theme.Theme) string {
	return lipgloss.NewStyle().
		Foreground(th.TextMuted).
		Render(strings.ToUpper(s))
}

func statusBadge(s status, th theme.Theme) string {
	switch s {
	case statusOK:
		return lipgloss.NewStyle().Foreground(th.Success).Render("● ok")
	case statusWarn:
		return lipgloss.NewStyle().Foreground(th.Warning).Render("● warn")
	case statusError:
		return lipgloss.NewStyle().Foreground(th.Error).Render("● error")
	}
	return ""
}

func p99Color(th theme.Theme, s service) lipgloss.Color {
	switch s.status {
	case statusError:
		return th.Error
	case statusWarn:
		return th.Warning
	}
	return th.Primary
}

// generateServices builds a deterministic fleet of fake services. The
// seed is fixed in newModel so the test suite renders the same view
// every run.
func generateServices(r *rand.Rand, n int) []service {
	names := []string{
		"api-gateway", "auth-svc", "billing-svc", "search-indexer",
		"order-router", "media-thumbnailer", "feature-flag",
		"cdn-purger", "report-builder", "notification-fanout",
		"cache-warmer", "audit-log", "rate-limiter", "session-store",
		"webhook-relay", "image-resizer", "deploy-coordinator",
		"ledger-svc", "schema-migrator", "geo-resolver",
	}
	regions := []string{"us-east-1", "us-west-2", "eu-central-1", "ap-southeast-1"}
	owners := []string{"platform", "growth", "infra", "data", "search", "payments"}

	out := make([]service, 0, n)
	for i := 0; i < n; i++ {
		base := 30 + r.Float64()*80
		jitter := 5.0 + r.Float64()*15
		var p99 []float64
		for j := 0; j < 24; j++ {
			v := base + (r.Float64()-0.5)*2*jitter
			if v < 1 {
				v = 1
			}
			p99 = append(p99, v)
		}
		var errs []float64
		for j := 0; j < 24; j++ {
			errs = append(errs, float64(r.Intn(6)))
		}
		st := statusOK
		latest := p99[len(p99)-1]
		switch {
		case latest > 110:
			st = statusError
		case latest > 90:
			st = statusWarn
		}

		name := names[i%len(names)]
		if i >= len(names) {
			name = fmt.Sprintf("%s-%d", name, (i/len(names))+1)
		}

		events := generateEvents(r, st)
		config := generateConfig(r, name, regions[r.Intn(len(regions))])

		out = append(out, service{
			name:   name,
			owner:  owners[r.Intn(len(owners))],
			region: regions[r.Intn(len(regions))],
			status: st,
			p99:    p99,
			errors: errs,
			events: events,
			config: config,
		})
	}
	return out
}

func generateEvents(r *rand.Rand, st status) []timeline.Event {
	out := []timeline.Event{
		{
			Time:   "now",
			Title:  liveTitle(st),
			Body:   liveBody(st),
			Status: liveStatus(st),
		},
	}
	now := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)
	patterns := []struct {
		ago    time.Duration
		title  string
		body   string
		status timeline.Status
	}{
		{45 * time.Minute, "deploy v2026.06.05", "promoted to 100% (image sha 9af0c7)", timeline.StatusSuccess},
		{3 * time.Hour, "canary 25%", "p99 +6ms, accepted", timeline.StatusInfo},
		{6*time.Hour + 12*time.Minute, "rollback v2026.06.04", "alert: error rate above 0.5%", timeline.StatusError},
		{18 * time.Hour, "deploy v2026.06.03", "promoted to 100%", timeline.StatusSuccess},
	}
	for _, p := range patterns {
		evtTime := now.Add(-p.ago)
		out = append(out, timeline.Event{
			Time:   evtTime.Format("15:04"),
			Title:  p.title,
			Body:   p.body,
			Status: p.status,
		})
	}
	if r.Intn(3) == 0 {
		out = append(out, timeline.Event{
			Time:   "yesterday",
			Title:  "owner ack",
			Body:   "on-call confirmed runbook applied",
			Status: timeline.StatusNeutral,
		})
	}
	return out
}

func liveTitle(st status) string {
	switch st {
	case statusError:
		return "alert firing"
	case statusWarn:
		return "p99 elevated"
	}
	return "steady-state"
}

func liveBody(st status) string {
	switch st {
	case statusError:
		return "error rate above SLO; runbook /handbook/incident-001"
	case statusWarn:
		return "p99 within 15% of SLO; watching"
	}
	return "no anomalies in last hour"
}

func liveStatus(st status) timeline.Status {
	switch st {
	case statusError:
		return timeline.StatusError
	case statusWarn:
		return timeline.StatusWarning
	}
	return timeline.StatusSuccess
}

func generateConfig(r *rand.Rand, name, region string) map[string]any {
	return map[string]any{
		"service":  name,
		"region":   region,
		"replicas": float64(2 + r.Intn(7)),
		"limits": map[string]any{
			"cpu":    fmt.Sprintf("%dm", 100*(1+r.Intn(8))),
			"memory": fmt.Sprintf("%dMi", 128*(1+r.Intn(8))),
		},
		"rollout": map[string]any{
			"strategy":    "canary",
			"steps":       []any{float64(10), float64(25), float64(50), float64(100)},
			"autoPromote": r.Intn(2) == 1,
		},
		"slo": map[string]any{
			"p99_ms":       float64(75 + r.Intn(40)),
			"availability": "99.9%",
			"error_budget": "0.1%",
		},
	}
}
