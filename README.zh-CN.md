# Deca - GitHub Release 包管理器

通过 GitHub Release 下载软件包并管理更新，采用声明式配置文件实现状态管理。

## 特性

- 声明式配置 - TOML 格式，简单易用
- 多种包格式 - 支持二进制、tar.gz、.deb/.rpm、AppImage、Windows .exe/.msi
- 交互式选择 - 可视化选择下载哪个 asset
- 状态跟踪 - 自动记录安装版本和时间
- 下载缓存 - 避免重复下载相同版本
- 镜像支持 - 多个 GitHub 镜像源可选
- 跨平台 - Linux、macOS、Windows
- 彩色输出 - 清晰的命令帮助和状态显示

## 安装

```bash
# 从源码构建
git clone https://github.com/kusutori/deca.git
cd deca
pixi run build
./deca init  # 初始化配置

# 或全局安装
sudo mv deca /usr/local/bin/
```

## 快速开始

```bash
# 初始化配置（自动检测系统信息）
deca init

# 添加并安装包
deca add eza-community/eza
deca add sharkdp/bat

# 交互式选择 asset
deca add sourcegit-scm/sourcegit --interactive

# 指定特定包格式
deca add sourcegit-scm/sourcegit --asset "*.deb"

# 查看已安装的包
deca list
deca config show

# 检查更新
deca status

# 更新所有包
deca update

# 搜索包
deca search fd

# 使用镜像加速（国内用户）
deca mirror list
deca mirror select

# 查看和管理缓存
deca cache
deca cache list
deca cache clean --orphans
```

## 配置文件

创建 `~/.config/deca/deca.toml`：

```toml
bin_dir = "$HOME/.local/bin"

[packages]
# 简短格式
eza = "eza-community/eza"
bat = "sharkdp/bat"

# 完整格式
zellij = { repo = "zellij-org/zellij", asset = "*.deb" }
neovim = { repo = "neovim/neovim", version = "0.10.0" }

# Windows 交互式安装器
sourcegit = { repo = "sourcegit-scm/sourcegit", asset = "*.exe", install_type = "installer" }

[settings]
auto_update = true
check_interval = "24h"
```

## 命令

## 命令

| 命令 | 说明 |
|------|------|
| `deca init` | 初始化配置，检测系统信息 |
| `deca apply` | 应用配置，安装/更新所有包 |
| `deca add <owner/repo>` | 添加并安装包 |
| `deca add <owner/repo> -i` | 交互式选择 asset |
| `deca add <owner/repo> --asset "*.deb"` | 指定 asset 模式 |
| `deca add <owner/repo> --no-install` | 仅添加配置 |
| `deca remove <name>` | 卸载并从配置移除包 |
| `deca remove <name> -k` | 仅从配置移除，保留安装 |
| `deca list` | 列出包和状态 |
| `deca config show` | 显示配置和安装状态 |
| `deca config diff` | 显示配置差异 |
| `deca config` | 编辑配置文件 |
| `deca status` | 检查更新 |
| `deca update [name]` | 更新包 |
| `deca search <query>` | 搜索 GitHub |
| `deca doctor` | 健康检查 |
| `deca cache` | 显示缓存状态 |
| `deca cache list` | 列出缓存文件 |
| `deca cache size` | 显示缓存大小 |
| `deca cache clean` | 清理缓存 |
| `deca schema` | 生成 `deca.toml` 的 JSON Schema |
| `deca mirror` | 显示当前镜像源 |
| `deca mirror list` | 列出可用镜像源 |
| `deca mirror select` | 交互式选择镜像源 |
| `deca mirror add <name> <url>` | 添加自定义镜像源 |
| `deca mirror remove <name>` | 移除自定义镜像源 |
| `deca --version` | 显示版本 |
| `deca --dry-run` | 预览操作而不执行 |
| `deca -v/--verbose` | 详细输出模式 |

## Shell 补全

通过 `deca completion <shell>` 生成补全脚本（基于 Carapace），并按以下方式集成：

### Bash
```bash
deca completion bash > /etc/bash_completion.d/deca
```

### Zsh
```bash
deca completion zsh > "${fpath[1]}/_deca"
```

### Fish
```bash
deca completion fish > ~/.config/fish/completions/deca.fish
```

### PowerShell
```powershell
deca completion powershell | Out-String | Invoke-Expression
```

### Nushell
```nu
deca completion nushell | save -f ~/.config/nushell/completions/deca.nu
```
并在 `config.nu` 中添加：
```nu
source ~/.config/nushell/completions/deca.nu
```

## 支持的包格式

| 格式 | 处理方式 |
|------|----------|
| tar.gz/tar.xz/zip | 自动提取二进制 |
| .deb | 通过 apt 安装（需要 sudo） |
| .rpm | 通过 dnf/yum 安装（需要 sudo） |
| AppImage | 直接复制，设置执行权限 |
| 单文件二进制 | 直接复制 |
| Windows 便携 .exe/zip | 安装到 `%LOCALAPPDATA%\deca\packages\<name>\<version>`，并通过 `%LOCALAPPDATA%\deca\bin` 暴露到 PATH |
| Windows .msi | 使用 `msiexec /qn /norestart` 安装和卸载 |
| Windows 安装器 .exe | 启动 GUI 安装器并等待结束；卸载仍需用户手动完成 |

Windows 下 `install_type` 可选 `auto`、`portable`、`msi`、`installer`。默认 `auto` 会把 `.msi` 当作 MSI 包，把直接的 `.exe` 当作便携可执行文件。传统 GUI 安装器请显式设置 `install_type = "installer"`。

## 下载缓存

下载的文件会缓存到 `~/.cache/deca`，避免重复下载相同版本：

```bash
# 查看缓存状态
deca cache

# 列出缓存文件
deca cache list

# 显示缓存大小
deca cache size

# 清理未使用的缓存
deca cache clean --orphans

# 清理所有缓存
deca cache clean --all
```

## 镜像源

国内用户可以使用镜像源加速下载：

```bash
# 列出所有可用镜像
deca mirror list

# 交互式选择镜像
deca mirror select

# 查看当前镜像
deca mirror

# 添加自定义镜像
deca mirror add "My Mirror" https://mirror.example.com
```

可用镜像：
- GitHub (Official) - 官方源
- GitHub Fast (China) - ghfast.top
- GitHub Proxy (China) - github.moeyy.xyz
- Jihulab (China) - jihulab.com
- FastGit (China) - fastgit.org

## 状态管理

安装状态保存在 `~/.local/state/deca/state.json`：

```json
{
  "packages": {
    "eza": {
      "repo": "eza-community/eza",
      "version": "v0.18.0",
      "asset_name": "eza-x86_64-unknown-linux-musl.tar.gz",
      "installed_at": "2024-01-15T10:30:00Z"
    }
  }
}
```

## 路径说明

| 类型 | Linux/macOS | Windows |
|------|-------------|---------|
| 配置 | `~/.config/deca/deca.toml` | `%APPDATA%\deca\deca.toml` |
| 状态 | `~/.local/state/deca/state.json` | `%LOCALAPPDATA%\deca\state.json` |
| 缓存 | `~/.cache/deca` | `%LOCALAPPDATA%\deca\cache` |
| 二进制 | `~/.local/bin` | `%LOCALAPPDATA%\deca\bin` |
| Windows 包根目录 | 不适用 | `%LOCALAPPDATA%\deca\packages` |
| 镜像配置 | `~/.config/deca/mirrors.toml` | `%APPDATA%\deca\mirrors.toml` |

## 全局选项

| 选项 | 说明 |
|------|------|
| `--config <path>` | 指定配置文件路径 |
| `--dry-run` | 预览操作而不执行 |
| `-v, --verbose` | 详细输出模式 |

## GitHub Token

对于私有仓库或提高 API 限制：

```bash
export GITHUB_TOKEN="your_token_here"
```

## Python 绑定（gopy）

项目现在提供了用于 gopy 的 Go API 包：`github.com/kusutori/deca/pkg/pydeca`。

### 1) 安装生成工具

```bash
just setup-python-bindings
```

### 2) 生成 Python 包

```bash
pixi run python-bindings
python3 -m pip install -e bindings/python
```

### 3) 在 Python 中调用

```python
from deca_py import pydeca

# 直接调用导出函数
schema_json = pydeca.GenerateSchemaJSON()
print(schema_json[:80])

# 使用 Client 调用 CLI 能力
c = pydeca.NewDefaultClient()
code = c.Run(["status"])
print("exit:", code)
```

可用 API（节选）：
- `pydeca.GenerateSchemaJSON()`
- `pydeca.WriteSchema(path)`
- `pydeca.LoadConfigJSON(path)`
- `pydeca.ListPackageNames(path)`
- `pydeca.InjectSchema(config_path, schema_path)`
- `pydeca.Client.Run(args)`

## 构建

```bash
pixi run build          # 构建（带版本信息）
pixi run test           # 运行测试
pixi run release        # 跨平台构建（动态链接）
pixi run static         # 静态链接构建（兼容旧系统）
pixi run musl-release   # musl 静态构建（推荐用于 Linux）
pixi run clean          # 清理
```

### glibc 兼容性

默认构建依赖 glibc 2.32+，可能无法在旧系统（如 Ubuntu 18.04、CentOS 7）上运行。

**解决方案：**

使用 musl 静态构建（推荐）：

```bash
# 安装 musl 工具链
sudo apt install musl-tools

# 构建静态链接版本
pixi run static
```

生成的静态二进制可以在几乎所有 Linux 发行版上运行，包括：
- Ubuntu 14.04+
- CentOS 7+
- Debian 9+
- Alpine Linux

## 项目结构

```
deca/
├── cmd/              # CLI 命令
│   ├── root.go       # 入口、持久 flags
│   ├── apply.go      # apply 命令
│   ├── add.go        # add 命令（交互式选择）
│   ├── remove.go     # remove 命令
│   ├── list.go       # list 命令
│   ├── status.go     # status 命令
│   ├── update.go     # update 命令
│   ├── search.go     # search 命令
│   ├── doctor.go     # doctor 命令
│   ├── config.go     # config 子命令
│   ├── init.go       # init 命令
│   ├── cache.go      # cache 命令
│   └── mirror.go     # mirror 命令
├── internal/
│   ├── config/       # 配置解析
│   ├── github/       # GitHub API
│   ├── download/     # 下载（进度条、缓存）
│   ├── install/      # 安装（支持系统包）
│   ├── cache/        # 缓存管理
│   └── ui/           # UI（颜色、交互选择）
├── pixi.toml          # 构建脚本
└── README.md
```

## 许可证

MIT

## 测试覆盖率提升两周计划

以下计划用于系统性提升 `deca` 的测试覆盖率与稳定性：

1. **第 1-2 天**：完成覆盖率热区图和可测试性改造清单。
2. **第 3-7 天**：主攻 `internal/` 核心包（每天跟踪单包覆盖率变化）。
3. **第 8-10 天**：主攻 `cmd/` 层与错误路径补测。
4. **第 11 天**：补齐遗漏分支与 flaky 测试治理。
5. **第 12 天**：接入 CI gate + PR 模板更新。
6. **第 13-14 天**：回归与文档收尾（记录测试策略、mock 规范、常见陷阱）。

建议执行配套：

- 每日固定产出覆盖率快照（`go test ./... -coverprofile=cover.out`）。
- 每个阶段结束后更新 `README` 或独立测试文档，记录关键决策与风险。
- 对 flaky 测试建立最小复现与隔离策略，避免 CI 偶发失败扩散。
