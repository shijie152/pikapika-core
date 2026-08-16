package pikapika

import (
	"encoding/base64"
	"encoding/json"
	"pikapika/pikapika/database/pkz_center"

	"github.com/niuhuan/pkz-go"
)

func pkzInfo(pkzPath string) (string, error) {
	archive, err := pkz.ReadPkzArchive(pkzPath)
	if err != nil {
		return "", err
	}
	return serialize(archive, nil)
}

func loadPkzFile(params string) (string, error) {
	var paramsStruct struct {
		PkzPath string `json:"pkzPath"`
		Path    string `json:"path"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return "", err
	}
	buff, err := pkz.ReadPkzPath(paramsStruct.PkzPath, paramsStruct.Path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buff), nil
}

func viewPkz(params string) error {
	var paramsStruct struct {
		FileName string `json:"fileName"`
		FilePath string `json:"filePath"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return err
	}
	return pkz_center.ViewPkz(paramsStruct.FileName, paramsStruct.FilePath)
}

func viewPkzComic(params string) error {
	var paramsStruct struct {
		FileName    string `json:"fileName"`
		FilePath    string `json:"filePath"`
		ComicId     string `json:"comicId"`
		ComicTitle  string `json:"comicTitle"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return err
	}
	return pkz_center.ViewPkzComic(paramsStruct.FileName, paramsStruct.FilePath, paramsStruct.ComicId, paramsStruct.ComicTitle)
}

func viewPkzEpAndPicture(params string) error {
	var paramsStruct struct {
		FileName    string `json:"fileName"`
		ComicId     string `json:"comicId"`
		ComicTitle  string `json:"comicTitle"`
		EpId        string `json:"epId"`
		EpName      string `json:"epName"`
		PictureRank int    `json:"pictureRank"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return err
	}
	return pkz_center.ViewPkzEpAndPicture(
		paramsStruct.FileName,
		paramsStruct.ComicId,
		paramsStruct.ComicTitle,
		paramsStruct.EpId,
		paramsStruct.EpName,
		paramsStruct.PictureRank,
	)
}

func pkzComicViewLogs(fileName string) (string, error) {
	logs, err := pkz_center.PkzComicViewLogs(fileName)
	if err != nil {
		return "", err
	}
	return serialize(logs, nil)
}

func pkzComicViewLogByPkzNameAndId(params string) (string, error) {
	var paramsStruct struct {
		FileName string `json:"fileName"`
		ComicId  string `json:"comicId"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return "", err
	}
	log, err := pkz_center.PkzComicViewLogByPkzNameAndId(paramsStruct.FileName, paramsStruct.ComicId)
	if err != nil {
		return "", nil
	}
	if log == nil {
		return "", nil
	}
	return serialize(log, nil)
}
