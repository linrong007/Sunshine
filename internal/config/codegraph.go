package config

// .sunshine/config.yaml 里 codegraph.install_declined 的行级读写：
// 记录「用户已在 init 时明确拒绝过安装 CodeGraph」，之后 init
// 不再重复发问（问一次口径）。行级改写保住用户文件里的注释与
// 排版，与 registry 的行级读写同法；整个 Config 结构不加此字段，
// YAML 全量解析时它只是未知键、无副作用。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CodegraphDeclined 报告项目是否记录过「拒绝安装 CodeGraph」。
// 文件缺失或内容损坏时按未拒绝处理（宁可多问一次，不漏问）。
func CodegraphDeclined(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, ".sunshine", "config.yaml"))
	if err != nil {
		return false
	}
	_, _, found := findDeclinedLine(strings.Split(string(data), "\n"))
	return found
}

// SetCodegraphDeclined 行级写入 / 清除拒绝记录。
// declined 为真时确保 codegraph 节里有 install_declined: true
// （没有 codegraph 节就追加一节）；为假时删掉该行（节保留无妨）。
func SetCodegraphDeclined(root string, declined bool) error {
	path := filepath.Join(root, ".sunshine", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if !declined || !os.IsNotExist(err) {
			return nil // 清除时文件不在就算了；写入时文件不在也算了（init 骨架未写盘前不落记录）
		}
		if mkErr := os.MkdirAll(filepath.Dir(path), 0o755); mkErr != nil {
			return mkErr
		}
		return os.WriteFile(path, []byte("codegraph:\n  install_declined: true\n"), 0o644)
	}
	lines := strings.Split(string(data), "\n")
	secStart, keyIdx, _ := findDeclinedLine(lines)
	switch {
	case declined && keyIdx >= 0:
		lines[keyIdx] = "  install_declined: true"
	case declined && secStart >= 0:
		lines = insertLine(lines, secStart+1, "  install_declined: true")
	case declined:
		content := string(data)
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += "\ncodegraph:\n  install_declined: true\n"
		return os.WriteFile(path, []byte(content), 0o644)
	case keyIdx >= 0: // 清除
		lines = append(lines[:keyIdx:keyIdx], lines[keyIdx+1:]...)
	default:
		return nil
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return fmt.Errorf("写入 CodeGraph 拒绝记录失败：%w", err)
	}
	return nil
}

// findDeclinedLine 在行列表里定位 codegraph 节与 install_declined 键：
// 返回（节起始行下标或 -1，键行下标或 -1，键值是否为 true）。
func findDeclinedLine(lines []string) (secStart, keyIdx int, isTrue bool) {
	secStart, keyIdx = -1, -1
	inSection := false
	for i, line := range lines {
		trimmed := strings.TrimRight(line, " \t\r")
		if trimmed == "codegraph:" {
			secStart, inSection = i, true
			continue
		}
		if inSection {
			if trimmed != "" && !strings.HasPrefix(trimmed, " ") && !strings.HasPrefix(trimmed, "\t") && !strings.HasPrefix(trimmed, "#") {
				inSection = false // 下一节开始
				continue
			}
			t := strings.TrimSpace(trimmed)
			if strings.HasPrefix(t, "install_declined:") {
				keyIdx = i
				isTrue = strings.TrimSpace(strings.TrimPrefix(t, "install_declined:")) == "true"
			}
		}
	}
	return secStart, keyIdx, isTrue
}

func insertLine(lines []string, idx int, line string) []string {
	lines = append(lines, "")
	copy(lines[idx+1:], lines[idx:])
	lines[idx] = line
	return lines
}
