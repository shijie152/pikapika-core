package pikapika

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"pikapika/pikapika/database/comic_center"
	"pikapika/pikapika/database/local_favorite"
	"time"

	"github.com/studio-b12/gowebdav"
)

// ---------- 浏览记录 ----------

func viewComic(comicId string) error {
	comic, err := loadComicInfoRemote(comicId)
	if err != nil {
		return err
	}
	view := comic_center.ComicView{}
	view.ID = comicId
	view.CreatedAt = comic.CreatedAt
	view.UpdatedAt = comic.UpdatedAt
	view.Title = comic.Title
	view.Author = comic.Author
	view.PagesCount = int32(comic.PagesCount)
	view.EpsCount = int32(comic.EpsCount)
	view.Finished = comic.Finished
	c, _ := json.Marshal(comic.Categories)
	view.Categories = string(c)
	view.ThumbOriginalName = comic.Thumb.OriginalName
	view.ThumbFileServer = comic.Thumb.FileServer
	view.ThumbPath = comic.Thumb.Path
	view.LikesCount = int32(comic.LikesCount)
	view.Description = comic.Description
	view.ChineseTeam = comic.ChineseTeam
	t, _ := json.Marshal(comic.Tags)
	view.Tags = string(t)
	view.AllowDownload = comic.AllowDownload
	view.ViewsCount = int32(comic.ViewsCount)
	view.IsFavourite = comic.IsFavourite
	view.IsLiked = comic.IsLiked
	view.CommentsCount = int32(comic.CommentsCount)
	return comic_center.ViewComicUpdateInfo(&view)
}

// ---------- 历史记录文件 ----------

// 历史文件格式: JSON 数组, 每个元素为漫画浏览记录
func exportHistoriesToFile(file string) error {
	views, err := comic_center.AllViewLogs()
	if err != nil {
		return err
	}
	buff, err := json.Marshal(views)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, buff, 0600)
}

func mergeHistoriesFromFile(file string) error {
	buff, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	var views []comic_center.ComicView
	err = json.Unmarshal(buff, &views)
	if err != nil {
		return err
	}
	for i := range views {
		view := &views[i]
		err := comic_center.MergeViewLog(view)
		if err != nil {
			return err
		}
	}
	return nil
}

func mergeHistoriesFromLocal(localPath string) error {
	real := filepath.Join(localPath, "pk.histories")
	if _, err := os.Stat(real); err != nil {
		return err
	}
	return mergeHistoriesFromFile(real)
}

func mergeHistoriesFromWebDav(params string) error {
	var paramsStruct struct {
		Root     string `json:"root"`
		Username string `json:"username"`
		Password string `json:"password"`
		File     string `json:"file"`
		Direction string `json:"direction"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return err
	}
	client := gowebdav.NewClient(paramsStruct.Root, paramsStruct.Username, paramsStruct.Password)
	client.SetTimeout(30 * time.Second)
	remoteFile := "/" + paramsStruct.File
	switch paramsStruct.Direction {
	case "up":
		return exportHistoriesToWebDav(client, remoteFile)
	case "all":
		// 双向: 下载远端, 合并, 上传
		if err := mergeHistoriesFromWebDavRemote(client, remoteFile); err != nil {
			return err
		}
		return exportHistoriesToWebDav(client, remoteFile)
	default:
		return errors.New("unknown direction : " + paramsStruct.Direction)
	}
}

func mergeHistoriesFromWebDavRemote(client *gowebdav.Client, remoteFile string) error {
	buff, err := client.Read(remoteFile)
	if err != nil {
		return err
	}
	var views []comic_center.ComicView
	err = json.Unmarshal(buff, &views)
	if err != nil {
		return err
	}
	for i := range views {
		view := &views[i]
		err := comic_center.MergeViewLog(view)
		if err != nil {
			return err
		}
	}
	return nil
}

func exportHistoriesToWebDav(client *gowebdav.Client, remoteFile string) error {
	views, err := comic_center.AllViewLogs()
	if err != nil {
		return err
	}
	buff, err := json.Marshal(views)
	if err != nil {
		return err
	}
	return client.Write(remoteFile, buff, 0600)
}

// ---------- 本地收藏同步 ----------

type localFavoriteSyncData struct {
	Folders []local_favorite.LocalFavoriteFolder `json:"folders"`
	Comics  []local_favorite.LocalFavoriteComic  `json:"comics"`
}

func mergeLocalFavoritesFromWebDav(params string) error {
	var paramsStruct struct {
		Root     string `json:"root"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return err
	}
	client := gowebdav.NewClient(paramsStruct.Root, paramsStruct.Username, paramsStruct.Password)
	client.SetTimeout(30 * time.Second)
	remoteFile := "/pk.local_favorites"
	// 下载远端
	var remote localFavoriteSyncData
	if buff, err := client.Read(remoteFile); err == nil {
		json.Unmarshal(buff, &remote)
	}
	// 合并: 以本地为准, 保留远端独有的文件夹
	localFolders, err := local_favorite.ListFolders()
	if err != nil {
		return err
	}
	localComics, err := local_favorite.ListAllComics()
	if err != nil {
		return err
	}
	mergedFolders := localFolders
	localFolderIds := map[string]bool{}
	for _, f := range localFolders {
		localFolderIds[f.ID] = true
	}
	for _, f := range remote.Folders {
		if !localFolderIds[f.ID] {
			mergedFolders = append(mergedFolders, f)
		}
	}
	mergedComics := localComics
	localComicIds := map[string]bool{}
	for _, c := range localComics {
		localComicIds[c.ComicId] = true
	}
	for _, c := range remote.Comics {
		if !localComicIds[c.ComicId] {
			mergedComics = append(mergedComics, c)
		}
	}
	err = local_favorite.MergeAll(mergedFolders, mergedComics)
	if err != nil {
		return err
	}
	// 上传合并结果
	buff, err := json.Marshal(localFavoriteSyncData{Folders: mergedFolders, Comics: mergedComics})
	if err != nil {
		return err
	}
	return client.Write(remoteFile, buff, 0600)
}
