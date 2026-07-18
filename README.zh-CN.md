# githand

Git 工作区同步与迁移 CLI — scan、status、snapshot、restore。

一条命令把整个 git 工作区搬到新机器，**包括未提交的更改**。

## 特性

- **完整工作区迁移** — 快照每个仓库的状态（远程地址、分支、stash、未提交更改、未跟踪文件）并在另一台机器上复现
- **脏状态保留** — 暂存/未暂存的 diff、stash 条目、未跟踪文件（含二进制）全部捕获并恢复
- **并行状态收集** — 并发收集所有仓库状态（可配置工作线程数）
- **智能分组** — 按子目录名自动分组，或手动管理分组
- **路径可移植** — 快照内部使用相对路径；`--base-path` 在恢复时重新映射根目录，`/Users/you/work` 无缝变为 `/home/me/projects`
- **过滤与查询** — 按脏状态、领先远程、有 stash、分组、所有者等条件筛选仓库
- **JSON 输出** — 机器可读的状态输出，方便脚本集成
- **TOML 配置** — 配置、注册表和分组默认存放在 `~/.config/githand/`（`githand.toml` + `repos.toml`），也可通过 `GITHAND_HOME` 指定目录
- **Cobra CLI** — 基于 spf13/cobra 构建，提供完善的命令行体验

## 安装

```bash
# 克隆后构建
git clone https://github.com/handy-sun/githand.git
cd githand
go build -o bin/githand ./cmd/githand/

# 或用 go install 安装
go install github.com/handy-sun/githand/cmd/githand@latest
```

需要 Go 1.26+ 和 git。CGO 禁用（纯 Go 构建）。

## 快速开始

```bash
# 1. 发现工作区下所有 git 仓库
githand scan ~/work --recursive --auto-group

# 2. 查看所有仓库状态
githand status

# 3. 迁移前快照
githand snapshot -o ~/snapshots

# 4. 在新机器上恢复
githand restore ~/snapshots/githand-snapshot.0515-221241 ~/work --base-path ~/work
```

## 命令

### scan — 发现并注册仓库

```bash
githand scan <path>                    # 扫描目录下的 git 仓库
githand scan <path> -r                 # 递归扫描子目录
githand scan <path> --auto-group       # 按子目录名自动创建分组
```

首次扫描时记录目录为 `base_path`，后续扫描会保留。已在注册表中的仓库会被跳过。

### status — 显示仓库状态

```bash
githand status                         # 显示所有仓库
githand status --sync                  # 自动同步仓库列表（检测新增和删除的仓库）
githand status --filter dirty          # 仅显示有未提交更改的仓库
githand status --filter ahead          # 仅显示领先远程的仓库
githand status --filter stash          # 仅显示有 stash 的仓库
githand status --filter detached       # 仅显示 detached HEAD 的仓库
githand status --group nix             # 仅显示 "nix" 分组中的仓库
githand status --user handy-sun        # 按远程 URL 所有者筛选
githand status --json                  # 机器可读的 JSON 输出
```

**自动同步功能：**

使用 `--sync` 标志或在配置文件中设置 `status.auto_sync = true`，`status` 命令会自动：
- 从注册表中移除已删除的仓库
- 发现并添加 `base_path` 下新增的仓库

这样你就不需要在每次添加或删除仓库后手动运行 `scan` 命令。

**状态符号：**

| 符号 | 含义 |
|------|------|
| `+` | 已暂存的更改 |
| `!` | 未暂存的更改 |
| `?` | 未跟踪的文件 |
| `$` | Stash 条目 |
| `D` | Detached HEAD |
| `clean` | 以上均无 |

**同步指示器：**

| 符号 | 含义 |
|------|------|
| `=` | 与远程同步 |
| `↑` | 本地领先 |
| `↓` | 远程领先 |
| `↕` | 已分叉 |
| `-` | 无远程配置 |

### sync — 拉取远程最新更改

```bash
githand sync                            # 拉取所有已注册仓库的最新代码
githand sync --group nix                # 仅 "nix" 分组
githand sync --remote upstream          # 从非默认远程拉取
```

并行从每个仓库的远程拉取当前分支（工作线程数可配置）。使用 `--autostash` 保留脏工作树，并遵循各仓库的 `pull.rebase` 配置。每个仓库的 git pull 输出会内联打印，仓库名在更新成功时显示为绿色，出错时显示为红色。

### snapshot — 序列化工作区用于迁移

```bash
githand snapshot                       # 快照所有已注册仓库
githand snapshot -o ~/snapshots        # 存放快照输出的父目录
githand snapshot --group nix           # 仅快照 "nix" 分组
githand snapshot --filter dirty        # 仅快照有未提交更改的仓库
githand snapshot --archive             # 需要目录时额外生成 .tar 归档
```

如果快照只需要 JSON 元数据和补丁，会直接生成带时间戳的 JSON 文件：

```
githand-snapshot.0515-221241.json
```

如果包含未跟踪文件或当前 HEAD 有未推送提交，则保持带时间戳的快照目录：

```
githand-snapshot.0515-221241/
  snapshot.json                               # 所有元数据 + 补丁文本
  untracked/
    expnix/
    githand/
  bundles/                                    # 未推送提交的增量 Git bundle
```

使用 `--archive` 时，该目录会额外打包为 `githand-snapshot.0515-221241.tar`。

**每个仓库捕获的内容：**

- 远程 URL
- 所有本地分支，含上游追踪
- 当前分支和 HEAD 提交
- 当前 HEAD 可达的未推送提交（增量 Git bundle）
- `core.hooksPath` 配置（生效值）
- 已暂存 diff（`git diff --cached`）
- 未暂存 diff（`git diff`）
- Stash 条目（每个作为完整补丁）
- 未跟踪文件（复制到快照目录，遵循 `.gitignore`）

### restore — 在新机器上复现工作区

```bash
githand restore <snapshot.json> <target_dir>
githand restore <snapshot.json> <target_dir> --base-path /new/root
githand restore <snapshot.json> <target_dir> --dry-run
```

恢复按顺序回放每个仓库的快照：

1. 从主远程 `git clone`
2. 添加额外的远程
3. 当前 HEAD 有未推送提交时导入增量 Git bundle
4. `git checkout` 到原始分支
5. 恢复 `core.hooksPath` 配置（写入本地配置）
6. 应用暂存补丁（`git apply --cached`）
7. 应用未暂存补丁（`git apply`）
8. 应用 stash 补丁（每个 `git apply --index` + `git stash`）
9. 从快照目录复制未跟踪文件

`--base-path` 将快照的原始根目录映射到新路径，保留相对目录结构。不指定时，`target_dir` 作为基础路径。

### ls、rm — 管理注册表

```bash
githand ls                             # 列出已注册仓库名
githand rm <name>                      # 从注册表中移除仓库
```

### group — 组织仓库

```bash
githand group add <group> <repo...>    # 添加仓库到分组
githand group rm <group>               # 删除分组
githand group ls                       # 列出所有分组
```

## 工作原理

### 路径可移植性

快照存储相对路径而非绝对路径。扫描 `~/work` 时，基础路径 `/Users/you/work` 被记录。快照时，每个仓库的路径相对于此基础计算（如 `nix/expnix`、`agent-switch/cc-switch`）。

恢复时，`--base-path` 设置新的根目录，相对结构被保留：

```
机器 A:  /Users/you/work/nix/expnix
                         ^^^^^^^^^^^^^  相对路径
机器 B:  /home/me/projects/nix/expnix  (--base-path /home/me/projects)
```

这意味着一份快照可在 macOS、Linux 或任何路径布局间通用。

### 为什么注册表不存相对路径？

如果 `base_path` 变化（比如从不同目录重新扫描），存储的相对路径就会失效。在快照时从存储的 `base_path` + 绝对 `path` 动态计算更简单，且始终正确。

## 与 gita 的对比

[gita](https://github.com/nosarthur/gita) 是一个流行的多仓库管理工具。以下是 githand 与它的区别：

| | githand | gita |
|---|---|---|
| **核心定位** | 工作区迁移与快照 | 多仓库可视化和命令派发 |
| **脏状态迁移** | 完整支持（补丁 + 未跟踪文件） | 不支持 — `freeze` 只捕获 URL + 分支 |
| **未提交更改** | 跨机器保留 | freeze/clone 时丢失 |
| **Stash 条目** | 序列化并恢复 | 不捕获 |
| **未跟踪文件** | 复制并恢复 | 不捕获 |
| **跨机器工作流** | `snapshot` → 拷贝快照目录 → `restore` | `freeze` → `clone -f`（仅限干净仓库） |
| **批量 git 命令** | 非核心功能 | 核心功能（`gita super`、`gita shell`） |
| **自定义命令派发** | — | 支持（cmds.json、super、shell） |
| **状态展示** | 每仓库详细视图 | 紧凑并排的 `gita ll` |
| **并行状态收集** | Goroutine 池 | 异步执行 |
| **按状态筛选** | dirty / ahead / stash / detached | 颜色编码展示 |
| **按子目录分组** | 扫描时 `--auto-group` | 添加时 `add -a` |
| **路径可移植** | 相对路径 + `--base-path` | `clone -p` 保留路径 |
| **JSON 输出** | `--json` 标志 | 无 |
| **实现语言** | Go（单二进制） | Python（pip 包） |

**总结：** 如果你需要在一个地方对多个仓库执行 git 命令，用 **gita**。如果你需要把整个工作区——包括所有进行中的工作——搬到新机器，用 **githand**。

## 开发

```bash
cd githand
make build          # 构建
make test           # 测试
make fmt            # 格式化 Go 代码
make fmt-check      # 检查 Go 代码格式
make install-hooks  # 启用提交前格式检查 hook
```

配置默认位于 `~/.config/githand/`，设置 `GITHAND_HOME` 后改用该目录。全局配置文件名为 `githand.toml`；删除 `repos.toml` 可重置注册表。

## 许可证

MIT
