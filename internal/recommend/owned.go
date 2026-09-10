package recommend

import (
	"sort"
)

// OwnedPickMaxMinutes is the playtime ceiling for "in library, still fair game" picks.
const OwnedPickMaxMinutes = 120

// RankOwnedFits ranks owned games (already filtered, e.g. under 2 hours) by taste fit
// from the player's library. Unlike Recommend, it does not drop owned titles.
func RankOwnedFits(library []Game, candidates []Game, limit int) []Card {
	if limit <= 0 {
		limit = 36
	}
	if len(candidates) == 0 {
		return nil
	}
	taste := BuildTaste(library)
	if len(taste.Vec) == 0 {
		return nil
	}
	opts := Options{}
	out := make([]Card, 0, len(candidates))
	for _, item := range candidates {
		if item.AppID <= 0 || IsJunkName(item.Name) {
			continue
		}
		out = append(out, scoreItem(item, taste, opts))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].familiarRaw != out[j].familiarRaw {
			return out[i].familiarRaw > out[j].familiarRaw
		}
		return out[i].AppID < out[j].AppID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
