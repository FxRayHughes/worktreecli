package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type homeModel struct {
	list simpleList
}

func newHomeModel() homeModel {
	items := []listItem{
		{Title: "创建工作树", Description: "选择基线分支 + 环境 → 创建新 worktree", Value: "create"},
		{Title: "管理工作树", Description: "列出、进入、删除现有 worktree", Value: "manage"},
		{Title: "环境管理", Description: "编辑 ~/.wtc/environments 下的脚本环境", Value: "envs"},
		{Title: "设置", Description: "worktree 根目录、会话模式、自动清理等", Value: "config"},
		{Title: "退出", Value: "quit"},
	}
	return homeModel{list: newList("请选择操作", items)}
}

func (m Model) updateHome(msg tea.Msg) (tea.Model, tea.Cmd) {
	activate := false
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			activate = true
		case "q", "esc":
			return m, tea.Quit
		}
	}
	if m.home.list.Update(msg) {
		activate = true
	}
	if !activate {
		return m, nil
	}
	sel := m.home.list.Selected()
	if sel == nil {
		return m, nil
	}
	switch sel.Value.(string) {
	case "create":
		return m.enterBranchPage()
	case "manage":
		return m.enterManagePage()
	case "envs":
		return m.enterEnvListPage()
	case "config":
		return m.enterConfigPage()
	case "quit":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) viewHome() string {
	return m.home.list.View(m.listHeight())
}

func homeButtons() []hotBtn {
	return []hotBtn{{"确认", "enter"}, {"退出", "esc"}}
}
