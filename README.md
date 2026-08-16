# pikapika-core（自定义核心）

基于 MIT 许可的原始核心源码（git 历史 2022-04 前）+ 反编译恢复的接口契约（v1.8.21）重写的自定义核心。

## 现状

- ✅ MIT 基础源码：`pikapika/`（客户端/数据库/下载/导出）+ `cmd/`（go-flutter 桌面 runner）+ `mobile/`（gomobile 绑定）
- ✅ 契约文档：`docs/contract.md`（152 个 flatInvoke 方法 + 5 个 sqlite schema + 属性键 + 端点）
- ✅ 已适配最新 pica-go API（示例：`comics` 新增 author 参数）
- ✅ `go build ./pikapika/...` 通过
- ⬜ 70 个新方法待实现（见 `docs/contract.md` 第 2 节，标注"新增"的部分）

## 构建

```bash
# 桌面版（需 flutter 引擎，hover 自动处理）
go install github.com/go-flutter-desktop/hover@latest
hover build linux-appimage

# Android 绑定
gomobile bind -target android pikapika/mobile
```

## 待实现清单（70 项）

导出（pkz/zip/cbz/epub/pdf）、pkz 查看器、本地收藏、订阅、浏览历史同步（WebDAV）、
下载管理新接口、分流地址同步、web server、view log、Pro/pat（可跳过，Dart 层已解锁）

## 实现策略

1. 按 `docs/contract.md` 的 schema 建表（gorm + sqlite）
2. 每个缺失方法在 `pikapika/` 内新增实现，接入 `FlatInvoke` 分发
3. 用 `go build` + 与官方行为对照验证（同一份 Dart 层）
