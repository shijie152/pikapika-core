package pikapika

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"pikapika/pikapika/database/comic_center"
	"pikapika/pikapika/database/local_favorite"
	"pikapika/pikapika/database/properties"
	"pikapika/pikapika/database/network_cache"
	"pikapika/pikapika/utils"
	"strconv"
	"time"

	source "github.com/niuhuan/pica-go"
)

// ---------- 应用目录 ----------

func getHomeDir() (string, error) {
	return homeDir, nil
}

func mkdirs(p string) error {
	return os.MkdirAll(p, 0700)
}

// ---------- 下载批量添加 ----------

func downloadAll(params string) error {
	var comicIds []string
	err := json.Unmarshal([]byte(params), &comicIds)
	if err != nil {
		return err
	}
	for _, comicId := range comicIds {
		err := downloadOne(comicId)
		if err != nil {
			return err
		}
	}
	return nil
}

func downloadOne(comicId string) error {
	// 检查是否已在下载列表
	if exist, _ := comic_center.FindComicDownloadById(comicId); exist != nil {
		return nil
	}
	comic, err := loadComicInfoRemote(comicId)
	if err != nil {
		return err
	}
	download := comicInfoToDownload(comic)
	// 获取全部章节
	var eps []comic_center.ComicDownloadEp
	page := 1
	for {
		epPage, err := client.ComicEpPage(comicId, page)
		if err != nil {
			return err
		}
		for _, ep := range epPage.Docs {
			eps = append(eps, comic_center.ComicDownloadEp{
				ComicId: comicId,
				ID:      ep.Id,
				EpOrder: int32(ep.Order),
				Title:   ep.Title,
			})
		}
		if epPage.Page < epPage.Pages {
			page++
			continue
		}
		break
	}
	if len(eps) == 0 {
		return errors.New("no eps")
	}
	err = comic_center.CreateDownload(&download, &eps)
	if err != nil {
		return err
	}
	// 创建文件夹 + 复制图标
	utils.Mkdir(downloadPath(download.ID))
	downloadComicLogo(&download)
	return nil
}

func comicInfoToDownload(comic *source.ComicInfo) comic_center.ComicDownload {
	var download comic_center.ComicDownload
	download.ID = comic.Id
	download.CreatedAt = comic.CreatedAt
	download.UpdatedAt = comic.UpdatedAt
	download.Title = comic.Title
	download.Author = comic.Author
	download.PagesCount = int32(comic.PagesCount)
	download.EpsCount = int32(comic.EpsCount)
	download.Finished = comic.Finished
	c, _ := json.Marshal(comic.Categories)
	download.Categories = string(c)
	download.ThumbOriginalName = comic.Thumb.OriginalName
	download.ThumbFileServer = comic.Thumb.FileServer
	download.ThumbPath = comic.Thumb.Path
	download.Description = comic.Description
	download.ChineseTeam = comic.ChineseTeam
	t, _ := json.Marshal(comic.Tags)
	download.Tags = string(t)
	return download
}

// ---------- 下载移动 ----------

func moveDownloadComic(params string) error {
	var paramsStruct struct {
		ComicIdList  []string `json:"comicIdList"`
		CustomFolder string   `json:"customFolder"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return err
	}
	return comic_center.MoveDownloadToFolder(paramsStruct.ComicIdList, paramsStruct.CustomFolder)
}

// ---------- 下载缓存路径 ----------

func loadDownloadCachePath() (string, error) {
	return properties.LoadProperty("downloadCachePath", "")
}

func saveDownloadCachePath(folder string) error {
	return properties.SaveProperty("downloadCachePath", folder)
}

// ---------- 图片加载方式 ----------

func setUseApiClientLoadImage(use string) error {
	return properties.SaveProperty("useApiClientLoadImage", use)
}

func getUseApiClientLoadImage() (string, error) {
	return properties.LoadProperty("useApiClientLoadImage", "false")
}

// ---------- 排行榜 ----------

func leaderboardOfKnight() (string, error) {
	knights, err := client.LeaderboardOfKnight()
	if err != nil {
		return "", err
	}
	return serialize(knights, nil)
}

// ---------- 本地收藏单查 ----------

func getLocalFavoriteComic(comicId string) (string, error) {
	comic, err := local_favorite.GetComic(comicId)
	if err != nil {
		return "", nil
	}
	if comic == nil {
		return "", nil
	}
	return serialize(comic, nil)
}

// ---------- 分流地址 ----------

func reloadSwitchAddress() error {
	// 从作者服务器同步分流地址 (失败时保留现有配置)
	body, err := defaultHttpClientGet(switchAddressUrl())
	if err != nil {
		return err
	}
	var data []string
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return err
	}
	buff, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return properties.SaveProperty("switchAddresses", string(buff))
}

func resetSwitchAddress() error {
	return properties.SaveProperty("switchAddresses", "")
}

// 获取分流地址列表 (优先远端同步的, 否则使用内置默认)
func loadSwitchAddresses() []string {
	raw, _ := properties.LoadProperty("switchAddresses", "")
	if raw != "" {
		var data []string
		if err := json.Unmarshal([]byte(raw), &data); err == nil && len(data) > 0 {
			return data
		}
	}
	return []string{"172.67.7.24:443", "104.20.180.50:443", "172.67.208.169:443"}
}

func switchAddressUrl() string {
	u, _ := properties.LoadProperty("switchAddressUrl", "https://cdn.comicsparks.work/cfg/pikapika/switch_addresses.json")
	return u
}

// ---------- 网络测速 ----------

func ping(idx string) (string, error) {
	i, err := strconv.Atoi(idx)
	if err != nil {
		return "", err
	}
	addrs := loadSwitchAddresses()
	if i < 0 || i >= len(addrs) {
		return "", errors.New("invalid idx")
	}
	ms, err := dialMeasure(addrs[i])
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(ms, 10), nil
}

func pingImg(idx string) (string, error) {
	i, err := strconv.Atoi(idx)
	if err != nil {
		return "", err
	}
	addrs := loadSwitchAddresses()
	if i < 0 || i >= len(addrs) {
		return "", errors.New("invalid idx")
	}
	ms, err := dialMeasure(addrs[i])
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(ms, 10), nil
}

func dialMeasure(addr string) (int64, error) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return 0, err
	}
	conn.Close()
	return time.Since(start).Milliseconds(), nil
}

var _ = http.DefaultClient
var _ = network_cache.LoadCache
var _ = fmt.Sprintf
var _ = path.Join
