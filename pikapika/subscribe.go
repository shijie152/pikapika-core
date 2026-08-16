package pikapika

import (
	"encoding/json"
	source "github.com/niuhuan/pica-go"
	"pikapika/pikapika/database/comic_center"
)

func comicInfoToSubscribe(comic *source.ComicInfo) comic_center.ComicSubscribe {
	var subscribe comic_center.ComicSubscribe
	subscribe.ID = comic.Id
	subscribe.CreatedAt = comic.CreatedAt
	subscribe.UpdatedAt = comic.UpdatedAt
	subscribe.Title = comic.Title
	subscribe.Author = comic.Author
	subscribe.PagesCount = int32(comic.PagesCount)
	subscribe.EpsCount = int32(comic.EpsCount)
	subscribe.Finished = comic.Finished
	c, _ := json.Marshal(comic.Categories)
	subscribe.Categories = string(c)
	subscribe.ThumbOriginalName = comic.Thumb.OriginalName
	subscribe.ThumbFileServer = comic.Thumb.FileServer
	subscribe.ThumbPath = comic.Thumb.Path
	subscribe.LikesCount = int32(comic.LikesCount)
	subscribe.Description = comic.Description
	subscribe.ChineseTeam = comic.ChineseTeam
	t, _ := json.Marshal(comic.Tags)
	subscribe.Tags = string(t)
	subscribe.AllowDownload = comic.AllowDownload
	subscribe.ViewsCount = int32(comic.ViewsCount)
	subscribe.IsFavourite = comic.IsFavourite
	subscribe.IsLiked = comic.IsLiked
	subscribe.CommentsCount = int32(comic.CommentsCount)
	return subscribe
}

func loadComicInfoRemote(comicId string) (*source.ComicInfo, error) {
	return client.ComicInfo(comicId)
}

func addSubscribed(comicId string) error {
	comic, err := loadComicInfoRemote(comicId)
	if err != nil {
		return err
	}
	subscribe := comicInfoToSubscribe(comic)
	return comic_center.Subscribe(&subscribe)
}

func removeSubscribed(comicId string) error {
	return comic_center.Unsubscribe(comicId)
}

func allSubscribed() (string, error) {
	subscribes, err := comic_center.AllSubscribes()
	if err != nil {
		return "", err
	}
	return serialize(subscribes, nil)
}

func loadSubscribed(comicId string) (string, error) {
	subscribe, err := comic_center.LoadSubscribe(comicId)
	if err != nil {
		return "", nil
	}
	if subscribe == nil {
		return "", nil
	}
	return serialize(subscribe, nil)
}

func removeAllSubscribed() error {
	return comic_center.RemoveAllSubscribes()
}

// updateSubscribed 检查全部订阅的更新情况
func updateSubscribed() error {
	return updateSubscribesAll(false)
}

// updateSubscribedForce 强制更新全部订阅
func updateSubscribedForce() error {
	return updateSubscribesAll(true)
}

func updateSubscribesAll(force bool) error {
	subscribes, err := comic_center.AllSubscribes()
	if err != nil {
		return err
	}
	for i := range subscribes {
		subscribe := &subscribes[i]
		if !force && subscribe.NewEpCount > 0 {
			// 已提示过新章节的订阅, 非强制更新时跳过
			continue
		}
		comic, err := loadComicInfoRemote(subscribe.ID)
		if err != nil {
			continue
		}
		updated := comicInfoToSubscribe(comic)
		updated.SubscribeTime = subscribe.SubscribeTime
		if subscribe.EpsCount > 0 && int32(comic.EpsCount) > subscribe.EpsCount {
			updated.NewEpCount = int32(comic.EpsCount) - subscribe.EpsCount
		}
		err = comic_center.Subscribe(&updated)
		if err != nil {
			return err
		}
	}
	return nil
}
