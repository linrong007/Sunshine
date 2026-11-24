// Package cli 的 sunshine codegraph 命令：手动管理 CodeGraph 代码地图。
// init 只对存量项目供给（装工具先经同意）；本命令给用户一个随时手动触发的入口：
// install 装工具、接线、建图（缺什么补什么），status 只看状态不动手。
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/linrong007/Sunshine/internal/codegraph"
)

// RegisterCodegraphCommand 把 `sunshine codegraph` 接到根命令上。
func RegisterCodegraphCommand(root *cobra.Command) {
	cgCmd := &cobra.Command{
		Use:   "codegraph",
		Short: "管理代码地图（CodeGraph）",
		Long: `管理代码地图（CodeGraph）：把项目代码解析成符号与调用关系图，
供 agent 查“谁调用了这个函数、改一处波及哪里”。图由 CodeGraph 自己
维护（文件变化自动同步），Sunshine 只负责装工具、接线和建图。

sunshine init 只对存量项目供给（装工具先经同意）；本命令可随时手动触发。
在非 Sunshine 项目目录里也能用（只建图，不接线）。`,
	}

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "装 CodeGraph 工具、接线到已装平台、为当前项目建图",
		Long: `为当前目录手动供给代码地图，缺什么补什么：

  1. 机器上没有 CodeGraph 时用官方脚本安装（失败退 npm 全局安装）；
  2. 把 CodeGraph 查询接进本项目已安装的 Sunshine 平台；
  3. 没有 .codegraph/ 时为当前项目建图。

与 init 的供给不同，手动执行不受“存量项目”判定限制、也不再询问，
且每次都会重新接线。任何一步失败只提示并跳过，不影响其余步骤。
SUNSHINE_NO_CODEGRAPH=1 时本命令同样停用。`,
		Example: `  sunshine codegraph install`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			if codegraphDisabled() {
				fmt.Println("代码地图已被关闭（SUNSHINE_NO_CODEGRAPH=1），未执行。")
				return nil
			}
			platforms := installedPlatformIDs(cwd)
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()
			res := codegraph.Provision(ctx, codegraph.Options{
				Dir: cwd, Legacy: true, Force: true, AllowInstall: true,
				Platforms: platforms, PlatformsInstalled: len(platforms),
				Progress: func(line string) { fmt.Println(line) },
			}, codegraph.StreamingRunner(os.Stdout))
			for _, line := range res.Lines {
				fmt.Println(line)
			}
			return nil
		},
	}

	statusCmd := &cobra.Command{
		Use:     "status",
		Short:   "查看代码地图状态（工具、项目图、可接线平台），不做任何改动",
		Example: `  sunshine codegraph status`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			fmt.Println("代码地图（CodeGraph）状态：")
			if bin, ok := codegraph.DefaultRunner().LookPath("codegraph"); ok {
				fmt.Printf("  工具：已安装（%s）\n", bin)
			} else {
				fmt.Println("  工具：未安装（运行 sunshine codegraph install 安装）")
			}
			if dirExists(filepath.Join(cwd, ".codegraph")) {
				fmt.Println("  项目图：已建（.codegraph/）")
			} else {
				fmt.Println("  项目图：未建（运行 sunshine codegraph install 建图）")
			}
			if codegraph.HasExistingCode(cwd) {
				fmt.Println("  存量代码：检测到（init 会供给代码地图，未装工具时先问）")
			} else {
				fmt.Println("  存量代码：未检测到（init 不供给，手动 install 不受此限）")
			}
			platforms := installedPlatformIDs(cwd)
			if targets := codegraph.TargetsFor(platforms); len(targets) > 0 {
				fmt.Printf("  可接线平台：%s → %s\n", strings.Join(platformNames(platforms), "、"), strings.Join(targets, "、"))
			} else if len(platforms) > 0 {
				fmt.Printf("  已装平台：%s（均无 CodeGraph 接线目标，靠 CLI 命令使用）\n", strings.Join(platformNames(platforms), "、"))
			} else {
				fmt.Println("  已装平台：无（非 Sunshine 项目或尚未 init）")
			}
			return nil
		},
	}

	cgCmd.AddCommand(installCmd, statusCmd)
	root.AddCommand(cgCmd)
}

// installedPlatformIDs 返回当前目录已安装的 Sunshine 平台 ID 列表，
// 按平台注册顺序排列（判定口径与 init 的 filterUninstalled 一致）。
func installedPlatformIDs(cwd string) []string {
	var out []string
	for _, id := range platformIDs {
		if platformInstalled(cwd, id) {
			out = append(out, id)
		}
	}
	return out
}

// platformNames 把平台 ID 列表换成中文展示名。
func platformNames(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if name, ok := platformDisplayNames[id]; ok {
			out = append(out, name)
		} else {
			out = append(out, id)
		}
	}
	return out
}
