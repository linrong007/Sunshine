// Package cli 的 sunshine spec 命令：维护 .sunshine/spec 下的规范索引。
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/linrong007/Sunshine/internal/specindex"
	"github.com/linrong007/Sunshine/internal/task"
)

// RegisterSpecCommand 把 `sunshine spec` 接到根命令上。
func RegisterSpecCommand(root *cobra.Command) {
	specCmd := &cobra.Command{
		Use:   "spec",
		Short: "维护项目规范（.sunshine/spec/）",
		Long:  `维护项目规范。\n\n索引是导航，不是替代：spec 文件头上的 ## 索引一节只列各节位置，\nagent 先读索引定位，再按需读正文；拿不准就读正文。`,
	}

	indexCmd := &cobra.Command{
		Use:   "index [文件...]",
		Short: "同步 spec 文件头上的索引节（不指定文件时同步全部）",
		Long: `同步 spec 文件头上的 ## 索引节。

按各文件的 ## 标题重新生成索引表：新增的节进索引，删掉的节出索引，
旧索引里的描述（括号部分）保留，新节只写标题。只改索引节，
正文一个字不动；反复跑结果相同。

不指定文件时同步当前项目 .sunshine/spec/ 下全部 Markdown 文件
（递归，跳过 guides/）。指定文件时只同步那几个文件（须在项目内）。

  --check    只检查不同步，不写盘；有不同步时退出码为 1（供 CI / pre-commit 用）
  --dry-run  只显示会改动的文件，不写盘`,
		Example: `  sunshine spec index
  sunshine spec index .sunshine/spec/backend.md
  sunshine spec index --check
  sunshine spec index --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			check, _ := cmd.Flags().GetBool("check")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			write := !check && !dryRun

			var files []string
			if len(args) > 0 {
				for _, a := range args {
					abs, err := filepath.Abs(a)
					if err != nil {
						return err
					}
					files = append(files, abs)
				}
			} else {
				root := task.FindRoot("")
				if root == "" {
					return fmt.Errorf("当前目录不在 Sunshine 项目内（未找到 .sunshine/），请先 cd 到项目目录，或运行 sunshine init 初始化")
				}
				specDir := filepath.Join(root, ".sunshine", "spec")
				found, err := specindex.SpecFiles(specDir)
				if err != nil {
					return err
				}
				if len(found) == 0 {
					fmt.Println("没有可同步的 spec 文件（.sunshine/spec/ 下无 Markdown 文件）")
					return nil
				}
				files = found
			}

			var changed []string
			for _, f := range files {
				if write {
					ok, err := specindex.SyncFile(f)
					if err != nil {
						return err
					}
					if ok {
						changed = append(changed, f)
					}
				} else {
					ok, err := specindex.CheckFile(f)
					if err != nil {
						return err
					}
					if ok {
						changed = append(changed, f)
					}
				}
			}

			if len(changed) == 0 {
				fmt.Println("索引已同步，无需改动")
				return nil
			}
			verb := "已同步"
			if check {
				verb = "不同步"
			} else if dryRun {
				verb = "将同步"
			}
			fmt.Printf("%s %d 个文件：\n", verb, len(changed))
			for _, f := range changed {
				rel := f
				if cwd, err := os.Getwd(); err == nil {
					if r, err := filepath.Rel(cwd, f); err == nil && !strings.HasPrefix(r, "..") {
						rel = r
					}
				}
				fmt.Printf("  %s\n", rel)
			}
			if check {
				return fmt.Errorf("索引不同步（%d 个文件），请运行 sunshine spec index 同步", len(changed))
			}
			return nil
		},
	}
	indexCmd.Flags().Bool("check", false, "只检查不同步，不写盘")
	indexCmd.Flags().Bool("dry-run", false, "只显示会改动的文件，不写盘")
	specCmd.AddCommand(indexCmd)
	root.AddCommand(specCmd)
}
