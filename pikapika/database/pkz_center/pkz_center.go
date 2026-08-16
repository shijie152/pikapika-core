package pkz_center

import (
	"path"
	"pikapika/pikapika/utils"
	"sync"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var mutex = sync.Mutex{}
var db *gorm.DB

type PkzViewLog struct {
	FileName       string    `gorm:"primarykey" json:"fileName"`
	FilePath       string    `json:"filePath"`
	LastViewComicId string   `json:"lastViewComicId"`
	LastViewComicTitle string `json:"lastViewComicTitle"`
	LastViewTime   time.Time `json:"lastViewTime"`
}

type PkzComicViewLog struct {
	FileName              string    `gorm:"primarykey" json:"fileName"`
	LastViewComicId       string    `gorm:"primarykey" json:"lastViewComicId"`
	FilePath              string    `json:"filePath"`
	LastViewComicTitle    string    `json:"lastViewComicTitle"`
	LastViewEpId          string    `json:"lastViewEpId"`
	LastViewEpName        string    `json:"lastViewEpName"`
	LastViewPictureRank   int       `json:"lastViewPictureRank"`
	LastViewTime          time.Time `json:"lastViewTime"`
}

func InitDBConnect(databaseDir string) {
	mutex.Lock()
	defer mutex.Unlock()
	var err error
	db, err = gorm.Open(sqlite.Open(path.Join(databaseDir, "pkz_center.db")), utils.GormConfig)
	if err != nil {
		panic("failed to connect database")
	}
	db.AutoMigrate(&PkzViewLog{})
	db.AutoMigrate(&PkzComicViewLog{})
}

func ViewPkz(fileName string, filePath string) error {
	mutex.Lock()
	defer mutex.Unlock()
	now := time.Now()
	return db.Transaction(func(tx *gorm.DB) error {
		var log PkzViewLog
		err := tx.Where("file_name = ?", fileName).First(&log).Error
		if err != nil {
			return tx.Create(&PkzViewLog{
				FileName:     fileName,
				FilePath:     filePath,
				LastViewTime: now,
			}).Error
		}
		return tx.Model(&PkzViewLog{}).Where("file_name = ?", fileName).Updates(map[string]interface{}{
			"file_path":       filePath,
			"last_view_time":  now,
		}).Error
	})
}

func ViewPkzComic(fileName string, filePath string, comicId string, comicTitle string) error {
	mutex.Lock()
	defer mutex.Unlock()
	now := time.Now()
	return db.Transaction(func(tx *gorm.DB) error {
		// 更新文件级日志
		var log PkzViewLog
		err := tx.Where("file_name = ?", fileName).First(&log).Error
		if err != nil {
			tx.Create(&PkzViewLog{FileName: fileName, FilePath: filePath, LastViewTime: now})
		} else {
			tx.Model(&PkzViewLog{}).Where("file_name = ?", fileName).Updates(map[string]interface{}{
				"last_view_comic_id": comicId, "last_view_comic_title": comicTitle, "last_view_time": now,
			})
		}
		// 漫画级日志
		var comicLog PkzComicViewLog
		err = tx.Where("file_name = ? AND last_view_comic_id = ?", fileName, comicId).First(&comicLog).Error
		if err != nil {
			return tx.Create(&PkzComicViewLog{
				FileName: fileName, FilePath: filePath,
				LastViewComicId: comicId, LastViewComicTitle: comicTitle,
				LastViewTime: now,
			}).Error
		}
		return tx.Model(&PkzComicViewLog{}).
			Where("file_name = ? AND last_view_comic_id = ?", fileName, comicId).
			Updates(map[string]interface{}{"last_view_time": now}).Error
	})
}

func ViewPkzEpAndPicture(fileName string, comicId string, comicTitle string, epId string, epName string, pictureRank int) error {
	mutex.Lock()
	defer mutex.Unlock()
	now := time.Now()
	return db.Model(&PkzComicViewLog{}).
		Where("file_name = ? AND last_view_comic_id = ?", fileName, comicId).
		Updates(map[string]interface{}{
			"last_view_ep_id": epId, "last_view_ep_name": epName,
			"last_view_picture_rank": pictureRank, "last_view_time": now,
			"last_view_comic_title": comicTitle,
		}).Error
}

func PkzComicViewLogs(fileName string) ([]PkzComicViewLog, error) {
	mutex.Lock()
	defer mutex.Unlock()
	var logs []PkzComicViewLog
	err := db.Where("file_name = ?", fileName).Order("last_view_time DESC").Find(&logs).Error
	if err != nil {
		return nil, err
	}
	return logs, nil
}

func PkzComicViewLogByPkzNameAndId(fileName string, comicId string) (*PkzComicViewLog, error) {
	mutex.Lock()
	defer mutex.Unlock()
	var log PkzComicViewLog
	err := db.Where("file_name = ? AND last_view_comic_id = ?", fileName, comicId).First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}
