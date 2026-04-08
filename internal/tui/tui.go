// Package tui implements the interactive terminal dashboard for barq-witness.
// It uses bubbletea for the event loop and lipgloss for styling.
package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yasserrmd/barq-witness/internal/analyzer"
	"github.com/yasserrmd/barq-witness/internal/store"
	"github.com/yasserrmd/barq-witness/internal/util"
)

// tickMsg is the internal message type used for the auto-refresh ticker.
type tickMsg time.Time

// refreshDoneMsg carries the result of a background refresh.
type refreshDoneMsg struct {
	report *analyzer.Report
	sha    string
	err    error
}

// --- lipgloss styles ---------------------------------------------------------

var (
	styleTier1 = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	styleTier2 = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	styleTier3 = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	styleHeader = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("15")).
			Bold(true).
			Padding(0, 1)

	styleFooter = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true)

	styleBold = lipgloss.NewStyle().Bold(true)
)

// Model is the bubbletea model for the TUI dashboard.
type Model struct {
	store    *store.Store
	repoPath string
	topN     int
	report   *analyzer.Report
	headSHA  string
	cursor   int
	width    int
	height   int
	lastErr  error
	loading  bool
}

// NewModel constructs the initial Model.
func NewModel(st *store.Store, repoPath string, topN int) Model {
	return Model{
		store:    st,
		repoPath: repoPath,
		topN:     topN,
		loading:  true,
	}
}

// Init fires the first tick and an immediate refresh.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		doRefresh(m.store, m.repoPath),
		tickCmd(),
	)
}

// tickCmd returns a tea.Cmd that fires every 2 seconds.
func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// doRefresh runs the analyzer in a goroutine and returns the result as a
// refreshDoneMsg.
func doRefresh(st *store.Store, repoPath string) tea.Cmd {
	return func() tea.Msg {
		sha, err := util.HeadSHA(repoPath)
		if err != nil || sha == "" {
			return refreshDoneMsg{err: fmt.Errorf("no HEAD SHA: %w", err)}
		}
		report, err := analyzer.Analyze(st, repoPath, "", sha)
		return refreshDoneMsg{report: report, sha: sha, err: err}
	}
}

// Update handles messages and keyboard events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		return m, tea.Batch(doRefresh(m.store, m.repoPath), tickCmd())

	case refreshDoneMsg:
		m.loading = false
		m.lastErr = msg.err
		if msg.err == nil {
			m.report = msg.report
			m.headSHA = msg.sha
			// Clamp cursor.
			maxCursor := 0
			if m.report != nil {
				n := len(m.visibleSegments())
				if n > 0 {
					maxCursor = n - 1
				}
			}
			if m.cursor > maxCursor {
				m.cursor = maxCursor
			}
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			m.loading = true
			return m, doRefresh(m.store, m.repoPath)
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			segs := m.visibleSegments()
			if m.cursor < len(segs)-1 {
				m.cursor++
			}
		}
	}

	return m, nil
}

// visibleSegments returns the top-N segments from the current report.
func (m Model) visibleSegments() []analyzer.Segment {
	if m.report == nil {
		return nil
	}
	segs := m.report.Segments
	if m.topN > 0 && len(segs) > m.topN {
		segs = segs[:m.topN]
	}
	return segs
}

// View renders the current state to a string.
func (m Model) View() string {
	var sb strings.Builder

	// --- Header ---
	sha := m.headSHA
	if sha == "" {
		sha = "unknown"
	} else if len(sha) > 12 {
		sha = sha[:12]
	}
	ts := time.Now().UTC().Format("15:04:05 UTC")
	headerText := fmt.Sprintf("barq-witness | commit: %s | %s", sha, ts)
	header := styleHeader.Width(m.width).Render(headerText)
	sb.WriteString(header)
	sb.WriteString("\n")

	// --- Body ---
	bodyLines := m.height - 3 // header + footer + padding
	if bodyLines < 1 {
		bodyLines = 1
	}

	if m.loading && m.report == nil {
		sb.WriteString("\nLoading...\n")
	} else if m.lastErr != nil && m.report == nil {
		sb.WriteString(fmt.Sprintf("\nError: %v\n", m.lastErr))
	} else {
		segs := m.visibleSegments()
		if len(segs) == 0 {
			sb.WriteString("\nNo flagged segments.\n")
		} else {
			for i, seg := range segs {
				line := renderSegment(i, seg, i == m.cursor)
				sb.WriteString(line)
				sb.WriteString("\n")
			}
		}
	}

	// Pad to fill the terminal height.
	rendered := sb.String()
	lineCount := strings.Count(rendered, "\n")
	for lineCount < m.height-1 {
		sb.WriteString("\n")
		lineCount++
	}

	// --- Footer ---
	footer := styleFooter.Render("q/Ctrl+C: quit | r: refresh now | up/down: scroll")
	sb.WriteString(footer)

	return sb.String()
}

// renderSegment formats a single segment row.
func renderSegment(idx int, seg analyzer.Segment, selected bool) string {
	tierStyle, tierLabel := tierStyleAndLabel(seg.Tier)

	badge := tierStyle.Render(tierLabel)

	// File path, truncated if needed.
	path := seg.FilePath
	reasons := strings.Join(seg.ReasonCodes, ", ")
	score := fmt.Sprintf("%.0f", seg.Score)

	line := fmt.Sprintf(" %s  %-40s  score=%-4s  %s",
		badge, path, score, reasons)

	if selected {
		line = styleBold.Render(">") + line
	} else {
		line = " " + line
	}

	return line
}

// tierStyleAndLabel returns the lipgloss style and label text for a tier.
func tierStyleAndLabel(tier int) (lipgloss.Style, string) {
	switch tier {
	case 1:
		return styleTier1, "[TIER 1]"
	case 2:
		return styleTier2, "[TIER 2]"
	case 3:
		return styleTier3, "[TIER 3]"
	default:
		return lipgloss.NewStyle(), "[TIER ?]"
	}
}

// Run starts the bubbletea program and blocks until the user quits.
func Run(st *store.Store, repoPath string, topN int) error {
	m := NewModel(st, repoPath, topN)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
