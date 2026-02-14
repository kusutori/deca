# Nushell 补全集成指南

本文档详细说明如何将基于 carapace 的补全正确集成到 Nushell 中，基于我们为 deca 实现补全功能的实践经验。

## 问题背景

在使用 carapace 实现 shell 补全时，与 Nushell 的集成需要特定的配置，这些配置并不是显而易见的。仅仅实现 `_carapace` 子命令并生成补全脚本是不够的。

## Nushell 补全机制详解

### 1. Extern 声明（单独使用不够）

Nushell 中的 `extern` 关键字用于声明外部命令的签名：

```nushell
extern "deca" [
  ...args: string@"nu-complete deca"
]
```

**重要**：这个声明本身**不会触发补全**。它只是告诉 Nushell 这个命令的签名。

### 2. External Completer（必需）

Nushell 使用 **external completer**（外部补全器），需要在 `$env.config.completions.external.completer` 中配置。这是一个闭包函数，它：
- 接收当前命令行作为输入（`spans`）
- 返回 JSON 格式的补全建议
- 在没有找到内置 Nushell 补全时运行

## 集成步骤

### 步骤 1：实现 `_carapace` 子命令

你的 CLI 工具应该支持 `_carapace` 子命令，输出 carapace 的 JSON 格式补全：

```bash
$ deca _carapace nushell deca ""
[{"value":"add ","display":"add","description":"添加一个包"}]
```

### 步骤 2：配置 External Completer

在 `~/.config/nushell/config.nu` 中配置 external completer：

```nushell
$env.config.completions = {
    case_sensitive: false
    quick: true
    partial: true
    algorithm: "fuzzy"
    external: {
        enable: true
        max_results: 100
        completer: {|spans|
            let cmd = $spans.0
            # 支持内置 _carapace 的命令列表
            # 开发新的 CLI 工具时，在这里添加命令名即可
            let carapace_native_cmds = ["deca"]

            if ($cmd in $carapace_native_cmds) {
                # 使用命令内置的 carapace 支持
                try {
                    ^$cmd _carapace nushell ...$spans | from json
                } catch {
                    null
                }
            } else {
                # 其他命令使用 carapace
                try {
                    carapace $cmd nushell ...$spans | from json
                } catch {
                    null
                }
            }
        }
    }
}
```

**可扩展性**：如果你开发了多个支持 `_carapace` 的 CLI 工具，只需将它们添加到 `carapace_native_cmds` 列表中，而不需要写多个 `if` 判断。这样配置保持简洁且易于维护。

### 步骤 3：无需单独的补全文件

配置好 external completer 后，你**不需要** source 单独的补全文件（如 `deca.nu`）。external completer 会处理一切。

## 常见陷阱

### 1. 模块名冲突（Nushell 0.110+）

**问题**：在名为 `deca.nu` 的文件中使用 `export extern "deca"` 会导致模块名冲突。

**解决方案**：不使用 `export` 关键字，或者完全不使用单独的补全文件（改用 external completer）。

### 2. 在 Carapace 中使用 `c.Args` 而非 `c.Value`

**问题**：在 carapace 的 `ActionCallback` 中，`c.Args` 包含已完成的参数，而不是当前正在输入的内容。

**解决方案**：使用 `c.Value` 获取当前正在输入的内容：

```go
carapace.ActionCallback(func(c carapace.Context) carapace.Action {
    query := c.Value  // 当前输入，而不是 c.Args[0]
    // ...
})
```

### 3. 缺少搜索过滤器

**问题**：没有过滤器的 GitHub API 搜索会返回空结果。

**解决方案**：在搜索查询中添加适当的过滤器：

```go
repos, err := ghClient.SearchRepositories(ctx, query+" has:releases sort:stars")
```

### 4. External Completer 为空

**问题**：设置 `external.completer: null` 或将其注释掉会禁用所有外部补全。

**解决方案**：始终配置一个正确的 external completer 闭包，即使它只是为不支持的命令返回 `null`。

## 测试补全

### 在 Bash 中测试（快速验证）

```bash
bash -c 'source <(deca completion bash) && export COMP_LINE="deca " COMP_POINT=5 && deca _carapace bash deca ""'
```

### 在 Nushell 中测试

```nushell
# 重新加载配置
source ~/.config/nushell/config.nu

# 测试补全
deca <TAB>          # 应该显示子命令
deca add vi<TAB>    # 输入 2+ 字符后搜索 GitHub
deca remove <TAB>   # 应该显示已安装的包
```

### 调试 External Completer

```nushell
# 检查 external completer 是否已配置
$env.config.completions.external.completer

# 直接测试 carapace
deca _carapace nushell deca "" | from json
```

## 架构总结

```
用户按下 TAB
    ↓
Nushell 检查内置补全
    ↓
未找到内置补全 → 调用 external completer
    ↓
External completer 检查命令名
    ↓
如果是 "deca" → 调用: deca _carapace nushell ...
如果是其他命令 → 调用: carapace <cmd> nushell ...
    ↓
返回 JSON 格式的补全数据
    ↓
Nushell 显示补全建议
```

## 这种方法的优势

1. **无需单独的补全文件** - 一切由 external completer 处理
2. **与 carapace 生态系统集成** - 其他支持 carapace 的工具自动工作
3. **回退支持** - 不支持 carapace 的命令回退到文件补全
4. **单一数据源** - 补全逻辑存在于你的 CLI 工具中，而不是分散在各个 shell 特定的文件中
5. **易于扩展** - 开发新工具时只需在列表中添加命令名

## 调试过程记录

### 问题 1：按 TAB 没有任何补全

**原因**：Nushell 的 `extern` 声明不会自动触发补全，必须配置 external completer。

**解决**：在 `$env.config.completions.external.completer` 中配置闭包函数。

### 问题 2：补全返回空结果

**原因**：
1. 在 carapace 回调中错误使用了 `c.Args[0]` 而不是 `c.Value`
2. GitHub 搜索缺少 `" has:releases sort:stars"` 过滤器

**解决**：
1. 使用 `c.Value` 获取当前输入
2. 添加搜索过滤器

### 问题 3：Nushell 0.110+ 模块名冲突

**原因**：在 `deca.nu` 文件中使用 `export extern "deca"` 导致模块名与 extern 名称冲突。

**解决**：移除 `export` 关键字，或完全不使用单独的补全文件。

### 问题 4：deca 补全工作但其他命令不工作

**原因**：External completer 只处理 deca，其他命令返回 `null`。

**解决**：修改 external completer，让它对 deca 使用内置 `_carapace`，对其他命令使用 carapace。

### 问题 5：多个 CLI 工具需要多个 if 判断

**原因**：每个工具一个 `if ($cmd == "xxx")` 判断，不易维护。

**解决**：使用列表 `carapace_native_cmds`，只需在列表中添加命令名即可。

## 参考资料

- [Nushell 自定义补全](https://www.nushell.sh/book/custom_completions.html)
- [Carapace 文档](https://carapace-sh.github.io/carapace/)
- [Nushell External Completers](https://www.nushell.sh/cookbook/external_completers.html)
