package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"pinGo/internal/monitor"
	"pinGo/internal/notify"
)

const (
	refreshInterval = 250 * time.Millisecond
	minWidth        = 64
)

var sparkBlocks = []rune("▁▂▃▄▅▆▇█")

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	upStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	downStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	barStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238"))
)

type Model struct {
	mon     *monitor.Monitor
	snap    monitor.Snapshot
	width   int
	started time.Time
}

type tickMsg time.Time

func New(mon *monitor.Monitor) *Model {
	return &Model{mon: mon, snap: mon.Snapshot(), started: time.Now()}
}

func Run(mon *monitor.Monitor) error {
	p := tea.NewProgram(New(mon), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m *Model) Init() tea.Cmd {
	return m.tick()
}

func (m *Model) tick() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tickMsg:
		m.snap = m.mon.Snapshot()
		return m, m.tick()
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "Q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *Model) View() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n")
	b.WriteString(m.table())
	if m.snap.LastEvent != nil {
		b.WriteString("\n")
		b.WriteString(m.lastEvent())
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(" q quit · ctrl+c quit"))
	return b.String()
}

func (m *Model) header() string {
	up := 0
	for _, t := range m.snap.Targets {
		if t.Up {
			up++
		}
	}
	left := titleStyle.Render(" pingo ")
	right := dimStyle.Render(fmt.Sprintf("up %d/%d · poll %d · interval %s · running %s",
		up, len(m.snap.Targets), m.snap.Poll, m.snap.Interval, time.Since(m.started).Round(time.Second)))
	return barStyle.Width(m.clampedWidth() - 2).Render(left + " " + right)
}

func (m *Model) table() string {
	const (
		wState  = 7
		wLast   = 8
		wAvg    = 8
		wUptime = 6
		wTrend  = 14
	)
	nameW := m.clampedWidth() - wState - wLast - wAvg - wUptime - wTrend - 10
	switch {
	case nameW < 10:
		nameW = 10
	case nameW > 28:
		nameW = 28
	}
	errW := m.clampedWidth() - nameW - wState - wLast - wAvg - wUptime - wTrend - 10

	var b strings.Builder
	header := fmt.Sprintf("%-*s  %-*s  %-*s  %-*s  %-*s  %s",
		nameW, "TARGET", wState, "STATE", wLast, "LAST", wAvg, "AVG", wUptime, "UPTIME", "TREND")
	b.WriteString(dimStyle.Render(header))
	b.WriteByte('\n')

	for _, t := range m.snap.Targets {
		state := dimStyle.Render(pad("· wait", wState))
		if t.Polls > 0 {
			if t.Up {
				state = upStyle.Render(pad("● up", wState))
			} else {
				state = downStyle.Render(pad("✖ down", wState))
			}
		}
		last := "-"
		if t.Up && t.Latency > 0 {
			last = t.Latency.Round(time.Millisecond).String()
		}
		avg := "-"
		if a, ok := avgLatency(t.Latencies); ok {
			avg = a.Round(time.Millisecond).String()
		}
		uptime := "-"
		if t.Polls > 0 {
			uptime = fmt.Sprintf("%.1f%%", 100*float64(t.Ups)/float64(t.Polls))
		}

		cells := []string{
			pad(truncate(t.Name, nameW), nameW),
			state,
			pad(last, wLast),
			pad(avg, wAvg),
			pad(uptime, wUptime),
			sparkline(t.Latencies, wTrend),
		}
		row := strings.Join(cells, "  ")
		if !t.Up && t.Err != "" && errW > 3 {
			row += "  " + errStyle.Render(truncate(t.Err, errW))
		}
		b.WriteString(row)
		b.WriteByte('\n')
	}
	return b.String()
}

func (m *Model) lastEvent() string {
	e := m.snap.LastEvent
	verb := "went down"
	if e.Transition == notify.TransitionRecovered {
		verb = "recovered"
	}
	return dimStyle.Render(fmt.Sprintf(" last event: %s %s at %s — %s",
		e.Target, verb, e.At.Format("15:04:05"), e.Detail))
}

func (m *Model) clampedWidth() int {
	if m.width < minWidth {
		return minWidth
	}
	return m.width
}

func sparkline(vals []time.Duration, n int) string {
	if len(vals) == 0 {
		return dimStyle.Render(strings.Repeat("·", n))
	}
	tail := vals
	if len(tail) > n {
		tail = tail[len(tail)-n:]
	}
	maxMS := int64(0)
	for _, v := range tail {
		if ms := v.Milliseconds(); ms > maxMS {
			maxMS = ms
		}
	}
	out := make([]rune, len(tail))
	for i, v := range tail {
		idx := 0
		if maxMS > 0 {
			idx = int(v.Milliseconds()) * 7 / int(maxMS)
			if idx > 7 {
				idx = 7
			}
		}
		out[i] = sparkBlocks[idx]
	}
	return string(out)
}

func avgLatency(vals []time.Duration) (time.Duration, bool) {
	if len(vals) == 0 {
		return 0, false
	}
	var total time.Duration
	for _, v := range vals {
		total += v
	}
	return total / time.Duration(len(vals)), true
}

func pad(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

func truncate(s string, w int) string {
	if len(s) <= w {
		return s
	}
	return s[:w-1] + "…"
}
