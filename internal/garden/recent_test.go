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

func TestTypedSuggestionsAndPinnedHash(t *testing.T) {
	a := "0x" + strings.Repeat("a", 40)
	h := strings.Repeat("b", 64)
	newer := "0x" + strings.Repeat("c", 40)
	s := Store{}
	s.Capture(a+" "+h+" "+newer, nil)
	if err := s.Edit(h, Entry{Value: h, Kind: "hash", Nickname: "입금 거래", Note: "memo"}); err != nil {
		t.Fatal(err)
	}
	got := s.Suggestions(Field{Name: "to_address"}, "")
	if len(got) != 2 || got[0].Value != newer {
		t.Fatal(got)
	}
	got = s.Suggestions(Field{Name: "txhash"}, "거래")
	if len(got) != 1 || got[0].Value != h {
		t.Fatal(got)
	}
	if len(s.Suggestions(Field{Name: "amount"}, "")) != 0 || len(s.Suggestions(Field{Name: "private-key"}, "b")) != 0 {
		t.Fatal("unexpected suggestions")
	}
	for i := 0; i < 1005; i++ {
		s.Put(fmt.Sprintf("%064x", i), "hash", "", "")
	}
	if len(s.Recent(false, "입금 거래")) != 1 {
		t.Fatal("pinned hash evicted")
	}
	path := filepath.Join(t.TempDir(), "cache.json")
	if err := Save(path, s); err != nil {
		t.Fatal(err)
	}
	var restored Store
	if err := Load(path, &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Suggestions(Field{Name: "tx_hash"}, "입금")) != 1 {
		t.Fatal("pin lost on reload")
	}
}

func TestKeysListCapturesOnlyPublicAddresses(t *testing.T) {
	const first = "xpla1zapp6g36m0j7gcsmr2n8l6yhmgw3es2pcte3tz"
	const second = "xpla1dt9m42z6gc0qsw4qzyx78guwmxxs4yzf5gc9v5"
	output := `[
 {"name":"conx-oper","type":"local","address":"` + first + `","pubkey":"{\"@type\":\"/cosmos.evm.crypto.v1.ethsecp256k1.PubKey\",\"key\":\"Am3d2DUt4tptQ97tMvPqcAca7LXUpbGsSWmEiHuFUqe1\"}"},
 {"name":"zena-dev0","type":"local","address":"` + second + `","pubkey":"{\"@type\":\"/cosmos.evm.crypto.v1.ethsecp256k1.PubKey\",\"key\":\"A9n0mmX3ZYGE+KEL9/FkddpwQJKSmDBZ/EBXDtqy5STV\"}"}
 ]`
	s := Store{}
	s.Capture(output, []string{"keys", "list"})
	if len(s.Entries) != 2 || s.Entries[0].Value != first || s.Entries[1].Value != second {
		t.Fatalf("public addresses missing: %+v", s.Entries)
	}
	s.Entries[0].Nickname = "운영"
	s.Capture(output, []string{"keys", "list"})
	if len(s.Entries) != 2 || s.Entries[0].Nickname != "운영" {
		t.Fatal("lost alias or duplicated address")
	}
	// Strings resembling addresses or hashes outside address fields stay excluded.
	s.Capture(fmt.Sprintf(`[{"address":"invalid","name":%q,"pubkey":%q,"private_key":%q}]`, "0x"+strings.Repeat("e", 40), strings.Repeat("f", 64), strings.Repeat("a", 64)), []string{"keys", "list"})
	s.Capture("invalid JSON "+"0x"+strings.Repeat("b", 40), []string{"keys", "list"})
	if len(s.Entries) != 2 {
		t.Fatal("captured non-address key data")
	}
	for _, path := range [][]string{{"keys", "add"}, {"keys", "export"}, {"keys", "list", "extra"}} {
		var excluded Store
		excluded.Capture(output, path)
		if len(excluded.Entries) != 0 {
			t.Fatal("sensitive command cached", path)
		}
	}
	path := filepath.Join(t.TempDir(), "bookmarks.json")
	if err := Save(path, s); err != nil {
		t.Fatal(err)
	}
	var restored Store
	if err := Load(path, &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Suggestions(Field{Name: "to_address"}, "")) != 2 {
		t.Fatal("addresses not available after reload")
	}
}
