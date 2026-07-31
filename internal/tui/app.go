package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/FxRayHughes/worktreecli/internal/config"
	"github.com/FxRayHughes/worktreecli/internal/git"
	tea "github.com/charmbracelet/bubbletea"
)

// Options 传入 TUI 的运行时依赖
type Options struct {
	Config *config.Config
	Repo   *git.Repo
	Eval   bool // --eval 模式（stdout 只能输出 shell 片段）
}

// page 页面枚举
type page int

const (
	pageHome page = iota
	pageCreateBranch
	pageCreateEnv
	pageCreateName
	pageCreateSession
	pageCreateConfirm
	pageCreateRunning
	pageCreateDone
	pageManage
	pageEnvList
	pageConfig
)

// Model 顶层模型
type Model struct {
	cfg  *config.Config
	repo *git.Repo
	eval bool

	page page
	err  error

	width  int
	height int


	// 子模型
	home    homeModel
	branch  branchModel
	envSel  envSelectModel
	nameIn  nameInputModel
	session sessionSelectModel
	confirm confirmModel
	running runningModel
	done    doneModel
	manage  manageModel
	envList envListModel
	conf    configModel

	// 创建流程收集的状态
	pick pickState
}

type pickState struct {
	baseBranch  string
	envFile     string // 空表示无环境
	name        string
	newBranch   string // wt/<name>
	path        string // 完整 worktree 路径
	session     config.SessionMode
	spawnScript string // 由 env.OnSpawned 渲染出的 shell 脚本，spawn 模式使用
}

// Run 启动 TUI
func Run(opts Options) error {
	m := newModel(opts)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	final, err := p.Run()
	if err != nil {
		return err
	}
	if mm, ok := final.(Model); ok && mm.err != nil {
		return mm.err
	}
	return nil
}

func newModel(opts Options) Model {
	m := Model{
		cfg:  opts.Config,
		repo: opts.Repo,
		eval: opts.Eval,
		page: pageHome,
		pick: pickState{session: opts.Config.SessionMode},
	}
	m.home = newHomeModel(opts.Repo != nil)
	return m
}

// Init 实现 tea.Model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update 实现 tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.MouseMsg:
		// 拦截 hotbar 点击 → 合成对应 KeyMsg 再派发给页面
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if key := m.hitTest(msg.X, msg.Y); key != "" {
				return m.Update(synthKey(key))
			}
		}
	case errMsg:
		m.err = msg.err
	}

	switch m.page {
	case pageHome:
		return m.updateHome(msg)
	case pageCreateBranch:
		return m.updateBranch(msg)
	case pageCreateEnv:
		return m.updateEnv(msg)
	case pageCreateName:
		return m.updateName(msg)
	case pageCreateSession:
		return m.updateSession(msg)
	case pageCreateConfirm:
		return m.updateConfirm(msg)
	case pageCreateRunning:
		return m.updateRunning(msg)
	case pageCreateDone:
		return m.updateDone(msg)
	case pageManage:
		return m.updateManage(msg)
	case pageEnvList:
		return m.updateEnvList(msg)
	case pageConfig:
		return m.updateConfig(msg)
	}
	return m, nil
}

// View 实现 tea.Model
func (m Model) View() string {
	title := titleStyle.Render("wtc — Git Worktree 快捷管理")
	var subText string
	if m.repo != nil {
		subText = fmt.Sprintf("仓库: %s   分支: %s", m.repo.Name, currentBranchDisplay(m.repo))
	} else {
		subText = "未在 git 仓库内（可管理环境与设置；创建/管理工作树需要进入 git 仓库）"
	}
	sub := subtleStyle.Render(subText)
	var body string
	switch m.page {
	case pageHome:
		body = m.viewHome()
	case pageCreateBranch:
		body = m.viewBranch()
	case pageCreateEnv:
		body = m.viewEnv()
	case pageCreateName:
		body = m.viewName()
	case pageCreateSession:
		body = m.viewSession()
	case pageCreateConfirm:
		body = m.viewConfirm()
	case pageCreateRunning:
		body = m.viewRunning()
	case pageCreateDone:
		body = m.viewDone()
	case pageManage:
		body = m.viewManage()
	case pageEnvList:
		body = m.viewEnvList()
	case pageConfig:
		body = m.viewConfig()
	}
	if m.err != nil {
		body += "\n\n" + errStyle.Render("错误: "+m.err.Error())
	}
	// 顶部结构：
	//   第 0 行: 标题
	//   第 1 行: 仓库/分支
	//   第 2 行: 空行
	//   第 3 行: 按钮条（可点击）      ← hotbar 固定在这一行
	//   第 4 行: 分隔线
	//   第 5 行起: body
	btns := m.currentButtons()
	availInner := m.width - 6 // frame border 2 + padding L/R 4
	hotbar := renderHotbarWithWidth(btns, availInner)
	// 分割线：留一点余量避免换行导致后续行错位
	sepWidth := m.width - 10
	if sepWidth < 10 {
		sepWidth = 40
	}
	if sepWidth > 100 {
		sepWidth = 100
	}
	divider := subtleStyle.Render(strings.Repeat("─", sepWidth))

	// hotbar 屏幕 Y = frame padding 顶部 1 + border 顶部 1 + 标题 1 + 副标题 1 + 空行 1 = 5
	// 左侧偏移 = border 1 + padding 2 = 3
	if hotbar != "" {
		setLastHotbar(5, computeHotHits(btns, 3, availInner))
	} else {
		setLastHotbar(-1, nil)
	}

	var content string
	if hotbar != "" {
		content = title + "\n" + sub + "\n\n" + hotbar + "\n" + divider + "\n" + body
	} else {
		content = title + "\n" + sub + "\n" + divider + "\n" + body
	}

	// 全屏
	w, h := m.width, m.height
	if w <= 0 || h <= 0 {
		return content
	}
	return frameStyle.Width(w - 2).Height(h - 2).Render(content)
}

// currentButtons 返回当前页面对应的按钮组
func (m Model) currentButtons() []hotBtn {
	switch m.page {
	case pageHome:
		return homeButtons()
	case pageCreateBranch:
		return branchButtons()
	case pageCreateEnv:
		return envButtons()
	case pageCreateName:
		return nameButtons()
	case pageCreateSession:
		return sessionButtons()
	case pageCreateConfirm:
		return confirmButtons()
	case pageCreateDone:
		return doneButtons()
	case pageManage:
		return manageButtons()
	case pageEnvList:
		return envListButtons()
	case pageConfig:
		return configButtons()
	}
	return nil
}

func currentBranchDisplay(r *git.Repo) string {
	b, err := r.CurrentBranch()
	if err != nil {
		return "?"
	}
	return b
}

// hitTest 判断鼠标点击是否命中底部按钮，命中则返回等价的 key
func (m Model) hitTest(x, y int) string {
	if y != lastHotRow || len(lastHotHits) == 0 {
		return ""
	}
	for _, h := range lastHotHits {
		if x >= h.x0 && x <= h.x1 {
			return h.key
		}
	}
	return ""
}

// synthKey 把字符串键名转成 tea.KeyMsg
func synthKey(key string) tea.KeyMsg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	default:
		// 单字符 key
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

// listHeight 返回列表可用行数（根据窗口高度自适应）
// 预留头部（标题2行 + 空1行 + list title 2行 + 空1行 = 6）和底部（helper 2行 + 边框 2行 = 4）
// listHeight 返回列表最多能显示多少行
// 精确计算顶部/底部固定占用，确保 body 不会溢出把 hotbar 挤走
func (m Model) listHeight() int {
	if m.height <= 0 {
		return 12
	}
	// 顶部固定占用: frame(2) + title(1) + subtitle(1) + 空(1) + hotbar(1) + divider(1) = 7
	// 底部固定占用: frame(2) + body 内的辅助行预留(3, 状态/提示/错误行) = 5
	// list 自己顶部还有 list.title(1) + 空(1) = 2
	avail := m.height - 7 - 5 - 2
	if avail < 3 {
		return 3
	}
	return avail
}

// errMsg 通用错误消息
type errMsg struct{ err error }

func toErr(err error) tea.Cmd {
	return func() tea.Msg { return errMsg{err} }
}

// exitPage 用来在流程结束后退出到 shell
func exitProgram() tea.Cmd {
	return tea.Quit
}

// helper: stderr 打印（用于 --eval 场景，普通提示不能污染 stdout）
func printStderr(s string) {
	fmt.Fprint(os.Stderr, s)
}
