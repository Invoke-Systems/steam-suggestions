package store

import (
	"os"
	"testing"
)

func TestSearchTagsFarm(t *testing.T) {
	if _, err := os.Stat("../../data/steam.sqlite"); err != nil {
		t.Skip("no sqlite")
	}
	db, err := Open("../../data/steam.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tags := db.SearchTags("farm", 12)
	if len(tags) == 0 {
		t.Fatal("expected farming tags")
	}
	found := false
	for _, tag := range tags {
		if tag.Name == "Farming Sim" || tag.Name == "Farming" {
			found = true
		}
	}
	if !found {
		t.Fatalf("got %#v", tags)
	}
}
