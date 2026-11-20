package scaffold

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestInitForce_BackupsExisting B3：init -f 重建前应先把旧 .sunshine/
// 备份到同级 .sunshine.backup-<时间戳> 目录，旧任务文件在备份里完好，
// 新 .sunshine/ 已重建；同秒内再次 -f 不应因备份名冲突失败。
func TestInitForce_BackupsExisting(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir, "张三", false); err != nil {
		t.Fatalf("首次 Init 失败：%v", err)
	}
	// 在旧 .sunshine/ 里放一份“用户数据”：任务文件与 journal。
	oldTask := filepath.Join(dir, ".sunshine", "tasks", "11-09-old-task", "task.json")
	if err := os.MkdirAll(filepath.Dir(oldTask), 0o755); err != nil {
		t.Fatal(err)
	}
	oldContent := []byte("{\"id\":\"old-task\",\"title\":\"旧任务\"}\n")
	if err := os.WriteFile(oldTask, oldContent, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".sunshine", "journal.md"), []byte("旧日志\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Init(dir, "李四", true); err != nil {
		t.Fatalf("force Init 失败：%v", err)
	}

	backups, err := filepath.Glob(filepath.Join(dir, ".sunshine.backup-*"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("应恰好生成 1 个备份目录，实际 %v（err=%v）", backups, err)
	}
	nameRe := regexp.MustCompile(`^\.sunshine\.backup-\d{8}-\d{6}(-\d+)?$`)
	if base := filepath.Base(backups[0]); !nameRe.MatchString(base) {
		t.Errorf("备份目录命名不合格式 .sunshine.backup-YYYYMMDD-HHMMSS：%s", base)
	}
	kept, err := os.ReadFile(filepath.Join(backups[0], "tasks", "11-09-old-task", "task.json"))
	if err != nil {
		t.Fatalf("备份里应含旧任务文件：%v", err)
	}
	if string(kept) != string(oldContent) {
		t.Errorf("备份里的旧任务文件内容不对：%q", kept)
	}
	if _, err := os.Stat(filepath.Join(backups[0], "journal.md")); err != nil {
		t.Errorf("备份里应含旧 journal：%v", err)
	}
	// 新 .sunshine/ 已重建，且不含旧任务。
	if _, err := os.Stat(filepath.Join(dir, ".sunshine", "config.yaml")); err != nil {
		t.Errorf("重建后 .sunshine/config.yaml 应存在：%v", err)
	}
	dev, _ := os.ReadFile(filepath.Join(dir, ".sunshine/.developer"))
	if strings.TrimSpace(string(dev)) != "李四" {
		t.Errorf("重建后 .developer 应为李四，实际 %q", dev)
	}
	if _, err := os.Stat(filepath.Join(dir, ".sunshine", "tasks", "11-09-old-task")); !os.IsNotExist(err) {
		t.Errorf("重建后的 .sunshine 不应含旧任务目录（err=%v）", err)
	}

	// 同秒内再次 force：备份名冲突要加后缀，不能失败也不能覆盖上一份备份。
	if err := Init(dir, "王五", true); err != nil {
		t.Fatalf("第二次 force Init 失败（备份名冲突处理有问题）：%v", err)
	}
	backups2, _ := filepath.Glob(filepath.Join(dir, ".sunshine.backup-*"))
	if len(backups2) != 2 {
		t.Errorf("第二次 force 后应有 2 个备份目录，实际 %v", backups2)
	}
}
