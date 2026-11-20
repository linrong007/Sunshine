package cli

import (
	"strings"
	"testing"
)

// TestInitForceHelpMentionsBackup_B3 B3：init -f 帮助文案应写明
// 会先把旧 .sunshine/ 备份到 .sunshine.backup-<时间戳> 再重建。
func TestInitForceHelpMentionsBackup_B3(t *testing.T) {
	flag := initCmd.Flags().Lookup("force")
	if flag == nil {
		t.Fatal("init 应有 --force flag")
	}
	if !strings.Contains(flag.Usage, ".sunshine.backup-") || !strings.Contains(flag.Usage, "备份") {
		t.Errorf("--force 帮助文案应写明先备份到 .sunshine.backup-<时间戳>，实际：%q", flag.Usage)
	}
}
