package garden

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	json5 "github.com/titanous/json5"
)

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
