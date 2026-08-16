package comic_center

import (
	"gorm.io/gorm"
	"time"
)

type ComicSubscribe struct {
	ID                  string    `gorm:"primarykey" json:"id"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
	Title               string    `json:"title"`
	Author              string    `json:"author"`
	PagesCount          int32     `json:"pagesCount"`
	EpsCount            int32     `json:"epsCount"`
	Finished            bool      `json:"finished"`
	Categories          string    `json:"categories"`
	ThumbOriginalName   string    `json:"thumbOriginalName"`
	ThumbFileServer     string    `json:"thumbFileServer"`
	ThumbPath           string    `json:"thumbPath"`
	LikesCount          int32     `json:"likesCount"`
	Description         string    `json:"description"`
	ChineseTeam         string    `json:"chineseTeam"`
	Tags                string    `json:"tags"`
	AllowDownload       bool      `json:"allowDownload"`
	ViewsCount          int32     `json:"viewsCount"`
	IsFavourite         bool      `json:"isFavourite"`
	IsLiked             bool      `json:"isLiked"`
	CommentsCount       int32     `json:"commentsCount"`
	SubscribeTime       time.Time `gorm:"index:idx_subscribe_time" json:"subscribeTime"`
	UpdateSubscribeTime time.Time `gorm:"index:idx_update_subscribe_time" json:"updateSubscribeTime"`
	NewEpCount          int32     `json:"newEpCount"`
}

func Subscribe(comic *ComicSubscribe) error {
	mutex.Lock()
	defer mutex.Unlock()
	now := time.Now()
	return db.Transaction(func(tx *gorm.DB) error {
		var exist ComicSubscribe
		err := tx.Where("id = ?", comic.ID).First(&exist).Error
		if err == nil {
			// 已订阅, 更新漫画信息, 记录新增章节数
			newEpCount := int32(0)
			if exist.EpsCount < comic.EpsCount {
				newEpCount = comic.EpsCount - exist.EpsCount
			}
			comic.SubscribeTime = exist.SubscribeTime
			comic.UpdateSubscribeTime = now
			comic.NewEpCount = newEpCount
			comic.CreatedAt = exist.CreatedAt
			comic.UpdatedAt = now
			return tx.Save(comic).Error
		}
		comic.SubscribeTime = now
		comic.UpdateSubscribeTime = now
		comic.NewEpCount = 0
		comic.CreatedAt = now
		comic.UpdatedAt = now
		return tx.Create(comic).Error
	})
}

func Unsubscribe(comicId string) error {
	mutex.Lock()
	defer mutex.Unlock()
	return db.Where("id = ?", comicId).Delete(&ComicSubscribe{}).Error
}

func AllSubscribes() ([]ComicSubscribe, error) {
	mutex.Lock()
	defer mutex.Unlock()
	var subscribes []ComicSubscribe
	err := db.Order("update_subscribe_time DESC").Find(&subscribes).Error
	if err != nil {
		return nil, err
	}
	return subscribes, nil
}

func LoadSubscribe(comicId string) (*ComicSubscribe, error) {
	mutex.Lock()
	defer mutex.Unlock()
	var subscribe ComicSubscribe
	err := db.Where("id = ?", comicId).First(&subscribe).Error
	if err != nil {
		return nil, err
	}
	return &subscribe, nil
}

func RemoveAllSubscribes() error {
	mutex.Lock()
	defer mutex.Unlock()
	return db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ComicSubscribe{}).Error
}
