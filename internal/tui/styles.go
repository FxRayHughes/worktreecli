package tui

import "github.com/charmbracelet/lipgloss"

// 蓝色主题
// 主色 #3B82F6 (blue-500)  |  深调 #2563EB (blue-600)  |  亮调 #60A5FA (blue-400)
const (
	colorPrimary  = "#3B82F6"
	colorAccent   = "#60A5FA"
	colorDeep     = "#2563EB"
	colorMuted    = "#666"
	colorHelp     = "#888"
	colorText     = "#F2F2F2"
	colorBorder   = "#334155"
	colorError    = "#EF4444"
	colorSuccess  = "#22C55E"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorPrimary)).
			Padding(0, 1)

	subtleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))

	itemStyle = lipgloss.NewStyle().PaddingLeft(2)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorText)).
			Background(lipgloss.Color(colorPrimary)).
			Bold(true).
			PaddingLeft(2).
			PaddingRight(2)

	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorHelp)).Italic(true)

	errStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorError)).Bold(true)

	okStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorSuccess))

	inputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorAccent)).
			Padding(0, 1)

	panelStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorDeep)).
			Padding(1, 2).
			Margin(1, 0)

	// frameStyle 全屏外框：占满窗口，蓝色圆角边框
	frameStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorPrimary)).
			Padding(1, 2)
)
