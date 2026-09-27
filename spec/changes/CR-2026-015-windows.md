---
id: CR-2026-015
title: "Windows 构建与路径兼容"
status: implemented
target_version: v0.1.5
created: 2026-09-26
---

# 变更摘要

发布目标新增 Windows（amd64、arm64），并审查路径、换行与文件替换逻辑，保证在 Windows 上与 macOS、Linux 行为一致。

## 目标行为

- `make build-all` 产出 `learn-windows-amd64.exe` 与 `learn-windows-arm64.exe`。
- 新增 `make test-windows-build`：为 Windows 交叉编译全部测试并运行 `go vet`。
- 所有写入磁盘的路径使用 `filepath`，Markdown 中的链接统一使用 `/`。
- Conversation、frontmatter 与目录解析兼容 CRLF。
- 文档说明 Windows 上需要 Git 与 PATH 配置。

## 验收条件

1. Windows 交叉编译与测试编译、`go vet` 全部通过。
2. CRLF 格式的 Conversation 与目录文件可以正确解析。
3. 在真实 Windows 机器上运行一次完整流程（待有 Windows 环境时补做）。

## 决策

2026-09-26 接受，进入 v0.1.5。
