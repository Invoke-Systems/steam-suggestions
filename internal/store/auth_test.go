package store

import (
	"path/filepath"
	"testing"
)

func TestSessionRoundTrip(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "auth.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.UpsertUser(User{SteamID: "76561198000000000", Name: "Tester"}); err != nil {
		t.Fatal(err)
	}
	token, err := db.CreateSession("76561198000000000")
	if err != nil || token == "" {
		t.Fatal(err)
	}
	user, ok := db.SessionUser(token)
	if !ok || user.SteamID != "76561198000000000" || user.Name != "Tester" {
		t.Fatalf("%+v %v", user, ok)
	}
	if _, ok := db.SessionUser("nope"); ok {
		t.Fatal("bogus token")
	}
	db.DeleteSession(token)
	if _, ok := db.SessionUser(token); ok {
		t.Fatal("deleted session still valid")
	}
}

func TestLibraryFocusRoundTrip(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "focus.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.CreateLibraryFocus("not-a-steamid"); err == nil {
		t.Fatal("expected invalid steamid to fail")
	}
	token, err := db.CreateLibraryFocus("76561198000000000")
	if err != nil || token == "" {
		t.Fatal(err)
	}
	id, ok := db.LibraryFocusSteamID(token)
	if !ok || id != "76561198000000000" {
		t.Fatalf("got %q %v", id, ok)
	}
	if _, ok := db.LibraryFocusSteamID("nope"); ok {
		t.Fatal("bogus token")
	}
	db.DeleteLibraryFocus(token)
	if _, ok := db.LibraryFocusSteamID(token); ok {
		t.Fatal("deleted focus still valid")
	}
}

func TestSavedSearchesBelongToUser(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "searches.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.UpsertUser(User{SteamID: "76561198000000000", Name: "A"}); err != nil {
		t.Fatal(err)
	}
	item, err := db.CreateSavedSearch("76561198000000000", "On sale roguelikes", "roguelike, bullet hell", "recs", []byte(`{"onSale":true}`))
	if err != nil || item.ID == 0 {
		t.Fatal(err)
	}
	list, err := db.ListSavedSearches("76561198000000000")
	if err != nil || len(list) != 1 || list[0].Name != "On sale roguelikes" || list[0].Terms == "" || list[0].Kind != "recs" {
		t.Fatalf("%+v %v", list, err)
	}
	if db.DeleteSavedSearch("someone-else", item.ID) {
		t.Fatal("other user deleted a search")
	}
	if !db.DeleteSavedSearch("76561198000000000", item.ID) {
		t.Fatal("owner should delete")
	}
}
