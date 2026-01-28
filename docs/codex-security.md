# Codex CLI 安全与沙箱配置指南

面向：内部开发者。本文整理 Codex CLI 的沙箱/审批配置，以及如何启用 `danger-full-access`。

## 风险提示（必读）

- `danger-full-access` 会禁用沙箱限制，权限极高。仅建议在隔离环境中使用（如容器、专用 CI Runner）。
- `--dangerously-bypass-approvals-and-sandbox` / `--yolo` 会同时禁用沙箱与审批提示，风险更高。
- 默认从更安全的模式开始（如 `workspace-write` 或 `read-only`），只在确有需要时临时切换。

## 基本概念

- **sandbox_mode**：决定“技术上能做什么”（文件/网络访问范围）。
- **approval_policy**：决定“什么时候需要你确认”。

常见取值：
- `sandbox_mode`：`read-only` / `workspace-write` / `danger-full-access`
- `approval_policy`：`untrusted` / `on-failure` / `on-request` / `never`

## 临时启用（命令行一次性）

> 适合临时需求，用完即回到安全默认值。

```bash
# 关闭沙箱
codex --sandbox danger-full-access

# 同时关闭审批 + 沙箱（更危险）
codex --dangerously-bypass-approvals-and-sandbox
# 或
codex --yolo

# 仅关闭审批提示（仍受沙箱约束）
codex --ask-for-approval never
```

## 永久配置（config.toml）

配置文件位于用户目录下 `~/.codex/config.toml`（Windows 通常是 `C:\Users\<User>\.codex\config.toml`）。

**示例：全局启用危险模式（不推荐）**

```toml
sandbox_mode = "danger-full-access"
approval_policy = "never"
```

## 使用 Profile（推荐）

将“危险模式”放进 profile，默认保持安全：

```toml
# 默认仍使用更安全的模式
approval_policy = "on-request"
sandbox_mode = "workspace-write"

[profiles.danger]
approval_policy = "never"
sandbox_mode = "danger-full-access"
```

启用方式：

```bash
codex --profile danger
```

> 需要时再切换，不改变默认行为。

## 更安全的替代方案（推荐顺序）

1. `workspace-write` + `on-request`：允许工作区写入，但关键动作仍需确认。
2. `workspace-write` + `untrusted`：仅对不可信命令提示审批。
3. `read-only` + `on-request`：只读检查和咨询场景。

## 实践建议

- 优先在版本库内操作，便于审查和回滚。
- 需要更多写权限时，优先考虑 `--add-dir` 为指定目录开放写入，而不是直接 `danger-full-access`。
- 只在明确知道风险边界时使用 `danger-full-access`。