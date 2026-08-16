package pikapika

import (
	"os"
	"sync"
	"testing"
	comic_center2 "pikapika/pikapika/database/comic_center"
)

func setupClaimTest(t *testing.T, count int) []comic_center2.ComicDownload {
	dir, err := os.MkdirTemp("", "claim-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	comic_center2.InitDBConnect(dir)
	var list []comic_center2.ComicDownload
	for i := 0; i < count; i++ {
		c := comic_center2.ComicDownload{}
		c.ID = string(rune('a' + i)) + "-comic"
		c.Title = c.ID
		if err := comic_center2.CreateDownload(&c, &[]comic_center2.ComicDownloadEp{}); err != nil {
			t.Fatal(err)
		}
		list = append(list, c)
	}
	return list
}

func resetClaimState() {
	claimedComics = map[string]bool{}
	pendingComics = nil
	pendingIndex = 0
}

func TestClaimComicExclusive(t *testing.T) {
	comics := setupClaimTest(t, 3)
	resetClaimState()

	var mu sync.Mutex
	claimed := map[string]int{}
	workers := 4
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := &downloadWorker{}
			for j := 0; j < 30; j++ {
				c := w.claimComic()
				if c == nil {
					break
				}
				mu.Lock()
				claimed[c.ID]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(claimed) != len(comics) {
		t.Fatalf("expected %d distinct comics claimed, got %d: %v", len(comics), len(claimed), claimed)
	}
	for _, c := range comics {
		if claimed[c.ID] != 1 {
			t.Errorf("comic %s claimed %d times, want 1", c.ID, claimed[c.ID])
		}
	}
}

func TestClaimComicReleaseReclaim(t *testing.T) {
	comics := setupClaimTest(t, 1)
	resetClaimState()

	w := &downloadWorker{}
	c := w.claimComic()
	w.comic = c
	if c == nil || c.ID != comics[0].ID {
		t.Fatalf("first claim failed: %v", c)
	}
	// 未释放前, 其他 worker 不能认领
	w2 := &downloadWorker{}
	if c2 := w2.claimComic(); c2 != nil {
		t.Fatalf("second worker claimed while still held: %v", c2)
	}
	// 释放后可再次认领
	if c3 := w.claimComic(); c3 == nil || c3.ID != comics[0].ID {
		t.Fatalf("reclaim after release failed: %v", c3)
	}
}
