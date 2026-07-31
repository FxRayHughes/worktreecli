package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// hotBtn 底部一个可点击按钮
type hotBtn struct {
	Label string // 显示文字
	Key   string // 触发的键名（等价 KeyMsg.String()）
}

var hotBtnStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFFFFF")).
	Background(lipgloss.Color(colorDeep)).
	Bold(true).
	Padding(0, 2).
	MarginRight(1)

var hotBtnEscStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFFFFF")).
	Background(lipgloss.Color("#B91C1C")).
	Bold(true).
	Padding(0, 2).
	MarginRight(1)

var hotBtnPrimaryStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFFFFF")).
	Background(lipgloss.Color(colorPrimary)).
	Bold(true).
	Padding(0, 2).
	MarginRight(1)

// renderHotbarStr 渲染一行按钮，纯字符串输出
// availWidth: 可用宽度（<=0 表示不限）；总宽超出时按钮省略 [key] 后缀，仍超出则截断
func renderHotbarStr(btns []hotBtn) string {
	return renderHotbarWithWidth(btns, 0)
}

func renderHotbarWithWidth(btns []hotBtn, availWidth int) string {
	// 先按完整样式渲染，累计宽度
	renderOne := func(b hotBtn, compact bool) string {
		lbl := b.Label
		if !compact {
			lbl = fmt.Sprintf("%s %s", b.Label, subtleFragment("["+b.Key+"]"))
		}
		switch b.Key {
		case "esc", "q":
			return hotBtnEscStyle.Render(lbl)
		case "enter":
			return hotBtnPrimaryStyle.Render(lbl)
		default:
			return hotBtnStyle.Render(lbl)
		}
	}
	// 先试完整版
	var parts []string
	total := 0
	for _, b := range btns {
		s := renderOne(b, false)
		parts = append(parts, s)
		total += lipgloss.Width(s) + 1
	}
	if availWidth <= 0 || total <= availWidth {
		return strings.Join(parts, "")
	}
	// 回退到紧凑版（去掉 [key]）
	parts = parts[:0]
	for _, b := range btns {
		parts = append(parts, renderOne(b, true))
	}
	return strings.Join(parts, "")
}

// computeHotHits 根据按钮组算出每个按钮的 x 命中范围
// availWidth: 可用宽度（<=0 表示不限，用完整版本渲染）
func computeHotHits(btns []hotBtn, offsetX int, availWidth int) []hotHit {
	renderOne := func(b hotBtn, compact bool) string {
		lbl := b.Label
		if !compact {
			lbl = fmt.Sprintf("%s %s", b.Label, subtleFragment("["+b.Key+"]"))
		}
		switch b.Key {
		case "esc", "q":
			return hotBtnEscStyle.Render(lbl)
		case "enter":
			return hotBtnPrimaryStyle.Render(lbl)
		default:
			return hotBtnStyle.Render(lbl)
		}
	}
	// 先估总宽度，决定是否紧凑
	total := 0
	for _, b := range btns {
		total += lipgloss.Width(renderOne(b, false)) + 1
	}
	compact := availWidth > 0 && total > availWidth

	hits := make([]hotHit, 0, len(btns))
	x := offsetX
	for _, b := range btns {
		w := lipgloss.Width(renderOne(b, compact))
		hits = append(hits, hotHit{x0: x, x1: x + w - 1, key: b.Key})
		x += w + 1
	}
	return hits
}

// subtleFragment 按键小字：改成半透明白色，保证在深色/彩色按钮底上都清晰可辨
func subtleFragment(s string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E2E8F0")).
		Faint(true).
		Render(s)
}

type hotHit struct {
	x0, x1 int
	key    string
}

// 每次 View() 时保存的最近一次 hotbar 布局，供下一个 Update 用来命中点击
// 单线程 tea 事件循环，简单包变量足够
var (
	lastHotHits []hotHit
	lastHotRow  int
)

// setLastHotbar 记录最近一次 hotbar 的 y 位置和命中区
func setLastHotbar(row int, hits []hotHit) {
	lastHotRow = row
	lastHotHits = hits
}
