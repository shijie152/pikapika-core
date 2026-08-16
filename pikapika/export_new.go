package pikapika

import (
	"archive/zip"
	"image"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	path2 "path"
	"pikapika/pikapika/database/comic_center"
	"pikapika/pikapika/utils"
	"strconv"
	"strings"
	"time"

	"github.com/niuhuan/pkz-go"
	"github.com/signintech/gopdf"
)

// 加载一个下载任务的完整数据 (漫画 + 章节 + 图片)
func loadComicExportData(comicId string, requireFinished bool) (*comic_center.ComicDownload, []comic_center.ComicDownloadEp, error) {
	comic, err := comic_center.FindComicDownloadById(comicId)
	if err != nil {
		return nil, nil, err
	}
	if comic == nil {
		return nil, nil, errors.New("not found")
	}
	if requireFinished && !comic.DownloadFinished {
		return nil, nil, errors.New("not download finish")
	}
	epList, err := comic_center.ListDownloadEpByComicId(comicId)
	if err != nil {
		return nil, nil, err
	}
	return comic, epList, nil
}

func listPicturesByEp(ep comic_center.ComicDownloadEp) ([]comic_center.ComicDownloadPicture, error) {
	return comic_center.ListDownloadPictureByEpId(ep.ID)
}

func readDownloadFile(picture comic_center.ComicDownloadPicture) ([]byte, error) {
	return ioutil.ReadFile(downloadPath(picture.LocalPath))
}

// ---------- PKZ 导出 ----------

func exportComicDownloadToPkz(params string) error {
	var paramsStruct struct {
		ComicIds []string `json:"comicIds"`
		Dir      string   `json:"dir"`
		Name     string   `json:"name"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(paramsStruct.Name)
	if len(name) == 0 {
		name = fmt.Sprintf("pikapika-%s.pkz", time.Now().Format("2006_01_02_15_04_05.999"))
	}
	if !strings.HasSuffix(name, ".pkz") {
		name += ".pkz"
	}
	filePath := path2.Join(paramsStruct.Dir, name)
	ex, err := utils.Exists(filePath)
	if err != nil {
		return err
	}
	if ex {
		return errors.New("exists")
	}
	fileStream, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer fileStream.Close()
	comics := make([]*comic_center.ComicDownload, 0, len(paramsStruct.ComicIds))
	epLists := make([][]comic_center.ComicDownloadEp, 0, len(paramsStruct.ComicIds))
	for _, comicId := range paramsStruct.ComicIds {
		comic, epList, err := loadComicExportData(comicId, true)
		if err != nil {
			return err
		}
		comics = append(comics, comic)
		epLists = append(epLists, epList)
	}
	fetcher := &pkz.ComicsFetcher{
		ArchiveInfo: func() (*pkz.ArchiveInfo, error) {
			if len(comics) == 1 {
				return &pkz.ArchiveInfo{Name: comics[0].Title, Author: comics[0].Author, Description: comics[0].Description}, nil
			}
			return &pkz.ArchiveInfo{Name: "pikapika", Author: "", Description: ""}, nil
		},
		ArchiveCover: func() ([]byte, error) {
			return readLogoBytes(comics[0])
		},
		ArchiveAuthorAvatar: func() ([]byte, error) {
			return nil, nil
		},
		ComicCount: func() (int, error) {
			return len(comics), nil
		},
		ComicInfo: func(comicIdx int) (*pkz.ComicInfo, error) {
			comic := comics[comicIdx]
			return &pkz.ComicInfo{
				Id: comic.ID, Title: comic.Title, Author: comic.Author,
				UpdatedAt: comic.UpdatedAt.Unix(), CreatedAt: comic.CreatedAt.Unix(),
				Description: comic.Description, ChineseTeam: comic.ChineseTeam, Finished: comic.Finished,
			}, nil
		},
		ComicCover: func(comicIdx int, comicInfo *pkz.ComicInfo) ([]byte, error) {
			return readLogoBytes(comics[comicIdx])
		},
		ComicAuthorAvatar: func(comicIdx int, comicInfo *pkz.ComicInfo) ([]byte, error) {
			return nil, nil
		},
		VolumeCount: func(comicIdx int, comicInfo *pkz.ComicInfo) (int, error) {
			return 1, nil
		},
		VolumeInfo: func(comicIdx int, comicInfo *pkz.ComicInfo, volumeIdx int) (*pkz.VolumeInfo, error) {
			comic := comics[comicIdx]
			return &pkz.VolumeInfo{Id: comic.ID, Title: comic.Title, CreatedAt: comic.CreatedAt.Unix(), UpdatedAt: comic.UpdatedAt.Unix()}, nil
		},
		VolumeCover: func(comicIdx int, comicInfo *pkz.ComicInfo, volumeIdx int, volumeInfo *pkz.VolumeInfo) ([]byte, error) {
			return readLogoBytes(comics[comicIdx])
		},
		ChapterCount: func(comicIdx int, comicInfo *pkz.ComicInfo, volumeIdx int, volumeInfo *pkz.VolumeInfo) (int, error) {
			return len(epLists[comicIdx]), nil
		},
		ChapterInfo: func(comicIdx int, comicInfo *pkz.ComicInfo, volumeIdx int, volumeInfo *pkz.VolumeInfo, chapterIdx int) (*pkz.ChapterInfo, error) {
			ep := epLists[comicIdx][chapterIdx]
			return &pkz.ChapterInfo{Id: ep.ID, Title: ep.Title, CreatedAt: ep.UpdatedAt.Unix(), UpdatedAt: ep.UpdatedAt.Unix()}, nil
		},
		ChapterCover: func(comicIdx int, comicInfo *pkz.ComicInfo, volumeIdx int, volumeInfo *pkz.VolumeInfo, chapterIdx int, chapterInfo *pkz.ChapterInfo) ([]byte, error) {
			return readLogoBytes(comics[comicIdx])
		},
		PictureCount: func(comicIdx int, comicInfo *pkz.ComicInfo, volumeIdx int, volumeInfo *pkz.VolumeInfo, chapterIdx int, chapterInfo *pkz.ChapterInfo) (int, error) {
			pictures, err := listPicturesByEp(epLists[comicIdx][chapterIdx])
			if err != nil {
				return 0, err
			}
			return len(pictures), nil
		},
		PictureInfo: func(comicIdx int, comicInfo *pkz.ComicInfo, volumeIdx int, volumeInfo *pkz.VolumeInfo, chapterIdx int, chapterInfo *pkz.ChapterInfo, pictureIdx int) (*pkz.PictureInfo, error) {
			pictures, err := listPicturesByEp(epLists[comicIdx][chapterIdx])
			if err != nil {
				return nil, err
			}
			picture := pictures[pictureIdx]
			return &pkz.PictureInfo{
				Id: picture.ID, Title: fmt.Sprintf("%04d", picture.RankInEp),
				Width: int(picture.Width), Height: int(picture.Height), Format: picture.Format,
			}, nil
		},
		PictureData: func(comicIdx int, comicInfo *pkz.ComicInfo, volumeIdx int, volumeInfo *pkz.VolumeInfo, chapterIdx int, chapterInfo *pkz.ChapterInfo, pictureIdx int, pictureInfo *pkz.PictureInfo) ([]byte, error) {
			pictures, err := listPicturesByEp(epLists[comicIdx][chapterIdx])
			if err != nil {
				return nil, err
			}
			return readDownloadFile(pictures[pictureIdx])
		},
	}
	return pkz.WritePkz(fileStream, fetcher)
}

func readLogoBytes(comic *comic_center.ComicDownload) ([]byte, error) {
	if comic.ThumbLocalPath == "" {
		return nil, nil
	}
	return ioutil.ReadFile(downloadPath(comic.ThumbLocalPath))
}

// ---------- PKI 导出 (data.js 数组格式) ----------

func exportComicDownloadToPki(params string) error {
	var paramsStruct struct {
		ComicId string `json:"comicId"`
		Dir     string `json:"dir"`
		Name    string `json:"name"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return err
	}
	return writePkiArchive([]string{paramsStruct.ComicId}, paramsStruct.Dir, paramsStruct.Name)
}

func exportAnyComicDownloadsToPki(params string) error {
	var paramsStruct struct {
		ComicIds []string `json:"comicIds"`
		Dir      string   `json:"dir"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return err
	}
	return writePkiArchive(paramsStruct.ComicIds, paramsStruct.Dir, "")
}

func writePkiArchive(comicIds []string, dir string, name string) error {
	if len(comicIds) == 0 {
		return errors.New("empty comic ids")
	}
	if len(name) == 0 {
		name = fmt.Sprintf("pikapika-%s.pki", time.Now().Format("2006_01_02_15_04_05.999"))
	}
	if !strings.HasSuffix(name, ".pki") {
		name += ".pki"
	}
	filePath := path2.Join(dir, name)
	ex, err := utils.Exists(filePath)
	if err != nil {
		return err
	}
	if ex {
		return errors.New("exists")
	}
	fileStream, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer fileStream.Close()
	zipWriter := zip.NewWriter(fileStream)
	defer zipWriter.Close()
	jsonComics := make([]JsonComicDownload, 0, len(comicIds))
	for _, comicId := range comicIds {
		jsonComic, err := buildJsonComicDownload(comicId, true)
		if err != nil {
			return err
		}
		// 写图片文件到 {comicId}/pictures/
		for _, ep := range jsonComic.EpList {
			for _, picture := range ep.PictureList {
				notifyExport(fmt.Sprintf("正在导出 PKI %s EP:%d PIC:%d", comicId, ep.EpOrder, picture.RankInEp))
				innerPath := fmt.Sprintf("%s/pictures/%04d_%04d", comicId, ep.EpOrder, picture.RankInEp)
				buff, err := readDownloadFile(picture.ComicDownloadPicture)
				if err != nil {
					return err
				}
				w, err := zipWriter.Create(innerPath)
				if err != nil {
					return err
				}
				_, err = w.Write(buff)
				if err != nil {
					return err
				}
			}
		}
		// 写 logo
		if logoBuff, err := readLogoBytes(&jsonComic.ComicDownload); err == nil && logoBuff != nil {
			w, err := zipWriter.Create(fmt.Sprintf("%s/logo", comicId))
			if err != nil {
				return err
			}
			w.Write(logoBuff)
		}
		jsonComics = append(jsonComics, jsonComic)
	}
	// data.js
	dataBuff, err := json.Marshal(&jsonComics)
	if err != nil {
		return err
	}
	w, err := zipWriter.Create("data.js")
	if err != nil {
		return err
	}
	_, err = w.Write(append([]byte("data = "), dataBuff...))
	return err
}

func buildJsonComicDownload(comicId string, requireFinished bool) (JsonComicDownload, error) {
	var jsonComic JsonComicDownload
	comic, epList, err := loadComicExportData(comicId, requireFinished)
	if err != nil {
		return jsonComic, err
	}
	jsonComic.ComicDownload = *comic
	jsonComic.EpList = make([]JsonComicDownloadEp, 0)
	for _, ep := range epList {
		jsonEp := JsonComicDownloadEp{}
		jsonEp.ComicDownloadEp = ep
		jsonEp.PictureList = make([]JsonComicDownloadPicture, 0)
		pictures, err := listPicturesByEp(ep)
		if err != nil {
			return jsonComic, err
		}
		for _, picture := range pictures {
			jsonPicture := JsonComicDownloadPicture{}
			jsonPicture.ComicDownloadPicture = picture
			jsonPicture.SrcPath = fmt.Sprintf("pictures/%04d_%04d", ep.EpOrder, picture.RankInEp)
			jsonEp.PictureList = append(jsonEp.PictureList, jsonPicture)
		}
		jsonComic.EpList = append(jsonComic.EpList, jsonEp)
	}
	return jsonComic, nil
}

// ---------- 批量 ZIP ----------

func exportAnyComicDownloadsToZip(params string) error {
	var paramsStruct struct {
		ComicIds []string `json:"comicIds"`
		Dir      string   `json:"dir"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return err
	}
	for _, comicId := range paramsStruct.ComicIds {
		comic, _, err := loadComicExportData(comicId, true)
		if err != nil {
			return err
		}
		_, err = exportComicDownload(fmt.Sprintf(`{"comicId":%q,"dir":%q,"name":""}`, comicId, paramsStruct.Dir))
		if err != nil {
			return err
		}
		_ = comic
	}
	return nil
}

// ---------- HTML+JPG ZIP ----------

func exportComicDownloadJpegZip(params string) error {
	var paramsStruct struct {
		ComicId string `json:"comicId"`
		Dir     string `json:"dir"`
		Name    string `json:"name"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return err
	}
	comic, epList, err := loadComicExportData(paramsStruct.ComicId, true)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(paramsStruct.Name)
	if len(name) == 0 {
		name = fmt.Sprintf("%s-%s", utils.ReasonableFileName(comic.Title), time.Now().Format("2006_01_02_15_04_05.999"))
	}
	filePath := path2.Join(paramsStruct.Dir, name+".zip")
	ex, err := utils.Exists(filePath)
	if err != nil {
		return err
	}
	if ex {
		return errors.New("exists")
	}
	fileStream, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer fileStream.Close()
	zipWriter := zip.NewWriter(fileStream)
	defer zipWriter.Close()
	for _, ep := range epList {
		pictures, err := listPicturesByEp(ep)
		if err != nil {
			return err
		}
		for _, picture := range pictures {
			notifyExport(fmt.Sprintf("正在导出 EP:%d PIC:%d", ep.EpOrder, picture.RankInEp))
			buff, err := readDownloadFile(picture)
			if err != nil {
				return err
			}
			w, err := zipWriter.Create(fmt.Sprintf("pictures/%04d_%04d.%s", ep.EpOrder, picture.RankInEp, picture.Format))
			if err != nil {
				return err
			}
			w.Write(buff)
		}
	}
	if logoBuff, err := readLogoBytes(comic); err == nil && logoBuff != nil {
		w, err := zipWriter.Create("logo")
		if err != nil {
			return err
		}
		w.Write(logoBuff)
	}
	// HTML
	w, err := zipWriter.Create("index.html")
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(indexHtml))
	return err
}

// ---------- CBZ 集合 ----------

func exportComicDownloadToCbzsZip(params string) error {
	var paramsStruct struct {
		ComicId string `json:"comicId"`
		Dir     string `json:"dir"`
		Name    string `json:"name"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return err
	}
	comic, epList, err := loadComicExportData(paramsStruct.ComicId, true)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(paramsStruct.Name)
	if len(name) == 0 {
		name = fmt.Sprintf("%s-%s", utils.ReasonableFileName(comic.Title), time.Now().Format("2006_01_02_15_04_05.999"))
	}
	filePath := path2.Join(paramsStruct.Dir, name+".zip")
	ex, err := utils.Exists(filePath)
	if err != nil {
		return err
	}
	if ex {
		return errors.New("exists")
	}
	fileStream, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer fileStream.Close()
	zipWriter := zip.NewWriter(fileStream)
	defer zipWriter.Close()
	for _, ep := range epList {
		cbzName := fmt.Sprintf("%s.cbz", utils.ReasonableFileName(ep.Title))
		cbzWriter, err := zipWriter.Create(cbzName)
		if err != nil {
			return err
		}
		inner := zip.NewWriter(cbzWriter)
		pictures, err := listPicturesByEp(ep)
		if err != nil {
			return err
		}
		for _, picture := range pictures {
			notifyExport(fmt.Sprintf("正在导出 CBZ EP:%d PIC:%d", ep.EpOrder, picture.RankInEp))
			buff, err := readDownloadFile(picture)
			if err != nil {
				return err
			}
			w, err := inner.Create(fmt.Sprintf("%04d.%s", picture.RankInEp, picture.Format))
			if err != nil {
				return err
			}
			w.Write(buff)
		}
		inner.Close()
	}
	return nil
}

// ---------- EPUB ----------

func exportComicDownloadToEpub(params string) error {
	var paramsStruct struct {
		ComicId string `json:"comicId"`
		Dir     string `json:"dir"`
		Name    string `json:"name"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return err
	}
	comic, epList, err := loadComicExportData(paramsStruct.ComicId, true)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(paramsStruct.Name)
	if len(name) == 0 {
		name = fmt.Sprintf("%s-%s", utils.ReasonableFileName(comic.Title), time.Now().Format("2006_01_02_15_04_05.999"))
	}
	filePath := path2.Join(paramsStruct.Dir, name+".epub")
	ex, err := utils.Exists(filePath)
	if err != nil {
		return err
	}
	if ex {
		return errors.New("exists")
	}
	fileStream, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer fileStream.Close()
	zipWriter := zip.NewWriter(fileStream)
	defer zipWriter.Close()
	// mimetype 必须为第一个文件且不压缩
	mimetypeWriter, err := zipWriter.CreateHeader(&zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store,
	})
	if err != nil {
		return err
	}
	mimetypeWriter.Write([]byte("application/epub+zip"))
	// container.xml
	container := `<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`
	w, _ := zipWriter.Create("META-INF/container.xml")
	w.Write([]byte(container))
	// 图片
	pictureFiles := make([]string, 0)
	manifest := ""
	spine := ""
	chapterIdx := 0
	picIdx := 0
	for _, ep := range epList {
		pictures, err := listPicturesByEp(ep)
		if err != nil {
			return err
		}
		chapterIdx++
		spine += fmt.Sprintf(`<itemref idref="chapter%d"/>`, chapterIdx)
		// 章节 XHTML
		xhtml := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>%s</title></head><body><h1>%s</h1>`, ep.Title, ep.Title)
		for _, picture := range pictures {
			picIdx++
			imgPath := fmt.Sprintf("images/%04d.%s", picIdx, picture.Format)
			pictureFiles = append(pictureFiles, imgPath)
			manifest += fmt.Sprintf(`<item id="img%d" href="%s" media-type="image/%s"/>`, picIdx, imgPath, picture.Format)
			xhtml += fmt.Sprintf(`<div><img src="%s" alt="%s"/></div>`, imgPath, picture.RankInEp)
			buff, err := readDownloadFile(picture)
			if err != nil {
				return err
			}
			w, _ := zipWriter.Create(imgPath)
			w.Write(buff)
		}
		xhtml += "</body></html>"
		manifest += fmt.Sprintf(`<item id="chapter%d" href="chapter%d.xhtml" media-type="application/xhtml+xml"/>`, chapterIdx, chapterIdx)
		w, _ := zipWriter.Create(fmt.Sprintf("OEBPS/chapter%d.xhtml", chapterIdx))
		w.Write([]byte(xhtml))
	}
	// content.opf
	opf := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?><package xmlns="http://www.idpf.org/2007/opf" unique-identifier="bookid" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="bookid">urn:uuid:%s</dc:identifier><dc:title>%s</dc:title><dc:creator>%s</dc:creator><dc:language>zh</dc:language></metadata><manifest><item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>%s</manifest><spine toc="ncx">%s</spine></package>`, utils.UUID(), comic.Title, comic.Author, manifest, spine)
	w, _ = zipWriter.Create("OEBPS/content.opf")
	w.Write([]byte(opf))
	// toc.ncx
	ncx := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?><ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><head></head><docTitle><text>%s</text></docTitle><navMap><navPoint id="navpoint1" playOrder="1"><navLabel><text>%s</text></navLabel><content src="chapter1.xhtml"/></navPoint></navMap></ncx>`, comic.Title, comic.Title)
	w, _ = zipWriter.Create("OEBPS/toc.ncx")
	w.Write([]byte(ncx))
	_ = pictureFiles
	return nil
}

// ---------- PDF ----------

func exportComicDownloadToPDF(params string) error {
	var paramsStruct struct {
		ComicId string `json:"comicId"`
		Dir     string `json:"dir"`
		Name    string `json:"name"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return err
	}
	comic, epList, err := loadComicExportData(paramsStruct.ComicId, true)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(paramsStruct.Name)
	if len(name) == 0 {
		name = fmt.Sprintf("%s-%s", utils.ReasonableFileName(comic.Title), time.Now().Format("2006_01_02_15_04_05.999"))
	}
	filePath := path2.Join(paramsStruct.Dir, name+".pdf")
	ex, err := utils.Exists(filePath)
	if err != nil {
		return err
	}
	if ex {
		return errors.New("exists")
	}
	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	for _, ep := range epList {
		pictures, err := listPicturesByEp(ep)
		if err != nil {
			return err
		}
		for _, picture := range pictures {
			notifyExport(fmt.Sprintf("正在导出 PDF EP:%d PIC:%d", ep.EpOrder, picture.RankInEp))
			buff, err := readDownloadFile(picture)
			if err != nil {
				return err
			}
			imgCfg, _, err := image.DecodeConfig(bytes.NewReader(buff))
			if err != nil {
				return err
			}
			img, err := gopdf.ImageHolderByBytes(buff)
			if err != nil {
				return err
			}
			// 适配 A4 页面
			pageW := 595.28
			pageH := 841.89
			scale := pageW / float64(imgCfg.Width)
			if float64(imgCfg.Height)*scale > pageH {
				scale = pageH / float64(imgCfg.Height)
			}
			imgW := float64(imgCfg.Width) * scale
			imgH := float64(imgCfg.Height) * scale
			pdf.AddPage()
			x := (pageW - imgW) / 2
			y := (pageH - imgH) / 2
			pdf.ImageByHolder(img, x, y, &gopdf.Rect{W: imgW, H: imgH})
		}
	}
	return pdf.WritePdf(filePath)
}

func exportComicDownloadToPDFFolder(params string) error {
	var paramsStruct struct {
		ComicId string `json:"comicId"`
		Dir     string `json:"dir"`
		Name    string `json:"name"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return err
	}
	comic, epList, err := loadComicExportData(paramsStruct.ComicId, true)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(paramsStruct.Name)
	if len(name) == 0 {
		name = fmt.Sprintf("%s-%s", utils.ReasonableFileName(comic.Title), time.Now().Format("2006_01_02_15_04_05.999"))
	}
	dirPath := path2.Join(paramsStruct.Dir, name)
	ex, err := utils.Exists(dirPath)
	if err != nil {
		return err
	}
	if ex {
		return errors.New("exists")
	}
	err = os.Mkdir(dirPath, utils.CreateDirMode)
	if err != nil {
		return err
	}
	for _, ep := range epList {
		pictures, err := listPicturesByEp(ep)
		if err != nil {
			return err
		}
		pdf := gopdf.GoPdf{}
		pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
		for _, picture := range pictures {
			notifyExport(fmt.Sprintf("正在导出 PDFF EP:%d PIC:%d", ep.EpOrder, picture.RankInEp))
			buff, err := readDownloadFile(picture)
			if err != nil {
				return err
			}
			imgCfg, _, err := image.DecodeConfig(bytes.NewReader(buff))
			if err != nil {
				return err
			}
			img, err := gopdf.ImageHolderByBytes(buff)
			if err != nil {
				return err
			}
			pageW := 595.28
			pageH := 841.89
			scale := pageW / float64(imgCfg.Width)
			if float64(imgCfg.Height)*scale > pageH {
				scale = pageH / float64(imgCfg.Height)
			}
			imgW := float64(imgCfg.Width) * scale
			imgH := float64(imgCfg.Height) * scale
			pdf.AddPage()
			x := (pageW - imgW) / 2
			y := (pageH - imgH) / 2
			pdf.ImageByHolder(img, x, y, &gopdf.Rect{W: imgW, H: imgH})
		}
		pdfFile := path2.Join(dirPath, utils.ReasonableFileName(ep.Title)+".pdf")
		err = pdf.WritePdf(pdfFile)
		if err != nil {
			return err
		}
	}
	return nil
}

// ---------- 未完成漫画 HTML+JPG ----------

func exportComicJpegsEvenNotFinish(params string) error {
	var paramsStruct struct {
		ComicId string `json:"comicId"`
		Dir     string `json:"dir"`
		Name    string `json:"name"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return err
	}
	// 复用 JPG 导出但跳过完成检查
	return exportComicJpegsInner(paramsStruct.ComicId, paramsStruct.Dir, paramsStruct.Name, false)
}

func exportComicJpegsInner(comicId string, dir string, name string, requireFinished bool) error {
	comic, err := comic_center.FindComicDownloadById(comicId)
	if err != nil {
		return err
	}
	if comic == nil {
		return errors.New("not found")
	}
	if requireFinished && !comic.DownloadFinished {
		return errors.New("not download finish")
	}
	if len(name) == 0 {
		name = fmt.Sprintf("%s-%s", utils.ReasonableFileName(comic.Title), time.Now().Format("2006_01_02_15_04_05.999"))
	}
	dirPath := path2.Join(dir, name)
	ex, err := utils.Exists(dirPath)
	if err != nil {
		return err
	}
	if ex {
		return errors.New("exists")
	}
	err = os.Mkdir(dirPath, utils.CreateDirMode)
	if err != nil {
		return err
	}
	err = os.Mkdir(path2.Join(dirPath, "pictures"), utils.CreateDirMode)
	if err != nil {
		return err
	}
	epList, err := comic_center.ListDownloadEpByComicId(comicId)
	if err != nil {
		return err
	}
	for _, ep := range epList {
		pictures, err := comic_center.ListDownloadPictureByEpId(ep.ID)
		if err != nil {
			return err
		}
		for _, picture := range pictures {
			if !picture.DownloadFinished {
				continue
			}
			notifyExport(fmt.Sprintf("正在导出 EP:%d PIC:%d", ep.EpOrder, picture.RankInEp))
			buff, err := readDownloadFile(picture)
			if err != nil {
				return err
			}
			err = ioutil.WriteFile(
				path2.Join(dirPath, fmt.Sprintf("pictures/%04d_%04d.%s", ep.EpOrder, picture.RankInEp, picture.Format)),
				buff, utils.CreateFileMode,
			)
			if err != nil {
				return err
			}
		}
	}
	if logoBuff, err := readLogoBytes(comic); err == nil && logoBuff != nil {
		ioutil.WriteFile(path2.Join(dirPath, "logo"), logoBuff, utils.CreateFileMode)
	}
	// data.js
	jsonComic, err := buildJsonComicDownload(comicId, false)
	if err != nil {
		return err
	}
	dataBuff, err := json.Marshal(&jsonComic)
	if err != nil {
		return err
	}
	ioutil.WriteFile(path2.Join(dirPath, "data.js"), append([]byte("data = "), dataBuff...), utils.CreateFileMode)
	// HTML
	return ioutil.WriteFile(path2.Join(dirPath, "index.html"), []byte(indexHtml), utils.CreateFileMode)
}

var _ = bytes.NewReader
var _ = strconv.Itoa
var _ = io.Copy
