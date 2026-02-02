# Deca - GitHub Release 包管理器

通过 GitHub Release 下载软件包并管理更新，采用声明式配置文件实现状态管理。

## 特性

- 声明式配置 - TOML 格式，简单易用
- 多种包格式 - 支持二进制、tar.gz、.deb/.rpm、AppImage
- 交互式选择 - 可视化选择下载哪个 asset
- 状态跟踪 - 自动记录安装版本和时间
- 跨平台 - Linux、macOS、Windows
- 彩色输出 - 清晰的命令帮助和状态显示

## 安装

```bash
# 从源码构建
git clone https://github.com/deca-org/deca.git
cd deca
make build
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

[settings]
auto_update = true
check_interval = "24h"
```

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
| `deca --version` | 显示版本 |
| `deca --dry-run` | 预览操作而不执行 |
| `deca -v/--verbose` | 详细输出模式 |

## 支持的包格式

| 格式 | 处理方式 |
|------|----------|
| tar.gz/tar.xz/zip | 自动提取二进制 |
| .deb | 通过 apt 安装（需要 sudo） |
| .rpm | 通过 dnf/yum 安装（需要 sudo） |
| AppImage | 直接复制，设置执行权限 |
| 单文件二进制 | 直接复制 |

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
| 二进制 | `~/.local/bin` | `%LOCALAPPDATA%\deca\bin` |

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

## 构建

```bash
make build        # 构建（带版本信息）
make test         # 运行测试
make release      # 跨平台构建
make clean        # 清理
```

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
│   └── init.go       # init 命令
├── internal/
│   ├── config/       # 配置解析
│   ├── github/       # GitHub API
│   ├── download/     # 下载（进度条）
│   ├── install/      # 安装（支持系统包）
│   └── ui/           # UI（颜色、交互选择）
├── Makefile          # 构建脚本
└── README.md
```

## 许可证

MIT
