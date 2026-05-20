package i18n

var zh = map[string]string{
	// root
	"root.short":           "Git 工作区同步与迁移工具",
	"root.long":            "扫描目录发现 Git 仓库，显示多仓库状态，快照状态，并在另一台机器上恢复。",
	"root.flag.config-dir": "配置目录（默认: ~/.config/githand，可用 $GITHAND_HOME 覆盖）",
	"root.flag.lang":       "输出语言 (en, zh)",

	// scan
	"scan.short":        "扫描目录发现 Git 仓库并注册",
	"scan.flag.recurse": "递归扫描子目录",
	"scan.flag.group":   "按子目录名自动创建分组",
	"scan.none_found":   "未找到 Git 仓库。",
	"scan.result":       "已扫描 %s：发现 %d 个仓库，新增 %d 个。",

	// status
	"status.short":       "显示所有已注册仓库的状态",
	"status.flag.filter": "过滤: dirty, ahead, stash, detached",
	"status.flag.group":  "按分组名过滤",
	"status.flag.owner":  "按远程 URL 所有者过滤",
	"status.flag.json":   "机器可读的 JSON 输出",

	// snapshot
	"snapshot.short":       "快照所有已注册仓库",
	"snapshot.flag.output": "存放带时间戳快照目录的父目录",
	"snapshot.flag.group":  "仅快照此分组中的仓库",
	"snapshot.flag.filter": "仅快照匹配的仓库 (dirty, ahead, stash, detached)",
	"snapshot.written":     "快照已写入 %s（%d 个仓库）",

	// restore
	"restore.short":          "从快照恢复仓库",
	"restore.flag.base-path": "为恢复的仓库重新映射基础路径",
	"restore.flag.dry-run":   "仅显示将要执行的操作，不实际修改",

	// ls
	"ls.short": "列出已注册的仓库名",

	// rm
	"rm.short":   "从注册表中移除仓库",
	"rm.removed": "已从注册表移除 %q。",

	// group
	"group.short":        "管理仓库分组",
	"group.add.short":    "将仓库添加到分组",
	"group.rm.short":     "移除分组",
	"group.ls.short":     "列出所有分组",
	"group.flag.repos":   "要添加的仓库名",
	"group.flag.name":    "分组名",
	"group.added":        "已将 %d 个仓库添加到分组 %q。",
	"group.removed":      "已移除分组 %q。",
	"group.none_defined": "未定义分组。",

	// display
	"display.no_repos": "未注册仓库。",
	"display.header":   "仓库\t分支\t状态\t领先\t落后\t暂存",
	"display.clean":    "干净",
	"display.dirty":    "脏",

	// restore (internal)
	"restore.progress": "正在将 %d 个仓库从 %s 恢复到 %s",
	"restore.dry_run":  "[试运行] 将恢复 %s -> %s",
	"restore.restored": "已恢复 %s",
}
