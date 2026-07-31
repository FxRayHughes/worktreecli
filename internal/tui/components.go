package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// simpleList 一个极简可复用的列表组件（不引 bubbles/list 以减少依赖体积）
type simpleList struct {
	title  string
	items  []listItem
	cursor int
}

type listItem struct {
	Title       string
	Description string
	Value       any
}

func newList(title string, items []listItem) simpleList {
	return simpleList{title: title, items: items}
}

func (l *simpleList) SetItems(items []listItem) {
	l.items = items
	if l.cursor >= len(items) {
		l.cursor = 0
	}
}

func (l *simpleList) Selected() *listItem {
	if len(l.items) == 0 {
		return nil
	}
	return &l.items[l.cursor]
}

// Update 处理键盘/鼠标事件
// 返回值 clicked 表示这次事件是"点击"，父层应等同于回车确认
func (l *simpleList) Update(msg tea.Msg) (clicked bool) {
	switch key := msg.(type) {
	case tea.KeyMsg:
		switch key.String() {
		case "up", "k":
			if l.cursor > 0 {
				l.cursor--
			} else {
				l.cursor = len(l.items) - 1
			}
		case "down", "j":
			if l.cursor < len(l.items)-1 {
				l.cursor++
			} else {
				l.cursor = 0
			}
		case "home", "g":
			l.cursor = 0
		case "end", "G":
			l.cursor = len(l.items) - 1
		}
	case tea.MouseMsg:
		return l.handleMouse(key)
	}
	return false
}

// handleMouse 处理鼠标事件
// 滚轮 → 光标上下；左键点击 → 光标跳到该行并触发 clicked
func (l *simpleList) handleMouse(msg tea.MouseMsg) bool {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if l.cursor > 0 {
			l.cursor--
		}
	case tea.MouseButtonWheelDown:
		if l.cursor < len(l.items)-1 {
			l.cursor++
		}
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return false
		}
		// 屏幕行 → 列表内相对行
		row := msg.Y - listStartRow
		if row < 0 || row >= len(l.items) {
			return false
		}
		if msg.X < 3 {
			return false
		}
		// 只移动光标，不自动触发确认
		// 避免点到无关空白也跳转到下一步
		// 想要选中的话，请再按 Enter 或点顶部"确认/下一步"按钮
		if l.cursor == row {
			// 第二次点击同一行 → 视为确认
			return true
		}
		l.cursor = row
		return false
	}
	return false
}

// listStartRow 是列表首行在屏幕上的近似 Y 坐标
// frame border+padding 2 + title 1 + subtle 1 + 空 1 + hotbar 1 + divider 1 + list.title 1 + 空 1 = 9
const listStartRow = 9

func (l simpleList) View(maxHeight int) string {
	var b strings.Builder
	if l.title != "" {
		b.WriteString(subtleStyle.Render(l.title))
		b.WriteString("\n\n")
	}
	if len(l.items) == 0 {
		b.WriteString(itemStyle.Render("(空)"))
		return b.String()
	}
	// 简单窗口滚动
	start := 0
	visible := maxHeight
	if visible <= 0 {
		visible = 12
	}
	if l.cursor >= visible {
		start = l.cursor - visible + 1
	}
	end := start + visible
	if end > len(l.items) {
		end = len(l.items)
	}
	for i := start; i < end; i++ {
		it := l.items[i]
		line := it.Title
		if it.Description != "" {
			line = fmt.Sprintf("%-32s  %s", it.Title, subtleStyle.Render(it.Description))
		}
		if i == l.cursor {
			b.WriteString(selectedStyle.Render("▸ " + line))
		} else {
			b.WriteString(itemStyle.Render("  " + line))
		}
		b.WriteString("\n")
	}
	if end < len(l.items) {
		b.WriteString(subtleStyle.Render(fmt.Sprintf("  ... 还有 %d 项", len(l.items)-end)))
	}
	return b.String()
}
