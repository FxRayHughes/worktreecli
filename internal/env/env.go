package env

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/FxRayHughes/worktreecli/internal/config"
	"gopkg.in/yaml.v3"
)

// Scripts 分平台脚本
type Scripts struct {
	Default string `yaml:"default"`
	MacOS   string `yaml:"macos"`
	Linux   string `yaml:"linux"`
	Windows string `yaml:"windows"`
}

// Environment 单个环境定义
type Environment struct {
	FileName    string  `yaml:"-"` // 不写盘的元数据
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	OnCreate    Scripts `yaml:"onCreate"`
	OnSpawned   Scripts `yaml:"onSpawned"`
	OnCleanup   Scripts `yaml:"onCleanup"`
}

// PickScript 按 goos 选择对应脚本，回退到 default
func (s Scripts) PickScript(goos string) string {
	var picked string
	switch goos {
	case "darwin":
		picked = s.MacOS
	case "linux":
		picked = s.Linux
	case "windows":
		picked = s.Windows
	}
	if strings.TrimSpace(picked) != "" {
		return picked
	}
	return s.Default
}

// List 列出 ~/.wtc/environments 下所有 yml
func List() ([]*Environment, error) {
	dir, err := config.EnvironmentsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var envs []*Environment
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !(strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")) {
			continue
		}
		env, err := loadFile(filepath.Join(dir, name))
		if err != nil {
			// 单文件错误不阻塞其他
			continue
		}
		env.FileName = name
		envs = append(envs, env)
	}
	sort.Slice(envs, func(i, j int) bool { return envs[i].FileName < envs[j].FileName })
	return envs, nil
}

// Load 按文件名读取
func Load(fileName string) (*Environment, error) {
	dir, err := config.EnvironmentsDir()
	if err != nil {
		return nil, err
	}
	return loadFile(filepath.Join(dir, fileName))
}

func loadFile(path string) (*Environment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	env := &Environment{}
	if err := yaml.Unmarshal(data, env); err != nil {
		return nil, fmt.Errorf("解析 %s 失败: %w", filepath.Base(path), err)
	}
	env.FileName = filepath.Base(path)
	return env, nil
}
