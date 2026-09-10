package steam

import (
	"time"

	"steam-suggestions/internal/store"
)

const TagTTLMs = 14 * 24 * 60 * 60 * 1000

func (c *Client) EnsureTags(db *store.DB, appids []int, names map[int]string) (map[int][]store.NamedTag, error) {
	ids := uniqueIDs(appids)
	if len(ids) == 0 {
		return map[int][]store.NamedTag{}, nil
	}
	if db.Meta("tag_list_at") == "" {
		tags, hash, err := c.TagList()
		if err == nil && len(tags) > 0 {
			_ = db.ReplaceTagDict(tags, hash)
		}
	}
	for _, id := range ids {
		if name := names[id]; name != "" {
			_ = db.UpsertGame(id, name)
		}
	}
	missing := db.StaleAppIDs(ids, TagTTLMs)
	const batch = 40
	for i := 0; i < len(missing); i += batch {
		end := i + batch
		if end > len(missing) {
			end = len(missing)
		}
		chunk := missing[i:end]
		items, err := c.GetItems(chunk)
		if err != nil {
			if _, ok := err.(*RateError); ok {
				time.Sleep(1200 * time.Millisecond)
			}
			continue
		}
		seen := map[int]bool{}
		for _, item := range items {
			seen[item.AppID] = true
			name := item.Name
			if name == "" {
				name = names[item.AppID]
			}
			_ = db.SetGameTags(item.AppID, name, item.Tags, len(item.Tags) > 0)
			PersistReviews(db, item)
			if item.BestPurchaseOption != nil || item.IsFree {
				_ = db.SetPrice(item.AppID, c.PriceFromItem(item, time.Now().UnixMilli()))
			}
		}
		for _, id := range chunk {
			if !seen[id] {
				_ = db.SetGameTags(id, names[id], nil, false)
			}
		}
		if end < len(missing) {
			time.Sleep(120 * time.Millisecond)
		}
	}
	return db.GetTags(ids), nil
}

func uniqueIDs(ids []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
