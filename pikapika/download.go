package pikapika

import (
	"bytes"
	"fmt"
	"image"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	comic_center2 "pikapika/pikapika/database/comic_center"
	utils2 "pikapika/pikapika/utils"
	"sync"
	"time"
)

// 使用协程进行后台下载
// downloadRunning 如果为false则停止下载
// downloadRestart 为true则取消从新启动下载功能

var downloadThreadCount = 1
var downloadComicCount = 1
var downloadThreadFetch = 100

var downloadRunning = false
var downloadRestart = false

var dlFlag = true

// 每个 worker 独立持有的下载状态 (漫画级并发)
type downloadWorker struct {
	comic *comic_center2.ComicDownload
	ep    *comic_center2.ComicDownloadEp
}

// 共享的待下载列表: 多个 worker 从这里认领不同的漫画
var downloadClaimMutex sync.Mutex
var pendingComics []comic_center2.ComicDownload
var pendingIndex = 0

// 程序启动后仅调用一次, 启动后台主管
func downloadBackground() {
	println("后台线程启动")
	if dlFlag {
		dlFlag = false
		go downloadSupervisor()
	}
}

// 主管: 按 downloadComicCount 维护多个并发下载漫画的 worker
func downloadSupervisor() {
	for {
		time.Sleep(time.Second * 3)
		count := downloadComicCount
		if count < 1 {
			count = 1
		}
		if count > 8 {
			count = 8
		}
		var wg sync.WaitGroup
		for i := 0; i < count; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				(&downloadWorker{}).loop()
			}()
		}
		wg.Wait()
	}
}

// worker 主循环: 认领漫画 -> 处理 -> 回到循环
func (w *downloadWorker) loop() {
	for {
		w.loadComic()
	}
}

// 下载周期中, 每个下载单元会调用此方法, 如果返回true应该停止当前动作
func downloadHasStop() bool {
	if !downloadRunning {
		return true
	}
	if downloadRestart {
		downloadRestart = false
		return true
	}
	return false
}

// worker 从共享列表认领下一本未下载的漫画
func (w *downloadWorker) claimComic() *comic_center2.ComicDownload {
	downloadClaimMutex.Lock()
	defer downloadClaimMutex.Unlock()
	for pendingIndex >= len(pendingComics) {
		list, err := comic_center2.AllNeedDownload()
		if err != nil || len(list) == 0 {
			pendingComics = nil
			pendingIndex = 0
			return nil
		}
		pendingComics = list
		pendingIndex = 0
	}
	c := pendingComics[pendingIndex]
	pendingIndex++
	return &c
}

// 删除下载任务, 当用户要删除下载的时候, 他会被加入删除队列, 而不是直接被删除, 以减少出错
func downloadDelete() bool {
	c, e := comic_center2.DeletingComic()
	if e != nil {
		panic(e)
	}
	if c != nil {
		os.RemoveAll(downloadPath(c.ID))
		e = comic_center2.TrueDelete(c.ID)
		if e != nil {
			panic(e)
		}
		return true
	}
	return false
}

// 加载并处理下一本需要下载的漫画 (worker 方法)
func (w *downloadWorker) loadComic() {
	// 每次下载完一个漫画, 或者启动的时候, 首先进行删除任务
	for downloadDelete() {
	}
	// 检测是否需要停止 (停止时稍等后重新进入)
	if downloadHasStop() {
		time.Sleep(time.Second * 3)
		return
	}
	// 认领下一本待下载漫画
	w.comic = w.claimComic()
	if w.comic == nil {
		// 没有任务, 稍等后重试
		time.Sleep(time.Second * 3)
		return
	}
	w.initComic()
}

// 初始化找到的下载任务 (worker 方法)
func (w *downloadWorker) initComic() {
	// 检测是否需要停止
	if downloadHasStop() {
		time.Sleep(time.Second * 3)
		return
	}
	// 若没有漫画要下载则重新启动
	if w.comic == nil {
		println("没有找到要下载的漫画")
		time.Sleep(time.Second * 3)
		return
	}
	// 打印日志, 并向前端的eventChannel发送下载信息
	println("正在下载漫画 " + w.comic.Title)
	downloadComicEventSend(w.comic)
	eps, err := comic_center2.ListDownloadEpByComicId(w.comic.ID)
	if err != nil {
		panic(err)
	}
	// 找到这个漫画需要下载的EP, 并搜索获取图片地址
	for _, ep := range eps {
		// FetchedPictures字段标志着这个章节的图片地址有没有获取过, 如果没有获取过就重新获取
		if !ep.FetchedPictures {
			println("正在获取章节的图片 " + w.comic.Title + " " + ep.Title)
			// 搜索图片地址, 如果五次没有请求成功, 就不在请求
			for i := 0; i < 5; i++ {
				if client.Token == "" {
					continue
				}
				err := w.fetchPictures(&ep)
				if err != nil {
					println(err.Error())
					continue
				}
				ep.FetchedPictures = true
				break
			}
			// 如果未能获取图片地址, 则直接置为失败
			if !ep.FetchedPictures {
				println("章节的图片获取失败 " + w.comic.Title + " " + ep.Title)
				err = comic_center2.EpFailed(ep.ID)
				if err != nil {
					panic(err)
				}
			} else {
				println("章节的图片获取成功 " + w.comic.Title + " " + ep.Title)
				w.comic.SelectedPictureCount = w.comic.SelectedPictureCount + ep.SelectedPictureCount
				downloadComicEventSend(w.comic)
			}
		}
	}
	// 获取图片地址结束, 去初始化下载的章节
	w.loadEp()
}

// 获取图片地址
func (w *downloadWorker) fetchPictures(downloadEp *comic_center2.ComicDownloadEp) error {
	var list []comic_center2.ComicDownloadPicture
	// 官方的图片只能分页获取, 从第1页开始获取, 每页最多40张图片
	page := 1
	for true {
		rsp, err := client.ComicPicturePage(w.comic.ID, int(downloadEp.EpOrder), page)
		if err != nil {
			return err
		}
		for _, doc := range rsp.Docs {
			list = append(list, comic_center2.ComicDownloadPicture{
				ID:           doc.Id,
				ComicId:      downloadEp.ComicId,
				EpId:         downloadEp.ID,
				EpOrder:      downloadEp.EpOrder,
				OriginalName: doc.Media.OriginalName,
				FileServer:   doc.Media.FileServer,
				Path:         doc.Media.Path,
			})
		}
		// 如果不是最后一页, 页码加1, 获取下一页
		if rsp.Page < rsp.Pages {
			page++
			continue
		}
		break
	}
	// 保存获取到的图片
	err := comic_center2.FetchPictures(downloadEp.ComicId, downloadEp.ID, &list)
	if err != nil {
		panic(err)
	}
	downloadEp.SelectedPictureCount = int32(len(list))
	return err
}

// 初始化下载 (worker 方法)
func (w *downloadWorker) loadEp() {
	// 周期停止检测
	if downloadHasStop() {
		time.Sleep(time.Second * 3)
		return
	}
	// 找到第一个需要下载的章节并去处理 （未下载失败的, 且未完成下载的）
	var err error
	w.ep, err = comic_center2.LoadFirstNeedDownloadEp(w.comic.ID)
	if err != nil {
		panic(err)
	}
	w.initEp()
}

// 处理需要下载的EP
func (w *downloadWorker) initEp() {
	if w.ep == nil {
		// 所有Ep都下完了, 汇总Download下载情况
		w.summaryDownload()
		return
	}
	// 没有下载完则去下载图片
	println("正在下载章节 " + w.ep.Title)
	w.loadPicture()
}

// EP下载汇总
func (w *downloadWorker) summaryDownload() {
	// 暂停检测
	if downloadHasStop() {
		time.Sleep(time.Second * 3)
		return
	}
	// 加载这个漫画的所有EP
	list, err := comic_center2.ListDownloadEpByComicId(w.comic.ID)
	if err != nil {
		panic(err)
	}
	// 判断所有章节是否下载完成
	over := true
	for _, downloadEp := range list {
		over = over && downloadEp.DownloadFinished
	}
	if over {
		// 如果所有章节下载完成则下载成功
		downloadAndExportLogo(w.comic)
		err = comic_center2.DownloadSuccess(w.comic.ID)
		if err != nil {
			panic(err)
		}
		w.comic.DownloadFinished = true
		w.comic.DownloadFinishedTime = time.Now()
	} else {
		// 否则下载失败
		err = comic_center2.DownloadFailed(w.comic.ID)
		if err != nil {
			panic(err)
		}
		w.comic.DownloadFailed = true
	}
	// 向前端发送下载状态
	downloadComicEventSend(w.comic)
	// 去下载下一个漫画
}

// 加载需要下载的图片
func (w *downloadWorker) loadPicture() {
	// 暂停检测
	if downloadHasStop() {
		time.Sleep(time.Second * 3)
		return
	}
	// 获取到这个章节需要下载的图片
	downloadingPictures, err := comic_center2.LoadNeedDownloadPictures(w.ep.ID, downloadThreadFetch)
	if err != nil {
		panic(err)
	}
	// 如果不需要下载
	if len(*downloadingPictures) == 0 {
		// 所有图片都下完了, 汇总EP下载情况
		w.summaryEp()
		return
	}
	// 线程池
	channel := make(chan int, downloadThreadCount)
	defer close(channel)
	wg := sync.WaitGroup{}
	for i := 0; i < len(*downloadingPictures); i++ {
		// 暂停检测
		if downloadHasStop() {
			wg.Wait()
			time.Sleep(time.Second * 3)
			return
		}
		channel <- 0
		wg.Add(1)
		// 不放入携程, 防止i已经变化
		picPoint := &((*downloadingPictures)[i])
		go func() {
			w.downloadPicture(picPoint)
			<-channel
			wg.Done()
		}()
	}
	wg.Wait()
	// 再次新一轮的下载, 直至 len(*downloadingPictures) == 0
	w.loadPicture()
}

var downloadEventChannelMutex = sync.Mutex{}

// 这里不能使用暂停检测, 多次检测会导致问题
func (w *downloadWorker) downloadPicture(downloadingPicture *comic_center2.ComicDownloadPicture) {
	// 下载图片, 最多重试5次
	println("正在下载图片 " + fmt.Sprintf("%d", downloadingPicture.RankInEp))
	for i := 0; i < 5; i++ {
		err := w.downloadThePicture(downloadingPicture)
		if err != nil {
			continue
		}
		func() {
			downloadEventChannelMutex.Lock()
			defer downloadEventChannelMutex.Unlock()
			// 对下载的漫画临时变量热更新并通知前端
			downloadingPicture.DownloadFinished = true
			w.ep.DownloadPictureCount = w.ep.DownloadPictureCount + 1
			w.comic.DownloadPictureCount = w.comic.DownloadPictureCount + 1
			downloadComicEventSend(w.comic)
		}()
		break
	}
	// 没能下载成功, 图片置为下载失败
	if !downloadingPicture.DownloadFinished {
		err := comic_center2.PictureFailed(downloadingPicture.ID)
		if err != nil {
			// ??? panic X channel ???
			// panic(err)
		}
	}
}

// 下载指定图片
func (w *downloadWorker) downloadThePicture(picturePoint *comic_center2.ComicDownloadPicture) error {
	// 为了不和页面前端浏览的数据冲突, 使用url做hash锁
	lock := utils2.HashLock(fmt.Sprintf("%s$%s", picturePoint.FileServer, picturePoint.Path))
	lock.Lock()
	defer lock.Unlock()
	// 图片保存位置使用相对路径储存, 使用绝对路径操作
	picturePath := fmt.Sprintf("%s/%d/%d", picturePoint.ComicId, picturePoint.EpOrder, picturePoint.RankInEp)
	realPath := downloadPath(picturePath)
	// 从缓存获取图片
	buff, img, format, err := decodeFromCache(picturePoint.FileServer, picturePoint.Path)
	if err != nil {
		// 若缓存不存在, 则从网络获取
		buff, img, format, err = decodeFromUrl(picturePoint.FileServer, picturePoint.Path)
	}
	if err != nil {
		return err
	}
	// 将图片保存到文件
	dir := filepath.Dir(realPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.Mkdir(dir, utils2.CreateDirMode)
	}
	err = ioutil.WriteFile(downloadPath(picturePath), buff, utils2.CreateFileMode)
	if err != nil {
		return err
	}
	// 下载时同时导出
	downloadAndExport(w.comic, w.ep, picturePoint, buff, format)
	// 存入数据库
	return comic_center2.PictureSuccess(
		picturePoint.ComicId,
		picturePoint.EpId,
		picturePoint.ID,
		int64(len(buff)),
		format,
		int32(img.Bounds().Dx()),
		int32(img.Bounds().Dy()),
		picturePath,
	)
}

// EP 下载内容汇总
func (w *downloadWorker) summaryEp() {
	// 暂停检测
	if downloadHasStop() {
		time.Sleep(time.Second * 3)
		return
	}
	// 找到所有下载的图片
	list, err := comic_center2.ListDownloadPictureByEpId(w.ep.ID)
	if err != nil {
		panic(err)
	}
	// 全部下载完成置为成功, 否则置为失败
	over := true
	for _, downloadPicture := range list {
		over = over && downloadPicture.DownloadFinished
	}
	if over {
		err = comic_center2.EpSuccess(w.ep.ComicId, w.ep.ID)
		if err != nil {
			panic(err)
		}
	} else {
		err = comic_center2.EpFailed(w.ep.ID)
		if err != nil {
			panic(err)
		}
	}
	// 去加载下一个EP
	w.loadEp()
}

// 边下载边导出(导出路径)
var downloadAndExportPath = ""

// 边下载边导出(导出图片)
func downloadAndExport(
	downloadingComic *comic_center2.ComicDownload,
	downloadingEp *comic_center2.ComicDownloadEp,
	downloadingPicture *comic_center2.ComicDownloadPicture,
	buff []byte,
	format string,
) {
	if downloadAndExportPath == "" {
		return
	}
	if i, e := os.Stat(downloadAndExportPath); e == nil {
		if i.IsDir() {
			// 进入漫画目录
			comicDir := path.Join(downloadAndExportPath, utils2.ReasonableFileName(downloadingComic.Title))
			i, e = os.Stat(comicDir)
			if e != nil {
				if os.IsNotExist(e) {
					e = os.Mkdir(comicDir, utils2.CreateDirMode)
				} else {
					return
				}
			}
			if e != nil {
				return
			}
			// 进入章节目录
			epDir := path.Join(comicDir, utils2.ReasonableFileName(fmt.Sprintf("%02d - ", downloadingEp.EpOrder)+downloadingEp.Title))
			i, e = os.Stat(epDir)
			if e != nil {
				if os.IsNotExist(e) {
					e = os.Mkdir(epDir, utils2.CreateDirMode)
				} else {
					return
				}
			}
			if e != nil {
				return
			}
			// 写入文件
			filePath := path.Join(epDir, fmt.Sprintf("%03d.%s", downloadingPicture.RankInEp, aliasFormat(format)))
			ioutil.WriteFile(filePath, buff, utils2.CreateFileMode)
		}
	}
}

// 边下载边导出(导出logo)
func downloadAndExportLogo(
	downloadingComic *comic_center2.ComicDownload,
) {
	if downloadAndExportPath == "" {
		return
	}
	comicLogoPath := downloadPath(path.Join(downloadingComic.ID, "logo"))
	if _, e := os.Stat(comicLogoPath); e == nil {
		buff, e := ioutil.ReadFile(comicLogoPath)
		if e == nil {
			_, f, e := image.Decode(bytes.NewBuffer(buff))
			if e == nil {
				if i, e := os.Stat(downloadAndExportPath); e == nil {
					if i.IsDir() {
						// 进入漫画目录
						comicDir := path.Join(downloadAndExportPath, utils2.ReasonableFileName(downloadingComic.Title))
						i, e = os.Stat(comicDir)
						if e != nil {
							if os.IsNotExist(e) {
								e = os.Mkdir(comicDir, utils2.CreateDirMode)
							}
						}
						if e != nil {
							return
						}
						// 写入文件
						filePath := path.Join(comicDir, fmt.Sprintf("%s.%s", "logo", aliasFormat(f)))
						ioutil.WriteFile(filePath, buff, utils2.CreateFileMode)
					}
				}
			}
		}
	}
}

// jpeg的拓展名
func aliasFormat(format string) string {
	if format == "jpeg" {
		return "jpg"
	}
	return format
}
