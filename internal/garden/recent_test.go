package garden

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecentOrderingAndLegacyBook(t *testing.T) {
	a := "0x" + strings.Repeat("a", 40)
	b := strings.Repeat("B", 64)
	s := Store{Entries: []Entry{{Value: a, Kind: "manual", Nickname: "거래소", LastSeen: time.Unix(1, 0)}, {Value: b, Kind: "hash", LastSeen: time.Unix(2, 0)}}}
	if got := s.Recent(false, ""); got[0].Value != b {
		t.Fatal(got)
	}
	if got := s.Recent(true, "거래"); len(got) != 1 || got[0].Value != a {
		t.Fatal(got)
	}
	s.Capture(a, nil)
	if s.Recent(false, "")[0].Value != a || s.Entries[0].Nickname != "거래소" {
		t.Fatal(s)
	}
	p := filepath.Join(t.TempDir(), "bookmarks.json")
	if e := Save(p, s); e != nil {
		t.Fatal(e)
	}
	var restored Store
	if e := Load(p, &restored); e != nil {
		t.Fatal(e)
	}
	if len(restored.Recent(true, "")) != 1 {
		t.Fatal(restored)
	}
}
func TestRecentLRUEviction(t *testing.T) {
	var s Store
	for i := 0; i < 1000; i++ {
		s.Put(fmt.Sprint(i), "hash", "", "")
	}
	s.Touch("0")
	s.Put("new", "hash", "", "")
	found := map[string]bool{}
	for _, e := range s.Entries {
		found[e.Value] = true
	}
	if !found["0"] || found["1"] || len(s.Entries) != 1000 {
		t.Fatal("evicted recently used entry")
	}
	s.Put("saved", "address", "별명", "")
	s.Put("new2", "hash", "", "")
	if len(s.Recent(false, "별명")) != 1 {
		t.Fatal("lost annotated entry")
	}
}
func TestInputCaptureSensitiveAndDisabled(t *testing.T) {
	var s Store
	v := strings.Repeat("a", 64)
	s.CaptureInput(Field{Name: "password"}, v, nil)
	s.CaptureInput(Field{Name: "seed"}, v, nil)
	s.CaptureInput(Field{Name: "value"}, v, []string{"keys", "add"})
	if len(s.Entries) != 0 {
		t.Fatal("sensitive input captured")
	}
	s.CaptureInput(Field{Name: "txhash"}, v, []string{"query"})
	if len(s.Entries) != 1 {
		t.Fatal(s)
	}
}
func TestAddressEditMergesAndClears(t *testing.T) {
	a := "0x" + strings.Repeat("a", 40)
	b := "0x" + strings.Repeat("b", 40)
	s := Store{}
	s.Put(a, "address", "first", "")
	s.Put(b, "address", "second", "")
	if e := s.Edit(a, Entry{Value: b, Nickname: "renamed", AddressBook: true}); e != nil {
		t.Fatal(e)
	}
	if len(s.Entries) != 1 || len(s.Recent(true, "renamed")) != 1 {
		t.Fatal(s)
	}
	if e := s.Edit(b, Entry{Value: b}); e != nil {
		t.Fatal(e)
	}
	if len(s.Recent(true, "")) != 0 || len(s.Entries) != 1 {
		t.Fatal(s)
	}
	if e := s.Edit("", Entry{Value: "not an address", Nickname: "bad", AddressBook: true}); e == nil {
		t.Fatal("invalid address allowed")
	}
}
