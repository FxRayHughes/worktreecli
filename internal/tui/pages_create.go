package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/FxRayHughes/worktreecli/internal/config"
	"github.com/FxRayHughes/worktreecli/internal/env"
	"github.com/FxRayHughes/worktreecli/internal/git"
	"github.com/FxRayHughes/worktreecli/internal/session"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

//
// ─── 基线分支选择 ───────────────────────────────────────
//

type branchModel struct {
	list       simpleList
	fetching   bool
	fetchedMsg string
}

func (m Model) enterBranchPage() (tea.Model, tea.Cmd) {
	m.page = pageCreateBranch
	m.branch = branchModel{list: newList("选择基线分支（f 拉取远程）", nil)}
	return m, m.loadBranches()
}

type branchesLoadedMsg struct {
	branches []git.Branch
	err      error
}

func (m Model) loadBranches() tea.Cmd {
	return func() tea.Msg {
		bs, err := m.repo.ListBranches()
		return branchesLoadedMsg{branches: bs, err: err}
	}
}

type fetchDoneMsg struct{ err error }

func (m Model) fetchRemote() tea.Cmd {
	return func() tea.Msg {
		return fetchDoneMsg{err: m.repo.Fetch()}
	}
}

func (m Model) updateBranch(msg tea.Msg) (tea.Model, tea.Cmd) {
	activate := false
	switch msg := msg.(type) {
	case branchesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		items := make([]listItem, 0, len(msg.branches))
		for _, b := range msg.branches {
			tag := "本地"
			if b.Remote {
				tag = "远程"
			}
			items = append(items, listItem{Title: b.Name, Description: tag, Value: b})
		}
		m.branch.list.SetItems(items)
		return m, nil
	case fetchDoneMsg:
		m.branch.fetching = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.branch.fetchedMsg = "已从远程拉取分支"
		return m, m.loadBranches()
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.page = pageHome
			return m, nil
		case "f":
			if !m.branch.fetching {
				sel := m.branch.list.Selected()
				if sel == nil {
					return m, nil
				}
				b := sel.Value.(git.Branch)
				m.branch.fetching = true
				m.branch.fetchedMsg = ""
				return m, m.fetchSelected(b.Name)
			}
		case "c":
			// 切换 wtc 所在目录的分支到目标分支并拉取
			if !m.branch.fetching {
				sel := m.branch.list.Selected()
				if sel == nil {
					return m, nil
				}
				b := sel.Value.(git.Branch)
				m.branch.fetching = true
				m.branch.fetchedMsg = ""
				return m, m.checkoutAndPull(b.Name)
			}
		case "enter":
			activate = true
		}
	}
	if m.branch.list.Update(msg) {
		activate = true
	}
	if !activate {
		return m, nil
	}
	sel := m.branch.list.Selected()
	if sel == nil {
		return m, nil
	}
	b := sel.Value.(git.Branch)
	m.pick.baseBranch = b.Name
	return m.enterEnvPage()
}

// fetchSelected 拉取指定分支的最新代码
// 分支被占用时会自动切到那个 worktree 里 pull
func (m Model) fetchSelected(branch string) tea.Cmd {
	repo := m.repo
	return func() tea.Msg {
		return fetchDoneMsg{err: repo.FetchBranch(branch)}
	}
}

// checkoutAndPull 把 wtc 所在目录切到 branch 并拉最新
func (m Model) checkoutAndPull(branch string) tea.Cmd {
	repo := m.repo
	return func() tea.Msg {
		return fetchDoneMsg{err: repo.CheckoutAndPull(branch)}
	}
}

func branchButtons() []hotBtn {
	return []hotBtn{
		{"下一步", "enter"},
		{"拉取分支", "f"},
		{"切换并拉取", "c"},
		{"返回", "esc"},
	}
}

func (m Model) viewBranch() string {
	body := m.branch.list.View(m.listHeight())
	status := ""
	if m.branch.fetching {
		status = subtleStyle.Render("正在拉取远程分支...")
	} else if m.branch.fetchedMsg != "" {
		status = okStyle.Render(m.branch.fetchedMsg)
	}
	return body + "\n\n" + status
}

//
// ─── 环境选择 ────────────────────────────────────────
//

type envSelectModel struct {
	list simpleList
}

func (m Model) enterEnvPage() (tea.Model, tea.Cmd) {
	m.page = pageCreateEnv
	envs, err := env.List()
	if err != nil {
		m.err = err
	}
	items := []listItem{
		{Title: "(不使用环境)", Description: "只创建 worktree，不运行脚本", Value: ""},
	}
	for _, e := range envs {
		items = append(items, listItem{
			Title:       e.Name,
			Description: e.FileName + "  " + e.Description,
			Value:       e.FileName,
		})
	}
	m.envSel = envSelectModel{list: newList("选择环境（在新 worktree 创建时执行的脚本）", items)}
	return m, nil
}

func (m Model) updateEnv(msg tea.Msg) (tea.Model, tea.Cmd) {
	activate := false
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m.enterBranchPage()
		case "enter":
			activate = true
		}
	}
	if m.envSel.list.Update(msg) {
		activate = true
	}
	if !activate {
		return m, nil
	}
	sel := m.envSel.list.Selected()
	if sel == nil {
		return m, nil
	}
	m.pick.envFile = sel.Value.(string)
	return m.enterNamePage()
}

func (m Model) viewEnv() string {
	return m.envSel.list.View(m.listHeight())
}

//
// ─── worktree 命名 ─────────────────────────────────
//

type nameInputModel struct {
	ti textinput.Model
}

func (m Model) enterNamePage() (tea.Model, tea.Cmd) {
	m.page = pageCreateName
	ti := textinput.New()
	ti.Placeholder = "worktree 名"
	ti.SetValue(defaultName(m.repo.Name))
	ti.CharLimit = 80
	ti.Width = 40
	ti.Focus()
	m.nameIn = nameInputModel{ti: ti}
	return m, textinput.Blink
}

func defaultName(repoName string) string {
	return repoName + "-" + git.ShortHash()
}

func (m Model) updateName(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m.enterEnvPage()
		case "enter":
			name := strings.TrimSpace(m.nameIn.ti.Value())
			if name == "" {
				m.err = fmt.Errorf("名称不能为空")
				return m, nil
			}
			m.err = nil
			m.pick.name = name
			m.pick.newBranch = "wt/" + name
			m.pick.path = filepath.Join(m.cfg.WorktreeRoot, name)
			return m.enterSessionPage()
		}
	}
	var cmd tea.Cmd
	m.nameIn.ti, cmd = m.nameIn.ti.Update(msg)
	return m, cmd
}

func (m Model) viewName() string {
	tip := fmt.Sprintf("默认: %s   新分支将为: wt/<name>", defaultName(m.repo.Name))
	preview := ""
	if v := strings.TrimSpace(m.nameIn.ti.Value()); v != "" {
		preview = fmt.Sprintf("worktree 路径: %s\n新分支: wt/%s",
			filepath.Join(m.cfg.WorktreeRoot, v), v)
	}
	return subtleStyle.Render("为 worktree 命名") + "\n\n" +
		inputStyle.Render(m.nameIn.ti.View()) + "\n\n" +
		subtleStyle.Render(tip) + "\n" + okStyle.Render(preview)
}

//
// ─── 会话模式选择 ─────────────────────────────────
//

type sessionSelectModel struct {
	list simpleList
}

func (m Model) enterSessionPage() (tea.Model, tea.Cmd) {
	m.page = pageCreateSession
	var items []listItem
	for _, mode := range session.Modes() {
		items = append(items, listItem{
			Title:       session.ModeLabel(mode),
			Description: sessionModeHelp(mode),
			Value:       mode,
		})
	}
	m.session = sessionSelectModel{list: newList("完成后如何进入 worktree？", items)}
	// 光标定位到默认
	for i, it := range items {
		if it.Value == m.pick.session {
			m.session.list.cursor = i
			break
		}
	}
	return m, nil
}

func sessionModeHelp(mode config.SessionMode) string {
	switch mode {
	case config.SessionSpawn:
		return "尝试打开新的终端窗口并 cd 到 worktree"
	case config.SessionEval:
		return "输出 shell 片段（需要 wrapper 函数配合）"
	default:
		return "打印 cd 命令并尝试复制到剪贴板"
	}
}

func (m Model) updateSession(msg tea.Msg) (tea.Model, tea.Cmd) {
	activate := false
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m.enterNamePage()
		case "enter":
			activate = true
		}
	}
	if m.session.list.Update(msg) {
		activate = true
	}
	if !activate {
		return m, nil
	}
	sel := m.session.list.Selected()
	if sel == nil {
		return m, nil
	}
	m.pick.session = sel.Value.(config.SessionMode)
	return m.enterConfirmPage()
}

func (m Model) viewSession() string {
	return m.session.list.View(m.listHeight()) + "\n\n"
}

//
// ─── 汇总确认 ─────────────────────────────────
//

type confirmModel struct{}

func (m Model) enterConfirmPage() (tea.Model, tea.Cmd) {
	m.page = pageCreateConfirm
	return m, nil
}

func (m Model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m.enterSessionPage()
		case "enter", "y":
			return m.enterRunningPage()
		}
	}
	return m, nil
}

func (m Model) viewConfirm() string {
	envDesc := "(无)"
	if m.pick.envFile != "" {
		envDesc = m.pick.envFile
	}
	lines := []string{
		"仓库源:       " + m.repo.Root,
		"基线分支:     " + m.pick.baseBranch,
		"新分支:       " + m.pick.newBranch,
		"worktree 名: " + m.pick.name,
		"worktree 路径: " + m.pick.path,
		"环境:         " + envDesc,
		"会话模式:     " + session.ModeLabel(m.pick.session),
	}
	return panelStyle.Render(strings.Join(lines, "\n")) + "\n"
}

//
// ─── 执行中 ─────────────────────────────────
//

type runningModel struct {
	stage   string // 当前显示的阶段文字
	tick    int    // spinner 帧
	done    bool
	failure error

	sw      *streamWriter      // 脚本输出，只取最新一行显示
	out     string             // sw 的快照（每个 tick 刷新一次）
	startAt time.Time          // 用于显示已运行时长
	cancel  context.CancelFunc // 中止脚本（连子孙进程一起杀）
}

type createStepMsg struct {
	line string
}
type createDoneMsg struct {
	err         error
	spawnScript string // env.OnSpawned 挑选出的脚本
}

// runningStream 使用一个 channel 把脚本输出的每一行送到 tea 事件循环
// tea.Program 通过反复调用 waitStep() 来"拉"消息，从而实现实时刷新
var runningStream = make(chan tea.Msg, 256)

// waitStep 一个 tea.Cmd：阻塞直到 stream 有下一条消息
func waitStep() tea.Cmd {
	return func() tea.Msg {
		return <-runningStream
	}
}

// streamWriter 接收脚本 stdout/stderr，只留最新一行给 UI 展示
//
// 不往 runningStream 灌：pnpm install 这类脚本一秒能刷几百行，
// 灌进事件循环会把 UI 冲垮。这里只维护"最后一行"，UI 按 tick 来取，
// 于是既不会撑破布局，用户也能看出脚本到底还在动没有 —— 之前是整个丢掉，
// 一个下载得一小时的 pnpm install 和真死锁在界面上长得一模一样。
type streamWriter struct {
	mu   sync.Mutex
	part []byte // 还没凑成一行的尾巴
	last string
}

// ansiRe 用来剥掉进度条的颜色/光标控制序列
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\][^\x07]*\x07`)

func (s *streamWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.part = append(s.part, p...)
	// 进度条用 \r 原地刷新，两种换行都当行分隔
	if i := bytes.LastIndexAny(s.part, "\r\n"); i >= 0 {
		for _, line := range bytes.FieldsFunc(s.part[:i], func(r rune) bool { return r == '\r' || r == '\n' }) {
			if t := strings.TrimSpace(ansiRe.ReplaceAllString(string(line), "")); t != "" {
				s.last = t
			}
		}
		s.part = append([]byte(nil), s.part[i+1:]...)
	}
	// 脚本一行不换地刷输出时别把内存吃光
	if len(s.part) > 8192 {
		s.part = s.part[len(s.part)-4096:]
	}
	return len(p), nil
}

// Flush 把最后没换行的一截也交出来
func (s *streamWriter) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t := strings.TrimSpace(ansiRe.ReplaceAllString(string(s.part), "")); t != "" {
		s.last = t
	}
	s.part = nil
}

// Last 最新一行脚本输出
func (s *streamWriter) Last() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}

func (m Model) enterRunningPage() (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	m.page = pageCreateRunning
	m.running = runningModel{
		stage:   "Creating worktree...",
		sw:      &streamWriter{},
		startAt: time.Now(),
		cancel:  cancel,
	}
	go m.runCreateAsync(ctx, m.running.sw)
	return m, tea.Batch(waitStep(), tickCmd())
}

// runCreateAsync 后台跑创建流程，通过 runningStream 送状态更新
func (m Model) runCreateAsync(ctx context.Context, sw *streamWriter) {
	pick := m.pick
	repo := m.repo

	runningStream <- createStepMsg{line: "[wtc] 正在创建 worktree..."}
	if err := repo.AddWorktree(pick.path, pick.baseBranch, pick.newBranch); err != nil {
		runningStream <- createDoneMsg{err: fmt.Errorf("创建 worktree 失败: %w", err)}
		return
	}
	runningStream <- createStepMsg{line: "[wtc] ✓ worktree 已创建: " + pick.path}

	if pick.envFile == "" {
		runningStream <- createDoneMsg{}
		return
	}

	envDef, err := env.Load(pick.envFile)
	if err != nil {
		runningStream <- createDoneMsg{err: fmt.Errorf("加载环境失败: %w", err)}
		return
	}
	vars := env.Vars{
		SourceTreePath: repo.Root,
		WorktreePath:   pick.path,
		WorktreeName:   pick.name,
		Branch:         pick.newBranch,
		BaseBranch:     pick.baseBranch,
	}
	runningStream <- createStepMsg{line: "[wtc] 执行环境 onCreate 脚本..."}
	err = env.Run(ctx, envDef, env.PhaseCreate, vars, sw)
	sw.Flush()
	if ctx.Err() != nil {
		// 用户按 Esc 中止：脚本（连子孙进程）已经被杀掉，不用再报错
		runningStream <- createDoneMsg{err: errors.New("已中止 onCreate 脚本（worktree 已创建，可在管理面板删除）")}
		return
	}
	if err != nil {
		runningStream <- createDoneMsg{err: fmt.Errorf("脚本执行失败: %w", err)}
		return
	}
	spawnScript := env.Render(envDef.OnSpawned.PickScript(runtimeGOOS()), vars)
	runningStream <- createDoneMsg{spawnScript: spawnScript}
}

func (m Model) updateRunning(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case createStepMsg:
		// 只保留最新阶段
		m.running.stage = msg.line
		return m, waitStep()
	case createDoneMsg:
		m.running.done = true
		m.running.failure = msg.err
		m.pick.spawnScript = msg.spawnScript
		return m.enterDonePage()
	case tickMsg:
		m.running.tick++
		if m.running.sw != nil {
			m.running.out = m.running.sw.Last()
		}
		return m, tickCmd()
	case tea.KeyMsg:
		if msg.String() == "esc" || msg.String() == "q" {
			// 先把脚本（含 pnpm/node 这些子孙进程）杀掉再退，
			// 否则 wtc 关了脚本还在后台跑，用户重试几次就攒出一堆并发任务
			if m.running.cancel != nil {
				m.running.cancel()
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

// tickMsg 定时事件，用于驱动 spinner
type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(_ time.Time) tea.Msg {
		return tickMsg{}
	})
}

// spinnerFrames 简单旋转字符
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m Model) viewRunning() string {
	frame := spinnerFrames[m.running.tick%len(spinnerFrames)]
	stage := m.running.stage
	if stage == "" {
		stage = "Creating worktree..."
	}
	// 截断到窗口宽度，避免换行破坏布局
	maxWidth := m.width - 10
	if maxWidth < 20 {
		maxWidth = 40
	}
	stage = truncateForDisplay(stage, maxWidth)
	title := okStyle.Render(frame + " " + stage)

	// 已运行时长 + 脚本最新一行输出：
	// onCreate 里跑 pnpm install / 构建动辄几分钟，没有这两行没法判断是慢还是卡死
	body := ""
	if !m.running.startAt.IsZero() {
		body += "\n  " + subtleStyle.Render("已运行 "+formatElapsed(time.Since(m.running.startAt)))
	}
	if m.running.out != "" {
		body += "\n  " + subtleStyle.Render("› "+truncateForDisplay(m.running.out, maxWidth-4))
	}
	hint := subtleStyle.Render("按 Esc/q 中止脚本并退出")
	return "\n" + title + body + "\n\n" + hint
}

// formatElapsed 把时长显示成 mm:ss（超过一小时带上小时）
func formatElapsed(d time.Duration) string {
	s := int(d.Seconds())
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, (s%3600)/60, s%60)
	}
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

// truncateForDisplay 按 rune 数截断，超长加 …
func truncateForDisplay(s string, max int) string {
	if max <= 3 {
		return "..."
	}
	// 先去掉 ANSI 转义（简单粗暴的按 rune 计数即可，不精确但够用）
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	return string(rs[:max-1]) + "…"
}

//
// ─── 完成 ─────────────────────────────────
//

type doneModel struct {
	output string
}

func (m Model) enterDonePage() (tea.Model, tea.Cmd) {
	m.page = pageCreateDone
	m.done = doneModel{}
	// 如果失败，直接展示错误
	if m.running.failure != nil {
		m.done.output = errStyle.Render("失败: "+m.running.failure.Error()) + "\n"
		return m, nil
	}
	m.done.output = okStyle.Render("✓ worktree 创建成功") + "\n" + subtleStyle.Render("正在进入会话...") + "\n"
	// 会话启动走异步 tea.Cmd：spawn 模式要开新终端窗口（osascript / open），
	// 放在 Update 里同步跑会把整个 UI 卡住
	return m, m.launchSessionCmd(m.pick.path, m.pick.spawnScript, m.pick.session)
}

// sessionLaunchedMsg 会话启动结果（异步）
type sessionLaunchedMsg struct {
	output string
	err    error
}

// launchSessionCmd 在后台线程里启动会话，结果回到事件循环
func (m Model) launchSessionCmd(path, initScript string, mode config.SessionMode) tea.Cmd {
	eval := m.eval
	return func() tea.Msg {
		launcher := session.New(mode)
		var evalW *evalStdoutWriter
		if eval {
			evalW = &evalStdoutWriter{}
		}
		var buf strings.Builder
		err := launcher.Launch(path, &buf, evalW, initScript)
		return sessionLaunchedMsg{output: buf.String(), err: err}
	}
}

func (m Model) updateDone(msg tea.Msg) (tea.Model, tea.Cmd) {
	if launched, ok := msg.(sessionLaunchedMsg); ok {
		m.done.output = okStyle.Render("✓ worktree 创建成功") + "\n" + launched.output
		if launched.err != nil {
			m.done.output += errStyle.Render("启动会话失败: "+launched.err.Error()) + "\n"
		}
		return m, nil
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter", "q", "esc":
			return m, tea.Quit
		case "h":
			m.page = pageHome
			m.pick = pickState{session: m.cfg.SessionMode}
			return m, nil
		}
	}
	return m, nil
}

func (m Model) viewDone() string {
	return panelStyle.Render(m.done.output) + "\n"
}
