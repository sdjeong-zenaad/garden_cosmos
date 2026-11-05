package garden

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

var hashPattern = regexp.MustCompile(`^(?:0x)?[0-9a-fA-F]{64}$`)

func IsAddress(value string) bool   { return addressPattern.MatchString(value) }
func (e Entry) InAddressBook() bool { return IsAddress(e.Value) && (e.AddressBook || e.Nickname != "") }
func (s *Store) Recent(addressBook bool, query string) []Entry {
	query = strings.ToLower(query)
	out := []Entry{}
	for _, e := range s.Entries {
		if addressBook && !e.InAddressBook() {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(e.Value+" "+e.Nickname+" "+e.Note), query) {
			continue
		}
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LastSeen.After(out[j].LastSeen) })
	return out
}
func (s *Store) pruneRecent() {
	count := 0
	for _, e := range s.Entries {
		if !e.AddressBook && e.Nickname == "" && e.Note == "" {
			count++
		}
	}
	for count > 1000 {
		oldest := -1
		for i, e := range s.Entries {
			if e.AddressBook || e.Nickname != "" || e.Note != "" {
				continue
			}
			if oldest < 0 || e.LastSeen.Before(s.Entries[oldest].LastSeen) {
				oldest = i
			}
		}
		if oldest < 0 {
			break
		}
		s.Entries = append(s.Entries[:oldest], s.Entries[oldest+1:]...)
		count--
	}
}
func (s *Store) Touch(value string) {
	for i := range s.Entries {
		if s.Entries[i].Value == value {
			s.Entries[i].LastSeen = time.Now()
			return
		}
	}
}
func (s *Store) Delete(value string) {
	for i, e := range s.Entries {
		if e.Value == value {
			s.Entries = append(s.Entries[:i], s.Entries[i+1:]...)
			return
		}
	}
}

// Edit merges by value and keeps old metadata when a renamed entry collides with an existing value.
func (s *Store) Edit(old string, entry Entry) error {
	entry.Value = strings.TrimSpace(entry.Value)
	entry.Nickname = strings.TrimSpace(entry.Nickname)
	if entry.Value == "" {
		return fmt.Errorf("값을 입력하세요")
	}
	if entry.AddressBook && (!IsAddress(entry.Value) || entry.Nickname == "") {
		return fmt.Errorf("주소록에는 올바른 주소 형식과 별명이 필요합니다")
	}
	if entry.LastSeen.IsZero() {
		entry.LastSeen = time.Now()
	}
	for _, e := range s.Entries {
		if e.Value == entry.Value && e.Value != old {
			if entry.Nickname == "" {
				entry.Nickname = e.Nickname
			}
			if entry.Note == "" {
				entry.Note = e.Note
			}
			entry.AddressBook = entry.AddressBook || e.AddressBook
			if e.LastSeen.After(entry.LastSeen) {
				entry.LastSeen = e.LastSeen
			}
		}
	}
	s.Delete(old)
	s.Delete(entry.Value)
	s.Entries = append(s.Entries, entry)
	s.pruneRecent()
	return nil
}
func (s *Store) CaptureInput(field Field, value string, path []string) {
	if secretKey.MatchString(field.Name) || strings.Contains(strings.ToLower(field.Name), "seed") || Sensitive(path) {
		return
	}
	s.Capture(value, path)
}

// RecentKind identifies fields that benefit from an empty-input recent list.
func RecentKind(f Field) string {
	name := strings.ToLower(strings.ReplaceAll(f.Name, "_", "-"))
	if secretKey.MatchString(name) || strings.Contains(name, "seed") {
		return ""
	}
	if f.Type == "hash" || strings.Contains(name, "hash") || name == "txid" || name == "tx-id" {
		return "hash"
	}
	if f.Type == "address" || strings.Contains(name, "address") || strings.Contains(name, "addr") {
		return "address"
	}
	switch name {
	case "from", "to", "sender", "recipient", "delegator", "validator", "contract":
		return "address"
	}
	return ""
}
func (s *Store) Suggestions(f Field, query string) []Entry {
	if secretKey.MatchString(f.Name) || strings.Contains(strings.ToLower(f.Name), "seed") {
		return nil
	}
	kind := RecentKind(f)
	if query == "" && kind == "" {
		return nil
	}
	var out []Entry
	for _, e := range s.Recent(false, query) {
		if kind == "address" && !IsAddress(e.Value) {
			continue
		}
		if kind == "hash" && (IsAddress(e.Value) || !hashPattern.MatchString(e.Value)) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func (s *Store) Put(value, kind, nickname, note string) {
	for i := range s.Entries {
		if s.Entries[i].Value == value {
			s.Entries[i].LastSeen = time.Now()
			if nickname != "" {
				s.Entries[i].Nickname = nickname
			}
			if note != "" {
				s.Entries[i].Note = note
			}
			return
		}
	}
	s.Entries = append(s.Entries, Entry{Value: value, Kind: kind, Nickname: nickname, Note: note, LastSeen: time.Now()})
	s.pruneRecent()
}
