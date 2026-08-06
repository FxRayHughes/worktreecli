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
	list           simpleList
	log            string
	loaded         bool
	pendingRemoves map[string]bool // 正在后台删除中的 worktree 路径集合
}

type worktreesLoadedMsg struct {
	trees []git.Worktree
	err   error
}

func (m Model) enterManagePage() (tea.Model, tea.Cmd) {
	m.page = pageManage
	m.manage = manageModel{
		list:           newList("已有 worktree（d 删除, Enter 进入, Esc 返回）", nil),
		pendingRemoves: map[string]bool{},
	}
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
	err  error
	name string
	path string
}

// removeCurrent 触发删除
// 立即从 UI 移除并把路径记入 pendingRemoves，后台异步做 git 操作
// 完成后 removeDoneMsg 会把路径从 pendingRemoves 移除
// 期间任何 worktreesLoadedMsg 都会过滤掉正在删除中的项，避免磁盘 IO 竞态导致"复活"
func (m Model) removeCurrent() (Model, tea.Cmd) {
	sel := m.manage.list.Selected()
	if sel == nil {
		return m, nil
	}
	wt := sel.Value.(git.Worktree)
	// 加入 pending 集合
	if m.manage.pendingRemoves == nil {
		m.manage.pendingRemoves = map[string]bool{}
	}
	m.manage.pendingRemoves[wt.Path] = true

	// 立即从列表里移除该项
	newItems := make([]listItem, 0, len(m.manage.list.items))
	for _, it := range m.manage.list.items {
		if w, ok := it.Value.(git.Worktree); ok && w.Path == wt.Path {
			continue
		}
		newItems = append(newItems, it)
	}
	m.manage.list.SetItems(newItems)
	pendingCount := len(m.manage.pendingRemoves)
	m.manage.log = subtleStyle.Render(fmt.Sprintf("后台删除中: %d 个 worktree", pendingCount)) + "\n"

	repo := m.repo
	return m, func() tea.Msg {
		err := repo.RemoveWorktree(wt.Path)
		if err == nil {
			_ = os.RemoveAll(wt.Path)
			if wt.Branch != "" {
				_ = repo.DeleteBranch(wt.Branch)
			}
		}
		return removeDoneMsg{err: err, name: wt.Name, path: wt.Path}
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
			// 过滤掉正在后台删除中的 worktree，避免磁盘还没删完时刷新出现"复活"
			if m.manage.pendingRemoves[t.Path] {
				continue
			}
			items = append(items, listItem{
				Title:       t.Name,
				Description: fmt.Sprintf("%s  [%s]", t.Path, t.Branch),
				Value:       t,
			})
		}
		m.manage.list.SetItems(items)
		return m, nil
	case removeDoneMsg:
		// 从 pending 集合里移除完成的项
		if m.manage.pendingRemoves != nil {
			delete(m.manage.pendingRemoves, msg.path)
		}
		if msg.err != nil {
			m.err = msg.err
		}
		// 全部并发删除完成后才刷新列表，避免中间态导致"复活"
		if len(m.manage.pendingRemoves) == 0 {
			m.manage.log = okStyle.Render("✓ 全部后台删除已完成") + "\n"
			return m, m.loadWorktrees()
		}
		m.manage.log = subtleStyle.Render(fmt.Sprintf("后台删除中: 剩余 %d 个", len(m.manage.pendingRemoves))) + "\n"
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.page = pageHome
			return m, nil
		case "d":
			nm, cmd := m.removeCurrent()
			return nm, cmd
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
