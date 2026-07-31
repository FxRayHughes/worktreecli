package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/FxRayHughes/worktreecli/internal/env"
	"github.com/FxRayHughes/worktreecli/internal/git"
	"github.com/FxRayHughes/worktreecli/internal/session"
	tea "github.com/charmbracelet/bubbletea"
)

type manageModel struct {
	list   simpleList
	log    string
	loaded bool
}

type worktreesLoadedMsg struct {
	trees []git.Worktree
	err   error
}

func (m Model) enterManagePage() (tea.Model, tea.Cmd) {
	m.page = pageManage
	m.manage = manageModel{list: newList("已有 worktree（d 删除, Enter 进入, Esc 返回）", nil)}
	return m, m.loadWorktrees()
}

func (m Model) loadWorktrees() tea.Cmd {
	repo := m.repo
	return func() tea.Msg {
		trees, err := repo.ListWorktrees()
		return worktreesLoadedMsg{trees: trees, err: err}
	}
}

type removeDoneMsg struct {
	err error
}

func (m Model) removeCurrent() tea.Cmd {
	sel := m.manage.list.Selected()
	if sel == nil {
		return nil
	}
	wt := sel.Value.(git.Worktree)
	repo := m.repo
	return func() tea.Msg {
		// 若匹配到同名环境 cleanup，则先执行（尝试从 wt/<name> 反推环境比较复杂，此处跳过）
		if err := repo.RemoveWorktree(wt.Path); err != nil {
			return removeDoneMsg{err: err}
		}
		// 可选：删除目录残留
		_ = os.RemoveAll(wt.Path)
		return removeDoneMsg{err: nil}
	}
}

func (m Model) updateManage(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case worktreesLoadedMsg:
		m.manage.loaded = true
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		items := make([]listItem, 0, len(msg.trees))
		for _, t := range msg.trees {
			items = append(items, listItem{
				Title:       t.Name,
				Description: fmt.Sprintf("%s  [%s]", t.Path, t.Branch),
				Value:       t,
			})
		}
		m.manage.list.SetItems(items)
		return m, nil
	case removeDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.manage.log = okStyle.Render("✓ 已删除") + "\n"
		return m, m.loadWorktrees()
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.page = pageHome
			return m, nil
		case "d":
			return m, m.removeCurrent()
		case "enter":
			return m.launchSelectedManageItem()
		case "r":
			return m, m.loadWorktrees()
		}
	}
	if m.manage.list.Update(msg) {
		return m.launchSelectedManageItem()
	}
	return m, nil
}

func (m Model) launchSelectedManageItem() (tea.Model, tea.Cmd) {
	sel := m.manage.list.Selected()
	if sel == nil {
		return m, nil
	}
	wt := sel.Value.(git.Worktree)
	launcher := session.New(m.cfg.SessionMode)
	var evalW *evalStdoutWriter
	if m.eval {
		evalW = &evalStdoutWriter{}
	}
	var buf strings.Builder
	_ = launcher.Launch(wt.Path, &buf, evalW, "")
	m.manage.log = buf.String()
	return m, nil
}

func (m Model) viewManage() string {
	body := m.manage.list.View(m.listHeight())
	log := ""
	if m.manage.log != "" {
		log = "\n" + panelStyle.Render(m.manage.log)
	}
	return body + log + "\n\n"
}

// ─── 环境列表页 ─────────────────────────

type envListModel struct {
	list simpleList
	log  string
}

func (m Model) enterEnvListPage() (tea.Model, tea.Cmd) {
	m.page = pageEnvList
	envs, err := env.List()
	if err != nil {
		m.err = err
	}
	var items []listItem
	for _, e := range envs {
		items = append(items, listItem{
			Title:       e.Name,
			Description: e.FileName,
			Value:       e.FileName,
		})
	}
	m.envList = envListModel{list: newList("环境（Enter 用编辑器打开, n 新建, Esc 返回）", items)}
	return m, nil
}

func (m Model) updateEnvList(msg tea.Msg) (tea.Model, tea.Cmd) {
	activate := false
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "q":
			m.page = pageHome
			return m, nil
		case "enter":
			activate = true
		case "n":
			p, err := createNewEnvFile()
			if err != nil {
				m.err = err
				return m, nil
			}
			m.envList.log = okStyle.Render("✓ 已创建: "+p) + "\n"
			return m.enterEnvListPage()
		}
	}
	if m.envList.list.Update(msg) {
		activate = true
	}
	if !activate {
		return m, nil
	}
	sel := m.envList.list.Selected()
	if sel == nil {
		return m, nil
	}
	path := envPath(sel.Value.(string))
	if err := openInEditor(path); err != nil {
		m.envList.log = errStyle.Render("打开编辑器失败: "+err.Error()) + "\n"
		m.envList.log += subtleStyle.Render("文件路径: "+path) + "\n"
	} else {
		m.envList.log = okStyle.Render("✓ 已用系统关联程序打开") + "\n"
		m.envList.log += subtleStyle.Render(path) + "\n"
	}
	return m, nil
}

func (m Model) viewEnvList() string {
	body := m.envList.list.View(m.listHeight())
	log := ""
	if m.envList.log != "" {
		log = "\n" + panelStyle.Render(m.envList.log)
	}
	return body + log + "\n\n"
}

// ─── 配置页 ─────────────────────────

type configModel struct {
	log string
}

func (m Model) enterConfigPage() (tea.Model, tea.Cmd) {
	m.page = pageConfig
	m.conf = configModel{}
	return m, nil
}

func (m Model) updateConfig(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "q":
			m.page = pageHome
			return m, nil
		case "s":
			// 切换会话模式
			modes := session.Modes()
			cur := 0
			for i, mode := range modes {
				if mode == m.cfg.SessionMode {
					cur = i
					break
				}
			}
			m.cfg.SessionMode = modes[(cur+1)%len(modes)]
			m.pick.session = m.cfg.SessionMode
			return m, m.saveConfig()
		case "a":
			m.cfg.AutoRemove = !m.cfg.AutoRemove
			return m, m.saveConfig()
		}
	}
	if msg, ok := msg.(configSavedMsg); ok {
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.conf.log = okStyle.Render("✓ 已保存配置") + "\n"
		}
	}
	return m, nil
}

type configSavedMsg struct{ err error }

func (m Model) saveConfig() tea.Cmd {
	cfg := m.cfg
	return func() tea.Msg {
		return configSavedMsg{err: saveCfg(cfg)}
	}
}

func (m Model) viewConfig() string {
	cfgPath, _ := cfgFilePath()
	envDir, _ := envDirPath()
	lines := []string{
		"配置文件:     " + cfgPath,
		"环境目录:     " + envDir,
		"worktree 根: " + m.cfg.WorktreeRoot,
		fmt.Sprintf("自动删除:     %v", m.cfg.AutoRemove),
		fmt.Sprintf("保留数量:     %d", m.cfg.Retention),
		"会话模式:     " + session.ModeLabel(m.cfg.SessionMode),
	}
	body := panelStyle.Render(strings.Join(lines, "\n"))
	if m.conf.log != "" {
		body += "\n" + m.conf.log
	}
	return body + "\n\n" + "\n" +
		subtleStyle.Render("详细字段请直接编辑 config.yml") + "\n" +
		subtleStyle.Render(fmt.Sprintf("(context 未使用: %v)", context.Background() != nil))
}

// ─── 底层辅助 ─────────────────────────
