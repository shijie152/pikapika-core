package pikapika

import (
	"archive/tar"
	"fmt"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"gorm.io/gorm"
	"io"
	"io/ioutil"
	"net"
	"os"
	path2 "path"
	"pikapika/pikapika/database/comic_center"
	"pikapika/pikapika/utils"
	"strconv"
	"strings"
)

func importComicDownloadUsingSocket(addr string) error {
	//
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	gr, err := gzip.NewReader(conn)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gr)
	//
	zipPath := path2.Join(tmpDir, "tmp.zip")
	closed := false
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer func() {
		if !closed {
			zipFile.Close()
		}
		os.Remove(zipPath)
	}()
	zipWriter := zip.NewWriter(zipFile)
	defer func() {
		if !closed {
			zipWriter.Close()
		}
	}()
	//
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		writer, err := zipWriter.Create(header.Name)
		if err != nil {
			return err
		}
		_, err = io.Copy(writer, tr)
		if err != nil {
			return err
		}
	}
	err = zipWriter.Close()
	zipFile.Close()
	closed = true
	return importComicDownload(zipPath)
}

func importComicDownload(zipPath string) error {
	zip, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zip.Close()
	dataJs, err := zip.Open("data.js")
	if err != nil {
		return err
	}
	defer dataJs.Close()
	dataBuff, err := ioutil.ReadAll(dataJs)
	if err != nil {
		return err
	}
	data := strings.TrimLeft(string(dataBuff), "data = ")
	var jsonComicDownload JsonComicDownload
	err = json.Unmarshal([]byte(data), &jsonComicDownload)
	if err != nil {
		return err
	}
	return comic_center.Transaction(func(tx *gorm.DB) error {
		// 删除
		err := tx.Unscoped().Delete(&comic_center.ComicDownload{}, "id = ?", jsonComicDownload.ID).Error
		if err != nil {
			return err
		}
		err = tx.Unscoped().Delete(&comic_center.ComicDownloadEp{}, "comic_id = ?", jsonComicDownload.ID).Error
		if err != nil {
			return err
		}
		err = tx.Unscoped().Delete(&comic_center.ComicDownloadPicture{}, "comic_id = ?", jsonComicDownload.ID).Error
		if err != nil {
			return err
		}
		// 插入
		err = tx.Save(&jsonComicDownload.ComicDownload).Error
		if err != nil {
			return err
		}
		for _, ep := range jsonComicDownload.EpList {
			err = tx.Save(&ep.ComicDownloadEp).Error
			if err != nil {
				return err
			}
			for _, picture := range ep.PictureList {
				notifyExport("事务 : " + picture.LocalPath)
				err = tx.Save(&picture.ComicDownloadPicture).Error
				if err != nil {
					return err
				}
			}
		}
		// VIEW日志
		view := comic_center.ComicView{}
		view.ID = jsonComicDownload.ID
		view.CreatedAt = jsonComicDownload.CreatedAt
		view.UpdatedAt = jsonComicDownload.UpdatedAt
		view.Title = jsonComicDownload.Title
		view.Author = jsonComicDownload.Author
		view.PagesCount = jsonComicDownload.PagesCount
		view.EpsCount = jsonComicDownload.EpsCount
		view.Finished = jsonComicDownload.Finished
		c, _ := json.Marshal(jsonComicDownload.Categories)
		view.Categories = string(c)
		view.ThumbOriginalName = jsonComicDownload.ThumbOriginalName
		view.ThumbFileServer = jsonComicDownload.ThumbFileServer
		view.ThumbPath = jsonComicDownload.ThumbPath
		view.LikesCount = 0
		view.Description = jsonComicDownload.Description
		view.ChineseTeam = jsonComicDownload.ChineseTeam
		t, _ := json.Marshal(jsonComicDownload.Tags)
		view.Tags = string(t)
		view.AllowDownload = true
		view.ViewsCount = 0
		view.IsFavourite = false
		view.IsLiked = false
		view.CommentsCount = 0
		err = comic_center.NoLockActionViewComicUpdateInfoDB(&view, tx)
		if err != nil {
			return err
		}
		// 覆盖文件
		comicDirPath := downloadPath(jsonComicDownload.ID)
		utils.Mkdir(comicDirPath)
		logoReader, err := zip.Open("logo")
		if err == nil {
			defer logoReader.Close()
			logoBuff, err := ioutil.ReadAll(logoReader)
			if err != nil {
				return err
			}
			ioutil.WriteFile(path2.Join(comicDirPath, "logo"), logoBuff, utils.CreateFileMode)
		}
		for _, ep := range jsonComicDownload.EpList {
			utils.Mkdir(path2.Join(comicDirPath, strconv.Itoa(int(ep.EpOrder))))
			for _, picture := range ep.PictureList {
				notifyExport("写入 : " + picture.LocalPath)
				zipEntry, err := zip.Open(picture.SrcPath)
				if err != nil {
					return err
				}
				err = func() error {
					defer zipEntry.Close()
					entryBuff, err := ioutil.ReadAll(zipEntry)
					if err != nil {
						return err
					}
					return ioutil.WriteFile(downloadPath(picture.LocalPath), entryBuff, utils.CreateFileMode)
				}()
				if err != nil {
					return err
				}
			}
		}
		// 结束
		return nil
	})
}

// importComicDownloadDir 导入文件夹下所有支持的归档文件
func importComicDownloadDir(dir string) error {
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		filePath := path2.Join(dir, name)
		if strings.HasSuffix(name, ".zip") {
			if err := importComicDownload(filePath); err != nil {
				notifyExport("导入失败 : " + name + " " + err.Error())
			}
		} else if strings.HasSuffix(name, ".pki") {
			if err := importComicDownloadPki(filePath); err != nil {
				notifyExport("导入失败 : " + name + " " + err.Error())
			}
		}
	}
	return nil
}

// importComicDownloadPki 导入 pki 归档 (data.js 为数组, 支持多漫画)
func importComicDownloadPki(zipPath string) error {
	z, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer z.Close()
	dataJs, err := z.Open("data.js")
	if err != nil {
		return err
	}
	defer dataJs.Close()
	dataBuff, err := ioutil.ReadAll(dataJs)
	if err != nil {
		return err
	}
	data := strings.TrimLeft(string(dataBuff), "data = ")
	var jsonComics []JsonComicDownload
	err = json.Unmarshal([]byte(data), &jsonComics)
	if err != nil {
		return err
	}
	for _, jsonComic := range jsonComics {
		err = importJsonComicDownload(z, &jsonComic, zipPath)
		if err != nil {
			return err
		}
	}
	return nil
}

// importJsonComicDownload 把 JsonComicDownload 写入数据库与文件系统
// 图片来源: zip 内 {comicId}/pictures/... 或 data.js 中的 srcPath
func importJsonComicDownload(z *zip.ReadCloser, jsonComic *JsonComicDownload, zipPath string) error {
	return comic_center.Transaction(func(tx *gorm.DB) error {
		// 删除旧记录
		err := tx.Unscoped().Delete(&comic_center.ComicDownload{}, "id = ?", jsonComic.ID).Error
		if err != nil {
			return err
		}
		err = tx.Unscoped().Delete(&comic_center.ComicDownloadEp{}, "comic_id = ?", jsonComic.ID).Error
		if err != nil {
			return err
		}
		err = tx.Unscoped().Delete(&comic_center.ComicDownloadPicture{}, "comic_id = ?", jsonComic.ID).Error
		if err != nil {
			return err
		}
		// 插入
		err = tx.Save(&jsonComic.ComicDownload).Error
		if err != nil {
			return err
		}
		for _, ep := range jsonComic.EpList {
			err = tx.Save(&ep.ComicDownloadEp).Error
			if err != nil {
				return err
			}
			for _, picture := range ep.PictureList {
				err = tx.Save(&picture.ComicDownloadPicture).Error
				if err != nil {
					return err
				}
			}
		}
		// VIEW 日志
		view := comic_center.ComicView{}
		view.ID = jsonComic.ID
		view.CreatedAt = jsonComic.CreatedAt
		view.UpdatedAt = jsonComic.UpdatedAt
		view.Title = jsonComic.Title
		view.Author = jsonComic.Author
		view.PagesCount = jsonComic.PagesCount
		view.EpsCount = jsonComic.EpsCount
		view.Finished = jsonComic.Finished
		c, _ := json.Marshal(jsonComic.Categories)
		view.Categories = string(c)
		view.ThumbOriginalName = jsonComic.ThumbOriginalName
		view.ThumbFileServer = jsonComic.ThumbFileServer
		view.ThumbPath = jsonComic.ThumbPath
		view.LikesCount = 0
		view.Description = jsonComic.Description
		view.ChineseTeam = jsonComic.ChineseTeam
		t, _ := json.Marshal(jsonComic.Tags)
		view.Tags = string(t)
		view.AllowDownload = true
		view.ViewsCount = 0
		view.IsFavourite = false
		view.IsLiked = false
		view.CommentsCount = 0
		err = comic_center.NoLockActionViewComicUpdateInfoDB(&view, tx)
		if err != nil {
			return err
		}
		// 写入文件
		comicDirPath := downloadPath(jsonComic.ID)
		utils.Mkdir(comicDirPath)
		// logo
		for _, logoPath := range []string{fmt.Sprintf("%s/logo", jsonComic.ID), "logo"} {
			logoReader, err := z.Open(logoPath)
			if err == nil {
				logoBuff, err := ioutil.ReadAll(logoReader)
				logoReader.Close()
				if err == nil {
					ioutil.WriteFile(path2.Join(comicDirPath, "logo"), logoBuff, utils.CreateFileMode)
				}
				break
			}
		}
		// 图片
		for _, ep := range jsonComic.EpList {
			utils.Mkdir(path2.Join(comicDirPath, strconv.Itoa(int(ep.EpOrder))))
			for _, picture := range ep.PictureList {
				if picture.DownloadFinished == false {
					continue
				}
				var zipEntry io.ReadCloser
				var err error
				if picture.SrcPath != "" {
					zipEntry, err = z.Open(picture.SrcPath)
				} else {
					zipEntry, err = z.Open(fmt.Sprintf("%s/pictures/%04d_%04d", jsonComic.ID, ep.EpOrder, picture.RankInEp))
				}
				if err != nil {
					return err
				}
				err = func() error {
					defer zipEntry.Close()
					entryBuff, err := ioutil.ReadAll(zipEntry)
					if err != nil {
						return err
					}
					return ioutil.WriteFile(downloadPath(picture.LocalPath), entryBuff, utils.CreateFileMode)
				}()
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
}
