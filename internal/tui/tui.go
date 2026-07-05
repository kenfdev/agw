package tui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/kenfdev/agw/internal/base"
	"github.com/kenfdev/agw/internal/docker"
	"github.com/kenfdev/agw/internal/doctor"
	"github.com/kenfdev/agw/internal/workspace"
)

const (
	focusWorkspaces = "workspaces"
	focusBase       = "base"
)

type Model struct {
	workspaces    []workspace.LocatedDefinition
	reports       []doctor.Report
	baseStatus    *base.Status
	actions       Actions
	selected      int
	status        string
	mode          string
	focus         string
	filter        string
	viewerTitle   string
	viewerPath    string
	viewerContent string
	requestShell  bool
	width         int
	height        int
	projectCursor int
	projects      []workspace.Project
	actionRunning bool
	actionName    string
	quitting      bool
}

type Actions interface {
	Status(workspace.LocatedDefinition) (string, error)
	Start(workspace.LocatedDefinition) (string, error)
	Restart(workspace.LocatedDefinition, docker.UpOptions) (string, error)
	Build(workspace.LocatedDefinition) (string, error)
	BaseStatus() (*base.Status, error)
	BuildBase() (string, error)
	Up(workspace.LocatedDefinition) (string, error)
	Down(workspace.LocatedDefinition) (string, error)
	Attach(workspace.LocatedDefinition) (string, error)
	Prepare(workspace.LocatedDefinition) (string, error)
	Refresh(workspace.LocatedDefinition) (doctor.Report, error)
	Logs(workspace.LocatedDefinition) (string, error)
	OpenShell(workspace.LocatedDefinition) (string, error)
	OpenPath(string) (string, error)
	EditPath(string) (string, error)
	CopyText(string) (string, error)
}

type StreamingActions interface {
	BuildStream(workspace.LocatedDefinition, io.Writer) (string, error)
	BuildBaseStream(io.Writer) (string, error)
	StartStream(workspace.LocatedDefinition, io.Writer) (string, error)
	RestartStream(workspace.LocatedDefinition, docker.UpOptions, io.Writer) (string, error)
	UpStream(workspace.LocatedDefinition, io.Writer) (string, error)
	DownStream(workspace.LocatedDefinition, io.Writer) (string, error)
}

type actionLogMsg struct {
	name  string
	chunk string
	ch    <-chan tea.Msg
}

type actionDoneMsg struct {
	name        string
	result      string
	err         error
	refreshBase bool
	ch          <-chan tea.Msg
}

func NewModel(workspaces []workspace.LocatedDefinition) Model {
	return newModel(workspaces, nil, nil, nil)
}

func NewModelWithActions(workspaces []workspace.LocatedDefinition, actions Actions) Model {
	return newModel(workspaces, nil, nil, actions)
}

func NewModelWithReports(reports []doctor.Report, actions Actions) Model {
	return newModel(nil, reports, nil, actions)
}

func NewModelWithReportsAndBaseStatus(reports []doctor.Report, baseStatus *base.Status, actions Actions) Model {
	return newModel(nil, reports, baseStatus, actions)
}

func NewModelWithWorkspacesReportsAndBaseStatus(workspaces []workspace.LocatedDefinition, reports []doctor.Report, baseStatus *base.Status, actions Actions) Model {
	return newModel(workspaces, reports, baseStatus, actions)
}

func newModel(workspaces []workspace.LocatedDefinition, reports []doctor.Report, baseStatus *base.Status, actions Actions) Model {
	return Model{
		workspaces: workspaces,
		reports:    reports,
		baseStatus: baseStatus,
		actions:    actions,
		focus:      focusWorkspaces,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case actionLogMsg:
		if msg.name == m.actionName {
			m.status = appendStatus(m.status, msg.chunk)
		}
		return m, waitActionMsg(msg.ch)
	case actionDoneMsg:
		if msg.name == m.actionName {
			m.actionRunning = false
			m.actionName = ""
			if msg.err != nil {
				m.status = appendStatus(m.status, msg.name+" failed: "+msg.err.Error())
			} else {
				result := msg.result
				if result == "" {
					result = msg.name
				}
				m.status = appendStatus(m.status, msg.name+" ok: "+result)
			}
			if msg.refreshBase {
				m = m.refreshBaseStatusQuiet()
			}
		}
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyTab {
			m = m.cycleFocus()
			return m, nil
		}
		if m.mode == "filter" {
			return m.updateFilter(msg)
		}
		if m.mode == "confirm-down" {
			return m.updateConfirmDown(msg)
		}
		if m.mode == "confirm-build" {
			return m.updateConfirmBuild(msg)
		}
		if m.mode == "confirm-restart" {
			return m.updateConfirmRestart(msg)
		}
		if m.mode == "confirm-base-build" {
			return m.updateConfirmBaseBuild(msg)
		}
		if m.mode == "project-selector" {
			return m.updateProjectSelector(msg)
		}
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "esc":
			m.mode = ""
			m.viewerTitle = ""
			m.viewerPath = ""
			m.viewerContent = ""
		case "?":
			m.mode = "help"
		case ":":
			m.mode = "commands"
		case "/":
			m.mode = "filter"
		case "up", "k":
			if m.focus == focusWorkspaces {
				m.moveSelection(-1)
			}
		case "down", "j":
			if m.focus == focusWorkspaces {
				m.moveSelection(1)
			}
		case "home", "g":
			m.selectVisibleEdge(false)
		case "end", "G":
			m.selectVisibleEdge(true)
		case "enter":
			if m.focus == focusWorkspaces {
				m.mode = "details"
			}
		case "s":
			m = m.requestShellAndQuit()
		case "t":
			return m.runAction("start", func(item workspace.LocatedDefinition) (string, error) {
				return m.actions.Start(item)
			}, func(actions StreamingActions, item workspace.LocatedDefinition, out io.Writer) (string, error) {
				return actions.StartStream(item, out)
			})
		case "R":
			m = m.confirmRestart()
		case "b":
			if m.focus == focusBase {
				m = m.confirmBaseBuild()
			} else {
				m = m.confirmBuild()
			}
		case "u":
			return m.runAction("up", func(item workspace.LocatedDefinition) (string, error) {
				return m.actions.Up(item)
			}, func(actions StreamingActions, item workspace.LocatedDefinition, out io.Writer) (string, error) {
				return actions.UpStream(item, out)
			})
		case "ctrl+d":
			m = m.confirmDown()
		case "a":
			m = m.requestShellAndQuit()
		case "p":
			return m.runAction("prepare", func(item workspace.LocatedDefinition) (string, error) {
				return m.actions.Prepare(item)
			}, nil)
		case "r":
			m = m.refreshFocused()
		case "l":
			m = m.openLogs()
		case "d":
			m = m.openFile("Definition", selectedDefinitionPath)
		case "c":
			m = m.openFile("Compose", selectedComposePath)
		case "f":
			m = m.openFile("Dockerfile", selectedDockerfilePath)
		case "o":
			m = m.openSelectedPath()
		case "e":
			m = m.editSelectedPath()
		case "y":
			m = m.copySelectedText()
		case "Y":
			m = m.copyProjectPath()
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if m.mode == "help" {
		return m.fullscreenOrPlain(m.helpView(), "Help")
	}
	if m.mode == "commands" {
		return m.fullscreenOrPlain(m.commandsView(), "Commands")
	}
	if m.mode == "viewer" {
		return m.viewerView()
	}
	if m.mode == "project-selector" {
		return m.projectSelectorView()
	}
	if m.mode == "confirm-down" {
		return m.confirmDownView()
	}
	if m.mode == "confirm-build" {
		return m.confirmBuildView()
	}
	if m.mode == "confirm-restart" {
		return m.confirmRestartView()
	}
	if m.mode == "confirm-base-build" {
		return m.confirmBaseBuildView()
	}
	if m.hasWindowSize() {
		return m.fullscreenView()
	}

	lines := []string{m.topBar(), ""}
	if m.filter != "" || m.mode == "filter" {
		lines = append(lines, "Filter: "+m.filter, "")
	}
	if len(m.workspaces) == 0 {
		lines = append(lines, "WORKSPACE             STATE          SERVICE        ISSUE")
		lines = append(lines, "No workspaces")
	} else {
		visible := m.visibleIndexes()
		if len(visible) == 0 {
			lines = append(lines, "No matching workspaces")
		}
		lines = append(lines, "WORKSPACE             STATE          SERVICE        ISSUE")
		for _, i := range visible {
			item := m.workspaces[i]
			prefix := " "
			if i == m.selected {
				prefix = ">"
			}
			report := m.reportAt(i)
			lines = append(lines, fmt.Sprintf("%s %-20s %-14s %-14s %s", prefix, item.Definition.ID, report.State, item.Definition.Container.Service, firstFailingCheck(report)))
		}
	}

	if detailLines := m.detailsLines(); len(detailLines) > 0 {
		lines = append(lines, "", "Details")
		for _, line := range detailLines {
			lines = append(lines, "  "+line)
		}
	}
	lines = append(lines, "", "Logs")
	for _, line := range m.logLines() {
		lines = append(lines, "  "+line)
	}
	lines = append(lines, "", "Keys  tab focus  ↑/↓/j/k move  enter details  t start  R restart  b build focused  l logs  s shell  d describe  c compose  f Dockerfile  Y copy project path  ctrl+d stop  r refresh  / filter  ? help  q quit")
	return strings.Join(lines, "\n")
}

func (m Model) hasWindowSize() bool {
	return m.width > 20 && m.height > 8
}

func (m Model) fullscreenView() string {
	footerHeight := 3
	logHeight := 4
	bodyHeight := m.height - footerHeight
	if bodyHeight < 6 {
		bodyHeight = 6
	}
	listHeight := bodyHeight / 2
	if listHeight < 6 {
		listHeight = 6
	}
	detailsHeight := bodyHeight - listHeight - logHeight
	if detailsHeight < 4 {
		detailsHeight = 4
		listHeight = bodyHeight - detailsHeight - logHeight
	}
	if listHeight < 3 {
		listHeight = 3
	}

	lines := []string{}
	lines = append(lines, borderedBlock(m.topBar(), m.workspaceLines(), m.width, listHeight)...)
	lines = append(lines, borderedBlock("Details", m.detailsLines(), m.width, detailsHeight)...)
	lines = append(lines, borderedBlock("Logs", m.logLines(), m.width, logHeight)...)
	lines = append(lines, borderedBlock("Keys", []string{"tab focus  ↑/↓/j/k move  enter details  t start  R restart  b build focused  l logs  s shell  d describe  c compose  f Dockerfile  Y copy project path  ctrl+d stop  r refresh  / filter  ? help  q quit"}, m.width, footerHeight)...)
	return strings.Join(fitLineCount(lines, m.height, m.width), "\n")
}

func (m Model) workspaceLines() []string {
	lines := []string{}
	if m.filter != "" || m.mode == "filter" {
		lines = append(lines, "Filter: "+m.filter)
	}
	lines = append(lines, "WORKSPACE             STATE          SERVICE        ISSUE")
	if len(m.workspaces) == 0 {
		return append(lines, "No workspaces")
	}
	visible := m.visibleIndexes()
	if len(visible) == 0 {
		return append(lines, "No matching workspaces")
	}
	for _, i := range visible {
		item := m.workspaces[i]
		prefix := " "
		if i == m.selected {
			prefix = ">"
		}
		report := m.reportAt(i)
		lines = append(lines, fmt.Sprintf("%s %-20s %-14s %-14s %s", prefix, item.Definition.ID, report.State, item.Definition.Container.Service, firstFailingCheck(report)))
	}
	return lines
}

func (m Model) detailsLines() []string {
	if m.focus == focusBase {
		return m.baseDetailsLines()
	}
	report, ok := m.currentReportForDetails()
	if !ok {
		return []string{"No workspace selected"}
	}
	lines := []string{
		fmt.Sprintf("Workspace: %s", report.WorkspaceID),
		fmt.Sprintf("State:     %s", report.State),
		fmt.Sprintf("Next:      %s", emptyDefault(report.NextAction, "none")),
	}
	lines = append(lines, "", "Checks:")
	if len(report.Checks) == 0 {
		return append(lines, "  none")
	}
	for _, check := range report.Checks {
		lines = append(lines, fmt.Sprintf("  %s %-18s %s", checkStatusSymbol(check.Status), check.Name, check.Detail))
	}
	return lines
}

func (m Model) logLines() []string {
	if m.status == "" {
		return []string{"idle"}
	}
	return displayLines(m.status)
}

func appendStatus(status, chunk string) string {
	chunk = strings.TrimRight(chunk, "\n")
	if chunk == "" {
		return status
	}
	if status == "" {
		return chunk
	}
	return strings.TrimRight(status, "\n") + "\n" + chunk
}

type actionStreamWriter struct {
	name string
	ch   chan tea.Msg
}

func (w actionStreamWriter) Write(p []byte) (int, error) {
	chunk := string(append([]byte(nil), p...))
	w.ch <- actionLogMsg{name: w.name, chunk: chunk, ch: w.ch}
	return len(p), nil
}

func startActionStream(name string, refreshBase bool, run func(io.Writer) (string, error)) tea.Cmd {
	ch := make(chan tea.Msg, 64)
	go func() {
		result, err := run(actionStreamWriter{name: name, ch: ch})
		ch <- actionDoneMsg{name: name, result: result, err: err, refreshBase: refreshBase, ch: ch}
		close(ch)
	}()
	return waitActionMsg(ch)
}

func waitActionMsg(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return actionDoneMsg{}
		}
		return msg
	}
}

func displayLines(value string) []string {
	value = sanitizeTerminalContent(value)
	lines := strings.Split(value, "\n")
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func sanitizeTerminalContent(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")

	var b strings.Builder
	escapeState := ""
	for _, r := range value {
		switch escapeState {
		case "start":
			switch r {
			case '[':
				escapeState = "csi"
			case ']':
				escapeState = "osc"
			default:
				escapeState = ""
			}
			continue
		case "csi":
			if r >= 0x40 && r <= 0x7e {
				escapeState = ""
			}
			continue
		case "osc":
			if r == '\a' {
				escapeState = ""
			} else if r == '\x1b' {
				escapeState = "osc-st"
			}
			continue
		case "osc-st":
			if r == '\\' {
				escapeState = ""
			} else {
				escapeState = "osc"
			}
			continue
		}
		if r == '\x1b' {
			escapeState = "start"
			continue
		}
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func emptyDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func (m Model) topBar() string {
	count := len(m.visibleIndexes())
	parts := []string{fmt.Sprintf("AGW / Workspaces   %d/%d visible", count, len(m.workspaces))}
	if segment := m.baseSegment(); segment != "" {
		parts = append(parts, segment)
	}
	parts = append(parts, "focus:"+m.displayFocus(), "mode:"+m.displayMode())
	return strings.Join(parts, "   ")
}

func (m Model) displayMode() string {
	if m.mode == "" {
		return "list"
	}
	return m.mode
}

func (m Model) displayFocus() string {
	if m.focus == "" {
		return focusWorkspaces
	}
	return m.focus
}

func (m Model) cycleFocus() Model {
	if m.baseStatus == nil || strings.TrimSpace(m.baseStatus.Config.Image) == "" {
		m.focus = focusWorkspaces
		return m
	}
	if m.focus == focusBase {
		m.focus = focusWorkspaces
	} else {
		m.focus = focusBase
	}
	return m
}

func (m Model) baseSegment() string {
	if m.baseStatus == nil || strings.TrimSpace(m.baseStatus.Config.Image) == "" {
		return ""
	}
	values := []string{"Base:", m.baseStatus.Config.Image}
	if m.baseStatus.Status != "" {
		values = append(values, string(m.baseStatus.Status))
	}
	if m.baseStatus.Age != "" {
		values = append(values, m.baseStatus.Age)
	}
	segment := strings.Join(values, " ")
	if m.focus == focusBase {
		return "[" + segment + "]"
	}
	return segment
}

func (m Model) baseDetailsLines() []string {
	if m.baseStatus == nil {
		return []string{"No base image configured"}
	}
	lines := []string{
		fmt.Sprintf("Base image:  %s", m.baseStatus.Config.Image),
		fmt.Sprintf("Status:      %s", emptyDefault(string(m.baseStatus.Status), "unknown")),
	}
	if m.baseStatus.Age != "" {
		lines = append(lines, fmt.Sprintf("Age:         %s", m.baseStatus.Age))
	}
	if m.baseStatus.CreatedAt != nil {
		lines = append(lines, fmt.Sprintf("Created:     %s", m.baseStatus.CreatedAt.Format("2006-01-02T15:04:05Z07:00")))
	}
	lines = append(lines,
		fmt.Sprintf("Context:     %s", m.baseStatus.Config.ContextDir),
		fmt.Sprintf("Dockerfile:  %s", m.baseStatus.Config.Dockerfile),
	)
	if m.baseStatus.Error != "" {
		lines = append(lines, fmt.Sprintf("Error:       %s", m.baseStatus.Error))
	}
	if m.baseStatus.Status == base.StatusMissing {
		lines = append(lines, "Next:        press b to build")
	}
	return lines
}

func (m Model) runAction(name string, run func(workspace.LocatedDefinition) (string, error), stream func(StreamingActions, workspace.LocatedDefinition, io.Writer) (string, error)) (Model, tea.Cmd) {
	if m.actions == nil {
		m.status = name + " failed: no actions configured"
		return m, nil
	}
	if m.actionRunning {
		m.status = appendStatus(m.status, name+" failed: "+m.actionName+" already running")
		return m, nil
	}
	if len(m.workspaces) == 0 {
		m.status = name + " failed: no workspace selected"
		return m, nil
	}
	item, ok := m.selectedWorkspace()
	if !ok {
		m.status = name + " failed: no workspace selected"
		return m, nil
	}
	if stream != nil {
		if actions, ok := m.actions.(StreamingActions); ok {
			m.actionRunning = true
			m.actionName = name
			m.status = name + " running..."
			return m, startActionStream(name, false, func(out io.Writer) (string, error) {
				return stream(actions, item, out)
			})
		}
	}
	result, err := run(item)
	if err != nil {
		m.status = name + " failed: " + err.Error()
		return m, nil
	}
	if result == "" {
		result = item.Definition.ID
	}
	m.status = name + " ok: " + result
	return m, nil
}

func (m Model) refreshSelectedReport() Model {
	if m.actions == nil {
		m.status = "refresh failed: no actions configured"
		return m
	}
	if len(m.workspaces) == 0 {
		m.status = "refresh failed: workspace path unavailable"
		return m
	}
	if m.selected >= len(m.workspaces) {
		m.status = "refresh failed: no workspace selected"
		return m
	}

	item, ok := m.selectedWorkspace()
	if !ok {
		m.status = "refresh failed: no workspace selected"
		return m
	}
	report, err := m.actions.Refresh(item)
	if err != nil {
		m.status = "refresh failed: " + err.Error()
		return m
	}
	if report.WorkspaceID == "" {
		report.WorkspaceID = item.Definition.ID
	}
	if m.selected < len(m.reports) {
		m.reports[m.selected] = report
	} else {
		for len(m.reports) < m.selected {
			m.reports = append(m.reports, doctor.Report{})
		}
		m.reports = append(m.reports, report)
	}
	m.status = "refresh ok: " + report.WorkspaceID
	return m
}

func (m Model) refreshFocused() Model {
	if m.focus == focusBase {
		return m.refreshBaseStatus()
	}
	m = m.refreshSelectedReport()
	return m.refreshBaseStatusQuiet()
}

func (m Model) refreshBaseStatus() Model {
	if m.actions == nil {
		m.status = "base refresh failed: no actions configured"
		return m
	}
	status, err := m.actions.BaseStatus()
	if err != nil {
		m.status = "base refresh failed: " + err.Error()
		return m
	}
	m.baseStatus = status
	if status == nil || strings.TrimSpace(status.Config.Image) == "" {
		m.focus = focusWorkspaces
		m.status = "base refresh ok: not configured"
		return m
	}
	m.status = "base refresh ok: " + status.Config.Image
	return m
}

func (m Model) refreshBaseStatusQuiet() Model {
	if m.actions == nil {
		return m
	}
	status, err := m.actions.BaseStatus()
	if err == nil {
		m.baseStatus = status
	}
	if m.baseStatus == nil && m.focus == focusBase {
		m.focus = focusWorkspaces
	}
	return m
}

func (m Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		m.mode = ""
	case "backspace":
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
			m.ensureSelectedVisible()
		}
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	default:
		if len(msg.Runes) > 0 {
			m.filter += string(msg.Runes)
			m.ensureSelectedVisible()
		}
	}
	return m, nil
}

func (m Model) updateConfirmDown(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		m.mode = ""
		return m.runAction("down", func(item workspace.LocatedDefinition) (string, error) {
			return m.actions.Down(item)
		}, func(actions StreamingActions, item workspace.LocatedDefinition, out io.Writer) (string, error) {
			return actions.DownStream(item, out)
		})
	case "n", "esc":
		m.mode = ""
		m.status = "down canceled"
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateConfirmBuild(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		m.mode = ""
		return m.runAction("build", func(item workspace.LocatedDefinition) (string, error) {
			return m.actions.Build(item)
		}, func(actions StreamingActions, item workspace.LocatedDefinition, out io.Writer) (string, error) {
			return actions.BuildStream(item, out)
		})
	case "n", "esc":
		m.mode = ""
		m.status = "build canceled"
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateConfirmRestart(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		m.mode = ""
		return m.restartWithOptions(docker.UpOptions{})
	case "f":
		m.mode = ""
		return m.restartWithOptions(docker.UpOptions{ForceRecreate: true})
	case "b":
		m.mode = ""
		return m.restartWithOptions(docker.UpOptions{Build: true})
	case "a":
		m.mode = ""
		return m.restartWithOptions(docker.UpOptions{Build: true, ForceRecreate: true})
	case "n", "esc":
		m.mode = ""
		m.status = "restart canceled"
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateConfirmBaseBuild(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		m.mode = ""
		if m.actions == nil {
			m.status = "base build failed: no actions configured"
			return m, nil
		}
		if m.actionRunning {
			m.status = appendStatus(m.status, "base build failed: "+m.actionName+" already running")
			return m, nil
		}
		if actions, ok := m.actions.(StreamingActions); ok {
			m.actionRunning = true
			m.actionName = "base build"
			m.status = "base build running..."
			return m, startActionStream("base build", true, func(out io.Writer) (string, error) {
				return actions.BuildBaseStream(out)
			})
		}
		result, err := m.actions.BuildBase()
		if err != nil {
			m.status = "base build failed: " + err.Error()
			return m, nil
		}
		if result == "" && m.baseStatus != nil {
			result = m.baseStatus.Config.Image
		}
		m.status = "base build ok: " + result
		m = m.refreshBaseStatusQuiet()
	case "n", "esc":
		m.mode = ""
		m.status = "base build canceled"
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateProjectSelector(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = ""
		m.projects = nil
		m.status = "copy canceled"
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.projectCursor > 0 {
			m.projectCursor--
		}
	case "down", "j":
		if m.projectCursor+1 < len(m.projects) {
			m.projectCursor++
		}
	case "home", "g":
		m.projectCursor = 0
	case "end", "G":
		if len(m.projects) > 0 {
			m.projectCursor = len(m.projects) - 1
		}
	case "enter":
		if m.projectCursor < 0 || m.projectCursor >= len(m.projects) {
			m.status = "copy failed: no project selected"
			return m, nil
		}
		m = m.copyProject(m.projects[m.projectCursor])
	}
	return m, nil
}

func (m Model) confirmDown() Model {
	if _, ok := m.selectedWorkspace(); !ok {
		m.status = "down failed: no workspace selected"
		return m
	}
	m.mode = "confirm-down"
	return m
}

func (m Model) confirmBuild() Model {
	if _, ok := m.selectedWorkspace(); !ok {
		m.status = "build failed: no workspace selected"
		return m
	}
	m.mode = "confirm-build"
	return m
}

func (m Model) confirmRestart() Model {
	if _, ok := m.selectedWorkspace(); !ok {
		m.status = "restart failed: no workspace selected"
		return m
	}
	m.mode = "confirm-restart"
	return m
}

func (m Model) confirmBaseBuild() Model {
	if m.baseStatus == nil || strings.TrimSpace(m.baseStatus.Config.Image) == "" {
		m.status = "base build failed: no base image configured"
		return m
	}
	m.mode = "confirm-base-build"
	return m
}

func (m Model) confirmDownView() string {
	item, ok := m.selectedWorkspace()
	if !ok {
		base := m
		base.mode = ""
		return base.View()
	}
	base := m
	base.mode = ""
	message := []string{
		fmt.Sprintf("Stop workspace %s?", item.Definition.ID),
		"",
		"y confirm   n cancel   esc cancel",
	}
	modalWidth := 48
	if m.hasWindowSize() {
		if modalWidth > m.width-4 {
			modalWidth = m.width - 4
		}
		if modalWidth < 24 {
			modalWidth = 24
		}
		return overlayModal(base.View(), borderedBlock("Confirm", message, modalWidth, 5), m.width, m.height)
	}
	return base.View() + "\n" + strings.Join(borderedBlock("Confirm", message, modalWidth, 5), "\n")
}

func (m Model) confirmBuildView() string {
	item, ok := m.selectedWorkspace()
	if !ok {
		base := m
		base.mode = ""
		return base.View()
	}
	base := m
	base.mode = ""
	message := []string{
		fmt.Sprintf("Build workspace %s?", item.Definition.ID),
		"",
		"y confirm   n cancel   esc cancel",
	}
	modalWidth := 48
	if m.hasWindowSize() {
		if modalWidth > m.width-4 {
			modalWidth = m.width - 4
		}
		if modalWidth < 24 {
			modalWidth = 24
		}
		return overlayModal(base.View(), borderedBlock("Confirm", message, modalWidth, 5), m.width, m.height)
	}
	return base.View() + "\n" + strings.Join(borderedBlock("Confirm", message, modalWidth, 5), "\n")
}

func (m Model) confirmRestartView() string {
	item, ok := m.selectedWorkspace()
	if !ok {
		base := m
		base.mode = ""
		return base.View()
	}
	base := m
	base.mode = ""
	message := []string{
		fmt.Sprintf("Restart workspace %s?", item.Definition.ID),
		"",
		"r restart",
		"f force recreate",
		"b build",
		"a build + force recreate",
		"esc cancel",
	}
	modalWidth := 56
	if m.hasWindowSize() {
		if modalWidth > m.width-4 {
			modalWidth = m.width - 4
		}
		if modalWidth < 24 {
			modalWidth = 24
		}
		return overlayModal(base.View(), borderedBlock("Confirm", message, modalWidth, 9), m.width, m.height)
	}
	return base.View() + "\n" + strings.Join(borderedBlock("Confirm", message, modalWidth, 9), "\n")
}

func (m Model) confirmBaseBuildView() string {
	if m.baseStatus == nil || strings.TrimSpace(m.baseStatus.Config.Image) == "" {
		base := m
		base.mode = ""
		return base.View()
	}
	baseModel := m
	baseModel.mode = ""
	message := []string{
		fmt.Sprintf("Build base image %s?", m.baseStatus.Config.Image),
		"",
		"y confirm   n cancel   esc cancel",
	}
	modalWidth := 56
	if m.hasWindowSize() {
		if modalWidth > m.width-4 {
			modalWidth = m.width - 4
		}
		if modalWidth < 24 {
			modalWidth = 24
		}
		return overlayModal(baseModel.View(), borderedBlock("Confirm", message, modalWidth, 5), m.width, m.height)
	}
	return baseModel.View() + "\n" + strings.Join(borderedBlock("Confirm", message, modalWidth, 5), "\n")
}

func (m Model) restartWithOptions(opts docker.UpOptions) (Model, tea.Cmd) {
	return m.runAction("restart", func(item workspace.LocatedDefinition) (string, error) {
		return m.actions.Restart(item, opts)
	}, func(actions StreamingActions, item workspace.LocatedDefinition, out io.Writer) (string, error) {
		return actions.RestartStream(item, opts, out)
	})
}

func (m Model) requestShellAndQuit() Model {
	if _, ok := m.selectedWorkspace(); !ok {
		m.status = "shell failed: no workspace selected"
		return m
	}
	m.requestShell = true
	m.quitting = true
	return m
}

func (m Model) openLogs() Model {
	if m.actions == nil {
		m.status = "logs failed: no actions configured"
		return m
	}
	item, ok := m.selectedWorkspace()
	if !ok {
		m.status = "logs failed: no workspace selected"
		return m
	}
	logs, err := m.actions.Logs(item)
	if err != nil {
		m.status = "logs failed: " + err.Error()
		return m
	}
	m.mode = "viewer"
	m.viewerTitle = "Logs"
	m.viewerPath = item.Definition.ID
	m.viewerContent = logs
	if strings.TrimSpace(m.viewerContent) == "" {
		m.viewerContent = "(no log output)"
	}
	return m
}

func (m Model) openFile(kind string, pathFor func(workspace.LocatedDefinition) string) Model {
	item, ok := m.selectedWorkspace()
	if !ok {
		m.status = strings.ToLower(kind) + " failed: no workspace selected"
		return m
	}
	path := pathFor(item)
	content, err := os.ReadFile(path)
	if err != nil {
		content = []byte("unable to read " + path + ": " + err.Error())
	}
	m.mode = "viewer"
	m.viewerTitle = kind + " - " + filepath.Base(path)
	m.viewerPath = path
	m.viewerContent = string(content)
	return m
}

func (m Model) openSelectedPath() Model {
	path := m.currentPath()
	if path == "" {
		m.status = "open failed: no path selected"
		return m
	}
	return m.runPathAction("open", path, func(path string) (string, error) {
		return m.actions.OpenPath(path)
	})
}

func (m Model) editSelectedPath() Model {
	path := m.currentPath()
	if path == "" {
		m.status = "edit failed: no path selected"
		return m
	}
	return m.runPathAction("edit", path, func(path string) (string, error) {
		return m.actions.EditPath(path)
	})
}

func (m Model) copySelectedText() Model {
	text := m.currentPath()
	if text == "" {
		if item, ok := m.selectedWorkspace(); ok {
			text = item.Definition.ID
		}
	}
	if text == "" {
		m.status = "copy failed: nothing selected"
		return m
	}
	return m.runPathAction("copy", text, func(text string) (string, error) {
		return m.actions.CopyText(text)
	})
}

func (m Model) copyProjectPath() Model {
	item, ok := m.selectedWorkspace()
	if !ok {
		m.status = "copy failed: no workspace selected"
		return m
	}
	if len(item.Definition.Projects) == 0 {
		m.status = "copy failed: workspace has no projects"
		return m
	}
	if len(item.Definition.Projects) > 1 {
		m.mode = "project-selector"
		m.projectCursor = 0
		m.projects = append([]workspace.Project(nil), item.Definition.Projects...)
		return m
	}
	return m.copyProjectAt(0)
}

func (m Model) copyProjectAt(index int) Model {
	item, ok := m.selectedWorkspace()
	if !ok {
		m.status = "copy failed: no workspace selected"
		return m
	}
	if index < 0 || index >= len(item.Definition.Projects) {
		m.status = "copy failed: no project selected"
		return m
	}
	return m.copyProject(item.Definition.Projects[index])
}

func (m Model) copyProject(project workspace.Project) Model {
	if strings.TrimSpace(project.HostPath) == "" {
		m.status = "copy failed: project path is empty"
		return m
	}
	m = m.runPathAction("copy", project.HostPath, func(path string) (string, error) {
		return m.actions.CopyText(path)
	})
	m.mode = ""
	m.projects = nil
	return m
}

func (m Model) runPathAction(name, value string, run func(string) (string, error)) Model {
	if m.actions == nil {
		m.status = name + " failed: no actions configured"
		return m
	}
	result, err := run(value)
	if err != nil {
		m.status = name + " failed: " + err.Error()
		return m
	}
	if result == "" {
		result = value
	}
	m.status = name + " ok: " + result
	return m
}

func (m Model) selectedWorkspace() (workspace.LocatedDefinition, bool) {
	if m.selected >= 0 && m.selected < len(m.workspaces) {
		return m.workspaces[m.selected], true
	}
	return workspace.LocatedDefinition{}, false
}

func (m Model) visibleIndexes() []int {
	if strings.TrimSpace(m.filter) == "" {
		indexes := make([]int, 0, len(m.workspaces))
		for i := range m.workspaces {
			indexes = append(indexes, i)
		}
		return indexes
	}
	query := strings.ToLower(strings.TrimSpace(m.filter))
	var indexes []int
	for i, item := range m.workspaces {
		if strings.Contains(strings.ToLower(item.Definition.ID), query) ||
			strings.Contains(strings.ToLower(item.Definition.Name), query) ||
			strings.Contains(strings.ToLower(item.Definition.Workspace.Dir), query) {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func (m *Model) moveSelection(delta int) {
	visible := m.visibleIndexes()
	if len(visible) == 0 {
		return
	}
	pos := 0
	for i, index := range visible {
		if index == m.selected {
			pos = i
			break
		}
	}
	pos += delta
	if pos < 0 {
		pos = 0
	}
	if pos >= len(visible) {
		pos = len(visible) - 1
	}
	m.selected = visible[pos]
}

func (m *Model) selectVisibleEdge(last bool) {
	visible := m.visibleIndexes()
	if len(visible) == 0 {
		return
	}
	if last {
		m.selected = visible[len(visible)-1]
		return
	}
	m.selected = visible[0]
}

func (m *Model) ensureSelectedVisible() {
	visible := m.visibleIndexes()
	if len(visible) == 0 {
		return
	}
	for _, index := range visible {
		if index == m.selected {
			return
		}
	}
	m.selected = visible[0]
}

func (m Model) currentPath() string {
	if m.mode == "viewer" && m.viewerPath != "" {
		return m.viewerPath
	}
	item, ok := m.selectedWorkspace()
	if !ok {
		return ""
	}
	return filepath.Dir(item.Path)
}

func selectedDefinitionPath(item workspace.LocatedDefinition) string {
	return item.Path
}

func selectedComposePath(item workspace.LocatedDefinition) string {
	return filepath.Join(filepath.Dir(item.Path), "compose.yaml")
}

func selectedDockerfilePath(item workspace.LocatedDefinition) string {
	return filepath.Join(filepath.Dir(item.Path), "Dockerfile")
}

func (m Model) viewerView() string {
	lines := []string{
		m.viewerTitle,
		m.viewerPath,
		"",
	}
	lines = append(lines, strings.Split(m.viewerContent, "\n")...)
	lines = append(lines, "", "esc back   o open   e edit   y copy path   q quit")
	if m.hasWindowSize() {
		return strings.Join(borderedBlock("Viewer / "+m.viewerTitle, lines, m.width, m.height), "\n")
	}
	lines = append([]string{"Viewer"}, lines...)
	return strings.Join(lines, "\n")
}

func (m Model) projectSelectorView() string {
	lines := []string{
		"Project Path",
		"",
		"Select project path to copy",
		"",
	}
	if len(m.projects) == 0 {
		lines = append(lines, "No projects")
	}
	for i, project := range m.projects {
		prefix := " "
		if i == m.projectCursor {
			prefix = ">"
		}
		lines = append(lines, fmt.Sprintf("%s %-18s %s", prefix, project.Name, project.HostPath))
	}
	lines = append(lines, "", "enter copy   ↑/↓/j/k move   esc cancel")
	content := strings.Join(lines, "\n")
	return m.fullscreenOrPlain(content, "Project Path")
}

func (m Model) fullscreenOrPlain(content, title string) string {
	if !m.hasWindowSize() {
		return content
	}
	return strings.Join(borderedBlock(title, strings.Split(content, "\n"), m.width, m.height), "\n")
}

func borderedBlock(title string, content []string, width, height int) []string {
	if width < 4 {
		width = 4
	}
	if height < 2 {
		height = 2
	}
	innerWidth := width - 2
	top := "╭" + borderTitle(title, innerWidth) + "╮"
	bottom := "╰" + strings.Repeat("─", innerWidth) + "╯"
	lines := []string{top}
	bodyHeight := height - 2
	for i := 0; i < bodyHeight; i++ {
		line := ""
		if i < len(content) {
			line = content[i]
		}
		lines = append(lines, "│"+fitWidth(line, innerWidth)+"│")
	}
	lines = append(lines, bottom)
	return lines
}

func overlayModal(base string, modal []string, width, height int) string {
	lines := strings.Split(base, "\n")
	lines = fitLineCount(lines, height, width)
	if len(modal) == 0 || height == 0 {
		return strings.Join(lines, "\n")
	}
	modalWidth := runeLen(modal[0])
	startRow := (height - len(modal)) / 2
	if startRow < 0 {
		startRow = 0
	}
	startCol := (width - modalWidth) / 2
	if startCol < 0 {
		startCol = 0
	}
	for i, modalLine := range modal {
		row := startRow + i
		if row >= len(lines) {
			break
		}
		lines[row] = replaceAt(lines[row], modalLine, startCol, width)
	}
	return strings.Join(lines, "\n")
}

func replaceAt(line, insert string, col, width int) string {
	base := []rune(fitWidth(line, width))
	value := []rune(insert)
	for i, r := range value {
		pos := col + i
		if pos < 0 || pos >= len(base) {
			continue
		}
		base[pos] = r
	}
	return string(base)
}

func borderTitle(title string, width int) string {
	if title == "" {
		return strings.Repeat("─", width)
	}
	label := " " + title + " "
	if runeLen(label) > width {
		return fitWidth(label, width)
	}
	return label + strings.Repeat("─", width-runeLen(label))
}

func fitLineCount(lines []string, height, width int) []string {
	if height <= 0 {
		return lines
	}
	for len(lines) < height {
		lines = append(lines, fitWidth("", width))
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return lines
}

func fitWidth(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) > width {
		if width <= 1 {
			return string(runes[:width])
		}
		return string(runes[:width-1]) + "…"
	}
	return string(runes) + strings.Repeat(" ", width-len(runes))
}

func runeLen(value string) int {
	return len([]rune(value))
}

func (m Model) helpView() string {
	return strings.Join([]string{
		"Help",
		"",
		"l logs",
		"s shell",
		"d definition",
		"c compose",
		"f Dockerfile",
		"tab focus workspaces/base",
		"r refresh focused",
		"t start",
		"b build focused (confirm)",
		"u up",
		"ctrl+d stop/down",
		"/ filter",
		": commands",
		"o open path",
		"e edit path",
		"y copy path",
		"Y copy project path",
		"esc back",
		"q quit",
	}, "\n")
}

func (m Model) commandsView() string {
	return strings.Join([]string{
		"Commands",
		"",
		"logs",
		"shell",
		"definition",
		"compose",
		"Dockerfile",
		"focus",
		"refresh focused",
		"start",
		"build focused",
		"up",
		"ctrl+d stop/down",
		"prepare",
		"open",
		"edit",
		"copy",
		"copy-project-path",
	}, "\n")
}

func (m Model) RequestedShell() bool {
	return m.requestShell
}

func (m Model) RequestedShellWorkspace() (workspace.LocatedDefinition, bool) {
	if !m.requestShell {
		return workspace.LocatedDefinition{}, false
	}
	return m.selectedWorkspace()
}

func (m Model) selectedReport() (doctor.Report, bool) {
	if m.selected < len(m.reports) {
		return m.reports[m.selected], true
	}
	return doctor.Report{}, false
}

func (m Model) currentReportForDetails() (doctor.Report, bool) {
	if report, ok := m.selectedReport(); ok {
		return report, true
	}
	if m.mode == "details" && m.selected < len(m.workspaces) {
		return m.reportAt(m.selected), true
	}
	return doctor.Report{}, false
}

func (m Model) reportAt(index int) doctor.Report {
	if index < len(m.reports) {
		return m.reports[index]
	}
	if index < len(m.workspaces) {
		return doctor.Report{WorkspaceID: m.workspaces[index].Definition.ID}
	}
	return doctor.Report{}
}

func firstFailingCheck(report doctor.Report) string {
	for _, check := range report.Checks {
		if check.Status == doctor.CheckFail {
			return check.Name
		}
	}
	return "-"
}

func checkStatusSymbol(status doctor.CheckStatus) string {
	switch status {
	case doctor.CheckPass:
		return "✓"
	case doctor.CheckFail:
		return "✗"
	case doctor.CheckWarn:
		return "!"
	case doctor.CheckSkip:
		return "-"
	default:
		return "-"
	}
}

func Run(workspaces []workspace.LocatedDefinition) error {
	_, err := tea.NewProgram(NewModel(workspaces), tea.WithAltScreen()).Run()
	return err
}

func RunWithActions(workspaces []workspace.LocatedDefinition, actions Actions) error {
	_, err := tea.NewProgram(NewModelWithActions(workspaces, actions), tea.WithAltScreen()).Run()
	return err
}

func RunWithReports(workspaces []workspace.LocatedDefinition, reports []doctor.Report, actions Actions) error {
	_, err := RunWithReportsResult(workspaces, reports, actions)
	return err
}

func RunWithReportsResult(workspaces []workspace.LocatedDefinition, reports []doctor.Report, actions Actions) (Model, error) {
	return RunWithReportsAndBaseStatusResult(workspaces, reports, nil, actions)
}

func RunWithReportsAndBaseStatusResult(workspaces []workspace.LocatedDefinition, reports []doctor.Report, baseStatus *base.Status, actions Actions) (Model, error) {
	model, err := tea.NewProgram(NewModelWithWorkspacesReportsAndBaseStatus(workspaces, reports, baseStatus, actions), tea.WithAltScreen()).Run()
	if err != nil {
		return Model{}, err
	}
	return model.(Model), nil
}
