package pikapika

import (
	"encoding/json"
	"pikapika/pikapika/database/local_favorite"
)

func allCustomFolders() (string, error) {
	names, err := local_favorite.AllCustomFolders()
	if err != nil {
		return "", err
	}
	return serialize(names, nil)
}

func createLocalFavoriteFolder(name string) (string, error) {
	folder, err := local_favorite.CreateFolder(name)
	if err != nil {
		return "", err
	}
	return serialize(folder, nil)
}

func updateLocalFavoriteFolder(params string) error {
	var folder local_favorite.LocalFavoriteFolder
	err := json.Unmarshal([]byte(params), &folder)
	if err != nil {
		return err
	}
	return local_favorite.UpdateFolder(&folder)
}

func deleteLocalFavoriteFolder(folderId string) error {
	return local_favorite.DeleteFolder(folderId)
}

func listLocalFavoriteFolders() (string, error) {
	folders, err := local_favorite.ListFolders()
	if err != nil {
		return "", err
	}
	return serialize(folders, nil)
}

func countLocalFavoriteFolders() (string, error) {
	count, err := local_favorite.CountFolders()
	if err != nil {
		return "", err
	}
	return serialize(count, nil)
}

func getLocalFavoriteFolder(folderId string) (string, error) {
	folder, err := local_favorite.GetFolder(folderId)
	if err != nil {
		return "", err
	}
	return serialize(folder, nil)
}

func listLocalFavoriteComics(params string) (string, error) {
	var paramsStruct struct {
		FolderId string `json:"folderId"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return "", err
	}
	comics, err := local_favorite.ListComicsByFolder(paramsStruct.FolderId)
	if err != nil {
		return "", err
	}
	return serialize(comics, nil)
}

func listAllLocalFavoriteComics() (string, error) {
	comics, err := local_favorite.ListAllComics()
	if err != nil {
		return "", err
	}
	return serialize(comics, nil)
}

func addLocalFavoriteComic(params string) error {
	var paramsStruct struct {
		ComicId  string `json:"comicId"`
		FolderId string `json:"folderId"`
		Info     string `json:"info"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return err
	}
	return local_favorite.AddComic(paramsStruct.ComicId, paramsStruct.FolderId, paramsStruct.Info)
}

func removeLocalFavoriteComic(comicId string) error {
	return local_favorite.RemoveComic(comicId)
}

func moveLocalFavoriteComics(params string) error {
	var paramsStruct struct {
		ComicIds []string `json:"comicIds"`
		FolderId string   `json:"folderId"`
	}
	err := json.Unmarshal([]byte(params), &paramsStruct)
	if err != nil {
		return err
	}
	return local_favorite.MoveComics(paramsStruct.ComicIds, paramsStruct.FolderId)
}
