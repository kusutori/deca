# Deca - GitHub Release 包管理器

通过 GitHub Release 下载软件包并管理更新，采用声明式配置文件实现状态管理。

## 安装

```bash
# 从源码构建
git clone https://github.com/deca-org/deca.git
cd deca
go build -o deca .
sudo mv deca /usr/local/bin/
```

## 使用方法

### 配置文件

创建 `~/.config/deca/deca.toml`：

```toml
bin_dir = "$HOME/.local/bin"

[packages]
# 简短格式：repo = "owner/repo"
eza = "eza-community/eza"
bat = "sharkdp/bat"

# 完整格式：指定 asset、版本等
zellij = { repo = "zellij-org/zellij", asset = "zellij.*x86_64" }
neovim = { repo = "neovim/neovim", version = "0.9.5" }
stern = { repo = "wercker/stern", os = "linux", arch = "amd64" }

[settings]
auto_update = true
check_interval = "24h"
```

### 命令

```bash
# 应用配置，安装/更新所有包
deca apply

# 添加新包（自动安装）
deca add owner/repo

# 添加新包（不安装）
deca add owner/repo --no-install

# 移除包
deca remove package_name

# 列出已配置的包
deca list

# 检查更新
deca status

# 更新所有包
deca update

# 更新指定包
deca update package_name

# 搜索包
deca search fd

# 健康检查
deca doctor
```

### 命令行参数

| 参数 | 说明 |
|------|------|
| `--config` | 配置文件路径 |
| `--dry-run` | 预览不执行 |
| `-v, --verbose` | 详细输出 |

### add 命令参数

| 参数 | 说明 |
|------|------|
| `-n, --name` | 包名（默认使用仓库名） |
| `--asset` | Asset 匹配模式 |
| `--os` | 目标操作系统 |
| `--arch` | 目标架构 |
| `--no-install` | 仅添加配置，不安装 |

## Asset 匹配

支持 glob 模式自动选择：

- `zellij.*x86_64` → 匹配 `zellij-x86_64-unknown-linux-musl.tar.gz`
- `*.tar.gz` → 匹配任意 tar.gz

自动检测 OS/Arch 过滤。

## 状态管理

安装状态保存在 `~/.local/state/deca/state.json`：

```json
{
  "packages": {
    "eza": {
      "repo": "eza-community/eza",
      "version": "0.18.0",
      "installed_at": "2024-01-15T10:30:00Z"
    }
  }
}
```

## 跨平台支持

| OS | bin_dir 默认值 |
|----|----------------|
| Linux | `$HOME/.local/bin` |
| macOS | `$HOME/.local/bin` |
| Windows | `%LOCALAPPDATA%\deca\bin` |

## GitHub Token

对于私有仓库或提高 API 限制，设置环境变量：

```bash
export GITHUB_TOKEN="your_token_here"
```

## 项目结构

```
deca/
├── cmd/              # CLI 命令
│   ├── root.go       # 入口
│   ├── apply.go      # apply 命令
│   ├── add.go        # add 命令
│   ├── remove.go     # remove 命令
│   ├── list.go       # list 命令
│   ├── status.go     # status 命令
│   ├── update.go     # update 命令
│   ├── search.go     # search 命令
│   └── doctor.go     # doctor 命令
├── internal/
│   ├── config/       # 配置解析
│   ├── github/       # GitHub API
│   ├── download/     # 下载和提取
│   └── install/      # 安装逻辑
├── main.go
├── go.mod
└── README.md
```

## 许可证

MIT
