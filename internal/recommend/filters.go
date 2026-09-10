package recommend

import (
	"regexp"
	"sort"
	"strings"
)

var junkTitle = regexp.MustCompile(`(?i)(?:\bdemo\b|\bplaytest\b|\bsoundtrack\b|\boriginal soundtrack\b|\bdedicated server\b|\bsdk\b|\bmod tools?\b|\bdeveloper tools\b|\bpublic test\b|\bbeta branch\b|\btrailer\b|\bbenchmark\b|\bdlc\b)`)

var adultTags = map[string]bool{
	"Sexual Content": true,
	"Nudity":         true,
	"Hentai":         true,
	"NSFW":           true,
}

var goreTags = map[string]bool{
	"Gore": true,
}

func IsJunkName(name string) bool {
	return junkTitle.MatchString(name)
}

func HasAdultTags(tags []Tag) bool {
	for _, tag := range tags {
		if adultTags[tag.Name] {
			return true
		}
	}
	return false
}

func HasGoreTags(tags []Tag) bool {
	for _, tag := range tags {
		if goreTags[tag.Name] {
			return true
		}
	}
	return false
}

func tagNameSet(tags []Tag) map[string]bool {
	out := map[string]bool{}
	for _, tag := range tags {
		if name := strings.ToLower(strings.TrimSpace(tag.Name)); name != "" {
			out[name] = true
		}
	}
	return out
}

func HasAllTagNames(tags []Tag, names []string) bool {
	have := tagNameSet(tags)
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		if !have[name] {
			return false
		}
	}
	return true
}

func HasAnyTagName(tags []Tag, names []string) bool {
	have := tagNameSet(tags)
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" && have[name] {
			return true
		}
	}
	return false
}

func NormalizeTagWeights(weights []TagWeight) []TagWeight {
	seen := map[string]int{}
	var out []TagWeight
	for _, item := range weights {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		weight := item.Weight
		if weight > 100 {
			weight = 100
		}
		if weight < -100 {
			weight = -100
		}
		if weight == 0 {
			continue
		}
		key := strings.ToLower(name)
		if i, ok := seen[key]; ok {
			out[i].Weight = weight
			continue
		}
		seen[key] = len(out)
		out = append(out, TagWeight{Name: name, Weight: weight})
	}
	return out
}

func tagPrefScore(tags []Tag, weights []TagWeight) float64 {
	if len(weights) == 0 {
		return 0
	}
	have := tagNameSet(tags)
	sum := 0.0
	for _, item := range weights {
		if have[strings.ToLower(strings.TrimSpace(item.Name))] {
			sum += float64(item.Weight)
		}
	}
	return sum
}

func MaxTagWeight(tags []Tag) int {
	max := 0
	for _, tag := range tags {
		if tag.Weight > max {
			max = tag.Weight
		}
	}
	return max
}

func DistinctiveTagIDs(games []Game, limit int) []int {
	if limit <= 0 {
		limit = 10
	}
	type played struct {
		game   Game
		weight float64
	}
	var ranked []played
	for _, game := range games {
		if game.Hours < tasteHourFloor {
			continue
		}
		weight := game.Hours
		if game.RecentlyPlayed {
			weight *= 2.4
		}
		ranked = append(ranked, played{game, weight})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].weight > ranked[j].weight })
	if len(ranked) > 12 {
		ranked = ranked[:12]
	}

	seen := map[int]bool{}
	var out []int
	for _, row := range ranked {
		tags := append([]Tag(nil), row.game.Tags...)
		sort.Slice(tags, func(i, j int) bool { return tags[i].Weight > tags[j].Weight })
		picked := 0
		for _, tag := range tags {
			if tag.TagID <= 0 || seen[tag.TagID] || TagSpecificity(tag.Name) < 0.5 {
				continue
			}
			seen[tag.TagID] = true
			out = append(out, tag.TagID)
			picked++
			if picked >= 2 {
				break
			}
		}
		if len(out) >= limit {
			break
		}
	}
	return out
}
