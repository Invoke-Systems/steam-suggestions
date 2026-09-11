package steam

import "testing"

func TestParseGamesByConcurrentPlayers(t *testing.T) {
	got, err := parseGamesByConcurrentPlayers([]byte(`{
		"response": {
			"last_update": 1,
			"ranks": [
				{"rank": 1, "appid": 730, "concurrent_in_game": 500000, "peak_in_game": 1200000},
				{"rank": 2, "appid": 570, "concurrent_in_game": 400000, "peak_in_game": 800000},
				{"rank": 3, "appid": 0, "concurrent_in_game": 1, "peak_in_game": 1}
			]
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].AppID != 730 || got[0].PeakInGame != 1200000 {
		t.Fatalf("%+v", got)
	}
}
