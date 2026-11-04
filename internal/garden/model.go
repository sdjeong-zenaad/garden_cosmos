package garden

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	json5 "github.com/titanous/json5"
)

const Schema = 2

type Flag struct {
	Name, Short, Type, Default, Description string
	Global                                  bool
}
type Command struct {
	Path                 []string
	Summary, Usage, Help string
	Flags                []Flag
	Children             []*Command
}
type Config struct {
	Path   string
	Values map[string]any
	Error  string
}
type Descriptor struct {
	Schema                             int
	Binary, Version, Fingerprint, Home string
	RequestedHome                      string
	AnalyzedAt                         time.Time
	Root                               *Command
	Configs                            []Config
	Warnings                           []string
}
type Profile struct{ Binary, Home string }
type Entry struct {
	Value, Kind, Nickname, Note string
	LastSeen                    time.Time
	AddressBook                 bool
}
type Store struct{ Entries []Entry }

func DataDir() (string, error) {
	d := os.Getenv("XDG_DATA_HOME")
	if d == "" {
		h, e := os.UserHomeDir()
		if e != nil {
			return "", e
		}
		d = filepath.Join(h, ".local", "share")
	}
	return filepath.Join(d, "garden_cosmos"), nil
}
func Save(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".garden-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(f.Name(), path)
}
func Load(path string, v any) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	if e = json5.NewDecoder(f).Decode(v); e != nil {
		return fmt.Errorf("%s: %w", path, e)
	}
	return nil
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
