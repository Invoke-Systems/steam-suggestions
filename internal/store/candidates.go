package store

type CandidateOpts struct {
	OnSale        bool
	PerTag        int
	MaxTotal      int
	MinReviews    int
	MaxReviews    int
	MinPositive   int
	IncludeIDs    []int
	ExcludeIDs    []int
	RequireReview bool
}

type Candidate struct {
	AppID int
	Name  string
}

func (db *DB) RecommendCandidates(tagIDs []int, opt CandidateOpts) []Candidate {
	ids := uniquePositive(tagIDs)
	if len(ids) == 0 {
		return nil
	}
	if opt.PerTag <= 0 {
		opt.PerTag = 700
	}
	if opt.MaxTotal <= 0 {
		opt.MaxTotal = 4000
	}

	seen := map[int]bool{}
	var out []Candidate
	for _, tagID := range ids {
		q := `
			SELECT g.appid, g.name
			FROM game_tags gt
			JOIN games g ON g.appid = gt.appid AND g.tags_ok = 1
			LEFT JOIN price_latest pl ON pl.appid = g.appid
			LEFT JOIN reviews rv ON rv.appid = g.appid
			WHERE gt.tagid = ?
			  AND g.name != ''
			  AND (SELECT COUNT(*) FROM game_tags x WHERE x.appid = g.appid) >= 15
		`
		args := []any{tagID}
		if opt.OnSale {
			q += ` AND pl.discount > 0`
		}
		if opt.RequireReview || opt.MinReviews > 0 || opt.MinPositive > 0 {
			q += ` AND rv.appid IS NOT NULL`
		}
		if opt.MinReviews > 0 {
			q += ` AND rv.total >= ?`
			args = append(args, opt.MinReviews)
		}
		if opt.MaxReviews > 0 {
			q += ` AND rv.total <= ?`
			args = append(args, opt.MaxReviews)
		}
		if opt.MinPositive > 0 {
			q += ` AND rv.positive >= ?`
			args = append(args, opt.MinPositive)
		}
		if len(opt.ExcludeIDs) > 0 {
			q += ` AND NOT EXISTS (SELECT 1 FROM game_tags ex WHERE ex.appid = g.appid AND ex.tagid IN (` + placeholders(len(opt.ExcludeIDs)) + `))`
			args = append(args, asArgs(opt.ExcludeIDs)...)
		}
		if len(opt.IncludeIDs) > 0 {
			q += ` AND (SELECT COUNT(DISTINCT inc.tagid) FROM game_tags inc WHERE inc.appid = g.appid AND inc.tagid IN (` + placeholders(len(opt.IncludeIDs)) + `)) = ?`
			args = append(args, asArgs(opt.IncludeIDs)...)
			args = append(args, len(opt.IncludeIDs))
		}
		q += `
			ORDER BY gt.weight DESC
			LIMIT ?
		`
		args = append(args, opt.PerTag)
		rows, err := db.sql.Query(q, args...)
		if err != nil {
			continue
		}
		for rows.Next() {
			var c Candidate
			if err := rows.Scan(&c.AppID, &c.Name); err != nil {
				continue
			}
			if seen[c.AppID] {
				continue
			}
			seen[c.AppID] = true
			out = append(out, c)
			if len(out) >= opt.MaxTotal {
				rows.Close()
				return out
			}
		}
		_ = rows.Err()
		rows.Close()
	}
	return out
}

func uniquePositive(ids []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
