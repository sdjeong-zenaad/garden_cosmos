package garden

import (
	"encoding/json"
	"regexp"
)

var candidates = regexp.MustCompile(`\b(?:[a-z][a-z0-9]{1,30}1[023456789acdefghjklmnpqrstuvwxyz]{20,90}|0x(?:[0-9a-fA-F]{64}|[0-9a-fA-F]{40})|[0-9a-fA-F]{64})\b`)

func (s *Store) Capture(output string, path []string) {
	// keys list exposes public addresses. Read only its structured address
	// fields; never scan names, pubkeys, or arbitrary key-management output.
	if len(path) == 2 && path[0] == "keys" && path[1] == "list" {
		var keys []struct {
			Address string `json:"address"`
		}
		if json.Unmarshal([]byte(Clean(output)), &keys) == nil {
			for _, key := range keys {
				if IsAddress(key.Address) {
					s.Put(key.Address, "address", "", "")
				}
			}
		}
		return
	}
	if Sensitive(path) {
		return
	}
	for _, v := range candidates.FindAllString(output, -1) {
		kind := "address"
		if !IsAddress(v) && hashPattern.MatchString(v) {
			kind = "hash"
		}
		s.Put(v, kind, "", "")
	}
}
