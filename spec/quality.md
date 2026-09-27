# 质量属性

## 数据安全

- Conversation append-only。
- 初始化和导入不静默覆盖文件。
- 模型失败不改变既有 Learner/Curriculum State。
- 关键 JSON、YAML 和 Markdown 使用同目录临时文件后原子 rename。
- API 凭据不写入 Vault。

## 可移植性

- 单 Go 二进制，目标为 macOS、Linux、Windows 的 arm64/amd64；不依赖任何单一平台的系统框架。
- 默认不依赖外部数据库或服务。
- 路径通过标准库处理，不假设 `/` 或固定用户目录。
- Vault 内由 Runtime 生成的引用使用相对路径。

## 可测试性

- 核心逻辑与 Cobra 命令层分离。
- 文件系统测试使用临时目录。
- Git 测试使用本地临时仓库，不访问网络。
- 模型相关测试使用 FakeProvider。
- 核心包覆盖率目标不低于 80%，整体目标不低于 70%。

## 可恢复性

- Git 提交失败时已落盘数据仍可用，并可再次提交。
- active session 持久化，中断后可恢复。
- Source 通过内容哈希识别，外部链接失效可诊断。

## 性能

常用状态查询不扫描教材正文；索引和元数据保存在 `.learning`。v0.1 不为大规模语义检索引入数据库。
