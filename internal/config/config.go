package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SessionMode 会话启动模式
type SessionMode string

const (
	SessionPrint SessionMode = "print"
	SessionSpawn SessionMode = "spawn"
	SessionEval  SessionMode = "eval"
)

// Config 全局配置
type Config struct {
	Version            int         `yaml:"version"`
	WorktreeRoot       string      `yaml:"worktreeRoot"`
	AutoRemove         bool        `yaml:"autoRemove"`
	Retention          int         `yaml:"retention"`
	SessionMode        SessionMode `yaml:"sessionMode"`
	DefaultEnvironment string      `yaml:"defaultEnvironment"`
	Editor             string      `yaml:"editor"`
}

// Default 返回默认配置
func Default() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Version:      1,
		WorktreeRoot: filepath.ToSlash(filepath.Join(home, "wtc-worktrees")),
		AutoRemove:   true,
		Retention:    15,
		SessionMode:  SessionPrint,
	}
}

// Load 读取配置文件
func Load() (*Config, error) {
	p, err := ConfigFile()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return nil, err
	}
	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Save 写入配置文件
// 会尽量保留文件开头连续的注释块（`#` 开头的行）
func Save(cfg *Config) error {
	p, err := ConfigFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	// 尝试保留原文件顶部注释
	if header := readTopComments(p); header != "" {
		data = append([]byte(header), data...)
	}
	return os.WriteFile(p, data, 0o644)
}

// readTopComments 读取文件开头连续的注释/空行段
func readTopComments(path string) string {
	f, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var buf []byte
	for _, line := range splitLinesKeepEnding(string(f)) {
		trimmed := line
		// 允许开头有 BOM/空白
		t := trimSpaces(trimmed)
		if t == "" || (len(t) > 0 && t[0] == '#') {
			buf = append(buf, line...)
			continue
		}
		break
	}
	return string(buf)
}

func splitLinesKeepEnding(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func trimSpaces(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\r' || s[len(s)-1] == '\n') {
		s = s[:len(s)-1]
	}
	return s
}

// writeDefaultConfig 释放带完整注释的初始 config.yml
func writeDefaultConfig(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	def := Default()
	// 用 filepath 风格的分隔符写入，Windows 用户直观
	root := filepath.ToSlash(def.WorktreeRoot)
	body := replaceAll(defaultConfigTemplate, defaultWorktreeRootPlaceholder, root)
	return os.WriteFile(path, []byte(body), 0o644)
}

func replaceAll(s, old, new string) string {
	// 避免额外依赖 strings，包内实现（此文件已不需要 strings）
	if old == "" {
		return s
	}
	var out []byte
	i := 0
	for i < len(s) {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			out = append(out, new...)
			i += len(old)
			continue
		}
		out = append(out, s[i])
		i++
	}
	return string(out)
}

const defaultConfigTemplate = `# wtc 全局配置
# 位置: ~/.wtc/config.yml
# 通过 TUI 的"设置"页或直接编辑本文件都可以修改
# TUI 保存时会尽量保留本文件顶部的注释

# 配置文件版本（不要修改）
version: 1

# 新建 worktree 存放的根目录
# 每个 worktree 会被创建在 <worktreeRoot>/<worktree名> 下
worktreeRoot: '` + defaultWorktreeRootPlaceholder + `'

# 自动删除旧 worktree
# 超过 retention 数量的最旧 worktree 会在创建新的时被清理
autoRemove: true

# 保留的 worktree 数量上限（配合 autoRemove）
retention: 15

# 创建完成后如何进入 worktree
# print — 打印 cd 命令并尝试写剪贴板（推荐，最稳）
# spawn — 尝试打开新的终端窗口
# eval  — 输出 shell 片段（需配合 wrapper 函数使用）
sessionMode: print

# 默认使用的环境文件名（可选）
# 留空则每次都提示选择
defaultEnvironment: ""

# 外部编辑器（可选，当前未使用；预留字段）
editor: ""
`

// defaultWorktreeRootPlaceholder 是模板中的占位符，写入时会被替换为真实路径
const defaultWorktreeRootPlaceholder = "__WORKTREE_ROOT__"

// EnsureInit 首次运行时初始化目录并释放默认环境
func EnsureInit() error {
	root, err := Root()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}

	envDir, _ := EnvironmentsDir()
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		return err
	}

	// 写入默认 config（如果不存在）—— 使用带注释的模板
	cfgPath, _ := ConfigFile()
	if _, err := os.Stat(cfgPath); errors.Is(err, os.ErrNotExist) {
		if err := writeDefaultConfig(cfgPath); err != nil {
			return err
		}
	}

	// 释放默认环境
	return writeDefaultEnvs(envDir)
}

const defaultCodegraphEnv = `# 环境名称（在 TUI 列表中显示）
name: "CodeGraph 初始化"
# 环境描述
description: "在新工作树上初始化 codegraph 索引"

# 平台块规则（onCreate / onSpawned / onCleanup 通用）:
#   支持 default / macos / linux / windows 四个键
#   运行时优先匹配当前系统，匹配不到则回退到 default
#   留空则跳过
#
# 可用环境变量:
#   $CODEX_SOURCE_TREE_PATH  — 原始 git 仓库路径
#   $CODEX_WORKTREE_PATH     — 新建 worktree 的绝对路径
#   $CODEX_WORKTREE_NAME     — worktree 名称
#   $CODEX_BRANCH            — 新建分支名
#   $CODEX_BASE_BRANCH       — 基线分支名
#   （Windows 中使用 $env:CODEX_WORKTREE_PATH 形式）

# onCreate: worktree 创建后立即在后台执行的脚本（wtc 内部子进程运行）
# 适合做一次性初始化: 索引构建、依赖安装、生成配置文件等
onCreate:
  default: |
    cd "$CODEX_WORKTREE_PATH"
    echo "worktree ready: $CODEX_WORKTREE_PATH"
  macos: |
    cd "$CODEX_WORKTREE_PATH"
    codegraph init
  linux: |
    cd "$CODEX_WORKTREE_PATH"
    codegraph init
  windows: |
    Set-Location "$env:CODEX_WORKTREE_PATH"
    codegraph init

# onSpawned: 会话模式为 spawn 时，在新终端窗口内自动执行的脚本
# 与 onCreate 的区别: onSpawned 运行在你新打开的交互式终端里
# 适合做需要人机交互或需要保留 shell 状态的动作: 激活 venv、启动 dev server、tail 日志
onSpawned:
  default: ""
  macos: |
    echo "welcome to $CODEX_WORKTREE_NAME"
  linux: |
    echo "welcome to $CODEX_WORKTREE_NAME"
  windows: |
    Write-Host "welcome to $env:CODEX_WORKTREE_NAME"

# onCleanup: worktree 被删除前执行（可选）
onCleanup:
  default: ""
`

const defaultNoopEnv = `# 空环境 —— 只创建 worktree，不运行任何脚本
name: "空环境"
description: "不执行任何脚本，只创建 worktree"

# 全部留空即跳过
onCreate:
  default: ""
onSpawned:
  default: ""
onCleanup:
  default: ""
`

func writeDefaultEnvs(dir string) error {
	files := map[string]string{
		"codegraph.yml": defaultCodegraphEnv,
		"noop.yml":      defaultNoopEnv,
	}
	for name, body := range files {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			continue
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}
