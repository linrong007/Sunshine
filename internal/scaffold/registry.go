package scaffold

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/linrong007/Sunshine/internal/registry"
	"github.com/linrong007/Sunshine/templates"
)

// InstallRegistry 从 registry 安装模板（marketplace 模式或直接下载模式），
// 是 sunshine init --registry 的 scaffold 层入口。
// 返回 registry.InstallResult：安装模式、安装的文件（相对项目根，
// 供调用方记入 .template-hashes.json）、待持久化的 registry.spec。
func InstallRegistry(cwd string, opts registry.InstallOptions) (*registry.InstallResult, error) {
	return registry.Install(cwd, opts)
}

// WriteDefaultSpec 把内置默认 spec 模板写进 .sunshine/spec，
// 返回实际写入的文件（相对项目根的 POSIX 路径，供调用方记入哈希清单）。
// 用于 --registry 安装失败时回退到空白模板（对应上游 init 的
// "Falling back to blank templates..."）。
// 只补缺失（与 writeDesignTemplates 同规矩）：目标文件已存在时原样
// 保留、不覆盖，也不在返回清单里——增量 init 的回退不能清空用户
// 手写的规范，更不能把手写内容记成模板哈希。全新 init 时文件都
// 不存在，照常全写。
func WriteDefaultSpec(cwd string) ([]string, error) {
	target := filepath.Join(cwd, ".sunshine", "spec")
	var files []string
	err := fs.WalkDir(templates.Sunshine, "sunshine/spec", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(path, "sunshine/spec/")
		dest := filepath.Join(target, filepath.FromSlash(rel))
		if _, err := os.Stat(dest); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("检查 %s 失败：%w", dest, err)
		}
		data, err := templates.Sunshine.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(filepath.Join(".sunshine/spec", rel)))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("写入默认规范模板失败：%w", err)
	}
	return files, nil
}
