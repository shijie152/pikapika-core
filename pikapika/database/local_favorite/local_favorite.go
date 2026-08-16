package local_favorite

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

type LocalFavoriteFolder struct {
	ID        string    `gorm:"primarykey" json:"id"`
	Name      string    `gorm:"index:idx_name" json:"name"`
	CreatedAt int64     `json:"createdAt"`
	UpdatedAt int64     `json:"updatedAt"`
	DeletedAt int64     `json:"deletedAt"`
}

type LocalFavoriteComic struct {
	ComicId   string `gorm:"primarykey" json:"comicId"`
	FolderId  string `gorm:"index:idx_folder_id" json:"folderId"`
	Info      string `json:"info"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
	DeletedAt int64  `json:"deletedAt"`
}

func InitDBConnect(databaseDir string) {
	mutex.Lock()
	defer mutex.Unlock()
	var err error
	db, err = gorm.Open(sqlite.Open(path.Join(databaseDir, "local_favorite.db")), utils.GormConfig)
	if err != nil {
		panic("failed to connect database")
	}
	db.AutoMigrate(&LocalFavoriteFolder{})
	db.AutoMigrate(&LocalFavoriteComic{})
}

func AllCustomFolders() ([]string, error) {
	mutex.Lock()
	defer mutex.Unlock()
	var folders []LocalFavoriteFolder
	err := db.Where("deleted_at = 0").Order("created_at ASC").Find(&folders).Error
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(folders))
	for _, f := range folders {
		if f.Name != "" {
			names = append(names, f.Name)
		}
	}
	return names, nil
}

func CreateFolder(name string) (*LocalFavoriteFolder, error) {
	mutex.Lock()
	defer mutex.Unlock()
	now := time.Now().Unix()
	folder := LocalFavoriteFolder{
		ID:        utils.UUID(),
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	err := db.Create(&folder).Error
	if err != nil {
		return nil, err
	}
	return &folder, nil
}

func UpdateFolder(folder *LocalFavoriteFolder) error {
	mutex.Lock()
	defer mutex.Unlock()
	return db.Model(&LocalFavoriteFolder{}).Where("id = ?", folder.ID).Updates(map[string]interface{}{
		"name":       folder.Name,
		"updated_at": time.Now().Unix(),
	}).Error
}

func DeleteFolder(folderId string) error {
	mutex.Lock()
	defer mutex.Unlock()
	now := time.Now().Unix()
	return db.Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&LocalFavoriteFolder{}).Where("id = ?", folderId).Update("deleted_at", now).Error
		if err != nil {
			return err
		}
		return tx.Model(&LocalFavoriteComic{}).Where("folder_id = ?", folderId).Update("deleted_at", now).Error
	})
}

func ListFolders() ([]LocalFavoriteFolder, error) {
	mutex.Lock()
	defer mutex.Unlock()
	var folders []LocalFavoriteFolder
	err := db.Where("deleted_at = 0").Order("created_at ASC").Find(&folders).Error
	if err != nil {
		return nil, err
	}
	return folders, nil
}

func CountFolders() (int64, error) {
	mutex.Lock()
	defer mutex.Unlock()
	var count int64
	err := db.Model(&LocalFavoriteFolder{}).Where("deleted_at = 0").Count(&count).Error
	return count, err
}

func GetFolder(folderId string) (*LocalFavoriteFolder, error) {
	mutex.Lock()
	defer mutex.Unlock()
	var folder LocalFavoriteFolder
	err := db.Where("id = ? AND deleted_at = 0", folderId).First(&folder).Error
	if err != nil {
		return nil, err
	}
	return &folder, nil
}

func AddComic(comicId string, folderId string, info string) error {
	mutex.Lock()
	defer mutex.Unlock()
	now := time.Now().Unix()
	return db.Transaction(func(tx *gorm.DB) error {
		var comic LocalFavoriteComic
		err := tx.Where("comic_id = ? AND deleted_at = 0", comicId).First(&comic).Error
		if err == nil {
			return tx.Model(&LocalFavoriteComic{}).Where("comic_id = ?", comicId).Updates(map[string]interface{}{
				"folder_id":  folderId,
				"info":       info,
				"updated_at": now,
			}).Error
		}
		return tx.Create(&LocalFavoriteComic{
			ComicId:   comicId,
			FolderId:  folderId,
			Info:      info,
			CreatedAt: now,
			UpdatedAt: now,
		}).Error
	})
}

func RemoveComic(comicId string) error {
	mutex.Lock()
	defer mutex.Unlock()
	return db.Model(&LocalFavoriteComic{}).Where("comic_id = ?", comicId).Update("deleted_at", time.Now().Unix()).Error
}

func MoveComics(comicIds []string, folderId string) error {
	mutex.Lock()
	defer mutex.Unlock()
	now := time.Now().Unix()
	return db.Model(&LocalFavoriteComic{}).
		Where("comic_id IN ? AND deleted_at = 0", comicIds).
		Updates(map[string]interface{}{"folder_id": folderId, "updated_at": now}).Error
}

func ListComicsByFolder(folderId string) ([]LocalFavoriteComic, error) {
	mutex.Lock()
	defer mutex.Unlock()
	var comics []LocalFavoriteComic
	err := db.Where("folder_id = ? AND deleted_at = 0", folderId).Order("updated_at DESC").Find(&comics).Error
	if err != nil {
		return nil, err
	}
	return comics, nil
}

func ListAllComics() ([]LocalFavoriteComic, error) {
	mutex.Lock()
	defer mutex.Unlock()
	var comics []LocalFavoriteComic
	err := db.Where("deleted_at = 0").Order("updated_at DESC").Find(&comics).Error
	if err != nil {
		return nil, err
	}
	return comics, nil
}

func GetComic(comicId string) (*LocalFavoriteComic, error) {
	mutex.Lock()
	defer mutex.Unlock()
	var comic LocalFavoriteComic
	err := db.Where("comic_id = ? AND deleted_at = 0", comicId).First(&comic).Error
	if err != nil {
		return nil, err
	}
	return &comic, nil
}

// MergeAll 用远端数据整体替换本地（folder + comic 全量）
func MergeAll(folders []LocalFavoriteFolder, comics []LocalFavoriteComic) error {
	mutex.Lock()
	defer mutex.Unlock()
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM local_favorite_folders").Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM local_favorite_comics").Error; err != nil {
			return err
		}
		if len(folders) > 0 {
			if err := tx.Create(&folders).Error; err != nil {
				return err
			}
		}
		if len(comics) > 0 {
			if err := tx.Create(&comics).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
