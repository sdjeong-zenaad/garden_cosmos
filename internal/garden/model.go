package garden

import (
	"time"
)

const Schema = 3

type Flag struct {
	Name, Short, Type, Default, Description string
	Global                                  bool
}
type Command struct {
	Path                 []string
	Summary, Usage, Help string
	Flags                []Flag
	Children             []*Command
	Interaction          Interaction `json:"interaction"`
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
