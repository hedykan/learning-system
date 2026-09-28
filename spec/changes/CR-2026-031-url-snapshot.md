---
id: CR-2026-031
title: "网址资料：导入时抓取快照"
status: accepted
target_version: v0.1.9
created: 2026-09-28
---

# 变更摘要

`learn source add <网址>` 用 HTTP GET 抓取网页，存成快照后按 HTML 资料处理；之后一律读快照，完全离线。可以按网站地图抓取整个文档站。

## 动机

很多学习资料是在线文档或在线书，例如 DDIA 中文翻译站 `ddia.vonng.com`：服务器直接生成静态 HTML，`sitemap.xml` 列出全部章节，`robots.txt` 允许抓取，内容以 CC-BY-4.0 授权。现在只能让学习者手动另存网页。

## 目标行为

- `learn source add <网址> [--sitemap] [--prefix <路径前缀>]`：
  - 单页：GET 一次，保存 HTML 与抓取时间、最终网址、HTTP 状态、内容哈希。
  - `--sitemap`：读取站点地图（支持索引型 sitemap），只抓与起始网址同域、且在 `--prefix` 下的页面，按站点地图顺序作为资料的章节。
  - 遵守 `robots.txt`；请求之间间隔至少 1 秒；单次抓取页数上限 500；标明 User-Agent。
- 快照交给 HTML 适配器（CR-2026-029）出草稿目录和提取文字。
- 学习中**不**访问网络。要更新时再次运行 `source add`，生成新版本；旧记录定位仍指向旧快照。
- 抓不到正文时（页面需要运行 JavaScript、要求登录、返回错误），报告原因并建议：让 Agent 用它自己的抓取能力取得内容后作为文件交给 `source add`，或由学习者另存为 HTML/PDF。
- 优先原稿：Skill 提示 Agent，若网站注明了 Git 仓库里的 Markdown 原稿（如 DDIA 的 `content/zh/`），建议克隆原稿作为资料。
- 沙箱：Codex 默认禁止联网，`learn` 作为它的子进程同样连不上网。连接失败时输出明确原因和两个办法：在 Vault 的 `.codex/config.toml` 中开启 `[sandbox_workspace_write] network_access = true`，或由学习者在终端自己运行这条命令。Skill 要求 Agent 把这一点告诉学习者，不静默失败。

## 数据与兼容性影响

纯新增。Runtime 首次具备网络访问能力，仅限这条显式命令；“不接入网络模型、不内置 OCR”的原则不变。

## 风险

- 版权：只保存在学习者本地 Vault，不分发；Skill 不建议抓取明确禁止转载的内容。
- 站点被抓崩：限速与页数上限由程序强制。

## 验收条件

1. 用本地测试 HTTP 服务验证单页、索引型 sitemap、前缀过滤、robots 禁止、页数上限。
2. 真实抓取 `ddia.vonng.com` 的中文章节，草稿目录包含序言与第 1–14 章。
3. 无网络时给出沙箱说明而不是笼统报错。
4. 学习过程中断网不影响读取。

## 决策

2026-09-28 接受，排入 v0.1.8；同日随版本拆分移至 v0.1.9。
