# Pikapika Core 接口契约（v1.8.21，反编译恢复）

> 本契约从官方二进制反编译 + Dart 层源码提取，用于重写自定义核心。
> 依据：`pikapika/pikapika` 核心二进制（Go 1.24.13，带 DWARF，未 stripped）

## 1. 核心架构

```
libapp.so (Dart) ──MethodChannel("method")──> 核心二进制 (Go)
     │                                            │
     │  flatInvoke(method, params)                ├─ pikapika/pikapika  (分发+业务 ~378符号)
     │                                            ├─ pikapika/pikapika/database (sqlite, 223)
     │                                            ├─ web_server (38)
     │                                            ├─ telemetry (41)
     │                                            ├─ pat (Patreon校验, 11)
     │                                            ├─ pro (兑换码, 5)
     │                                            └─ github.com/niuhuan/pica-go (MIT, pika API)
```

## 2. flatInvoke 方法面（152 个，Dart 调用）

| 类别 | 方法 |
|---|---|
| 属性 | loadProperty, saveProperty |
| 分流 | getSwitchAddress, setSwitchAddress, reloadSwitchAddress, resetSwitchAddress, getImageSwitchAddress, setImageSwitchAddress, getUseApiClientLoadImage, setUseApiClientLoadImage |
| 网络 | getProxy, setProxy, defaultHttpClientGet, ping, pingImg, clientIpSet |
| 用户 | preLogin, login, register, userProfile, getUsername, setUsername, updateAvatar, updateSlogan, updatePassword, getPassword, setPassword, forgotPassword, resetPassword, punchIn, clearToken |
| 漫画列表 | categories, comics, searchComics, comicEpPage, comicPicturePageWithQuality, leaderboard, leaderboardOfKnight, randomComics, recommendation, collections, favouriteComics |
| 评论 | comments, commentChildren, postComment, postChildComment, myComments, switchLikeComment |
| 游戏 | games, game, gameComments, gameCommentChildren, postGameComment, postGameChildComment, switchLikeGameComment, downloadGame |
| 收藏/订阅 | switchFavourite, switchLike, addSubscribed, removeSubscribed, updateSubscribed, updateSubscribedForce, allSubscribed, loadSubscribed, removeAllSubscribed |
| 浏览记录 | viewComic, loadView, storeViewEp, viewLogPage, clearAllViewLog, deleteViewLog, loadViewedList, mergeHistoriesFromLocal, mergeHistoriesFromWebDav |
| 下载 | addDownload, allDownloads, loadDownloadComic, downloadAll, createDownload, deleteDownloadComic, moveDownloadComic, downloadRunning, setDownloadRunning, downloadEpList, downloadPicturesByEpId, downloadImagePath, resetAllDownloads |
| 下载配置 | loadDownloadThreadCount, saveDownloadThreadCount, loadDownloadAndExportPath, saveDownloadAndExportPath, loadDownloadCachePath, saveDownloadCachePath |
| 导出 | exportComicDownload, exportAnyComicDownloadsToPki, exportAnyComicDownloadsToZip, exportComicDownloadJpegZip, exportComicDownloadToCbzsZip, exportComicDownloadToEpub, exportComicDownloadToJPG, exportComicDownloadToPDF, exportComicDownloadToPDFFolder, exportComicDownloadToPki, exportComicDownloadToPkz, exportComicJpegsEvenNotFinish, exportComicUsingSocket, exportComicUsingSocketExit, importComicDownload, importComicDownloadDir, importComicDownloadPki, importComicDownloadUsingSocket, importComicViewFormOff, loadPkzFile, pkzInfo, pkzComicViewLogs, pkzComicViewLogByPkzNameAndId, viewPkz, viewPkzComic, viewPkzEpAndPicture |
| 本地收藏 | allCustomFolders, createLocalFavoriteFolder, updateLocalFavoriteFolder, deleteLocalFavoriteFolder, listLocalFavoriteFolders, countLocalFavoriteFolders, getLocalFavoriteFolder, listLocalFavoriteComics, listAllLocalFavoriteComics, addLocalFavoriteComic, removeLocalFavoriteComic, moveLocalFavoriteComics, mergeLocalFavoritesFromWebDav |
| 图片 | convertImageToJPEG100, remoteImageData, remoteImagePreload |
| 清理 | clean, autoClean, cleanImageCache, cleanNetworkCache |
| 杂项 | appConfig, configLinks, getHomeDir, mkdirs, startWebServer, stopWebServer, setDownloadRunning |
| WebDAV | mergeHistoriesFromWebDav, mergeLocalFavoritesFromWebDav (见上) |
| Pro (可移除) | proInfoAll, reloadPro, inputCdKey, setPatAccessKey, reloadPatAccount, bindThisAccount, clearPat |

> 完整签名见 `contract_methods2.txt`（152 条 Future 签名 + flatInvoke 参数）。
> 核心侧 handler：203 个 `pikapika/pikapika.*` 函数 + 29 个由 FlatInvoke 跳表分发。

## 3. 数据库 Schema（5 个 sqlite）

### comic_center.db
- `comic_views`：浏览记录（id, title, author, pages_count, eps_count, categories, last_view_time, last_view_ep_order...）
- `remote_images`：图片缓存（file_server, path, file_size, format, width, height, local_path）
- `comic_downloads`：下载任务（selected_ep_count, download_picture_count, download_finished, pause, custom_folder...）
- `comic_download_eps` / `comic_download_pictures`：下载明细
- `comic_subscribes`：订阅（subscribe_time, new_ep_count）

### local_favorite.db
- `local_favorite_folders` / `local_favorite_comics`（info 为 JSON 文本）

### network_cache.db / properties.db / pkz_center.db
- 见运行时导出（properties 为 k/v 表）

## 4. 关键属性键（properties.db）

`install_id`, `LAST_LOGIN`, `LAST_LOGIN_PRO`(格式 `{expire}|{md5hex}`), `LAST_ACCESS_KEY`, `CHECK_STR`, `CHECK_SUM`, `CHECK_TIME_SUCCESS_LOCAL`, `CHECK_TIME_FAIL_LOCAL`, `downloadThreadCount`, `downloadAndExportPath`, `downloadCachePath`, `passed`, `window_width`, `window_height`, `full_screen`, `switchAddress` 等

## 5. 服务端端点

- pika API：pica-go 内置（`picaapi.picacomic.com`，签名/加密在公开模块中）
- 图片 CDN：`storage1.picacomic.com`, `storage-b.picacomic.com`, `storage.tipatipa.xyz`
- 作者配置：`https://cdn.comicsparks.work/cfg/pikapikahash/crc32`
- Patreon：`https://pat.comicsparks.work/`
- 分流地址：`https://s2.picacomic.com` 等

## 6. 重写策略

1. **基础**：仓库 git 历史中 MIT 源码 `go/`（2022-04 前，含 cmd/desktop runner + mobile/gomobile 绑定 + pikapika 客户端）
2. **API 客户端**：换用公开的 `github.com/niuhuan/pica-go`（MIT）
3. **数据库层**：按本契约 schema 重建（gorm + sqlite，与官方一致）
4. **下载/导出/浏览记录**：按 MIT 源码结构重写 + 补齐新功能
5. **Pro/pat 模块**：自行实现等价契约（或直接移除，Dart 层已解锁）
6. **构建**：go-flutter(hover) 出 Linux AppImage，gomobile 出 Android .aar，替换官方核心

## 7. 实现状态（2026-08-16）

- ✅ **152/152 flatInvoke 方法全部实现**（commit 9484be1）
- ✅ 数据库层：properties / network_cache / comic_center / local_favorite / pkz_center（schema 与官方一致）
- ✅ 本地收藏 13 / 订阅 7 / 历史同步 5 / 导出家族 24 / pkz 查看 7 / 下载管理 / 配置 / web server / 密码找回 / Pro 空实现
- ✅ 功能测试全通过（导出→导入往返、pkz 加密读写、web server 局域网访问）
- ⬜ 待办：
  - 与真实 pika 账号联调（需网络）：login/viewComic/downloadAll/updateSubscribed
  - 端到端替换：hover 构建 Linux 版、gomobile 构建 Android 版，替换官方核心
  - pkz/pki 与官方 App 的互操作性验证
