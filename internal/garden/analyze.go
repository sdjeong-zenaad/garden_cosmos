package garden

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

var commandLine = regexp.MustCompile(`^  ([a-zA-Z0-9][a-zA-Z0-9:_-]*)\s{2,}(.*)$`)
var flagLine = regexp.MustCompile(`^\s+(?:-([^, ]),\s+)?--([\w-]+)(?:[ =]([^ ]+))?\s{2,}(.*)$`)
var defaultValue = regexp.MustCompile(`\(default (.*)\)$`)

func ParseHelp(path []string, help string) *Command {
	c := &Command{Path: append([]string{}, path...), Help: Clean(help)}
	section := ""
	for _, line := range strings.Split(c.Help, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasSuffix(trim, ":") && !strings.HasPrefix(line, " ") {
			section = trim
			continue
		}
		if c.Summary == "" && section == "" && trim != "" {
			c.Summary = trim
		}
		if section == "Usage:" && c.Usage == "" && trim != "" {
			c.Usage = trim
		}
		if strings.Contains(section, "Commands:") {
			if m := commandLine.FindStringSubmatch(line); m != nil && m[1] != "help" && m[1] != "completion" {
				c.Children = append(c.Children, &Command{Path: append(append([]string{}, path...), m[1]), Summary: m[2]})
			}
		}
		if section == "Flags:" || section == "Global Flags:" {
			if m := flagLine.FindStringSubmatch(line + "  "); m != nil {
				f := Flag{Name: m[2], Short: m[1], Type: m[3], Description: strings.TrimSpace(m[4]), Global: section == "Global Flags:"}
				if f.Type == "" {
					f.Type = "bool"
				}
				if d := defaultValue.FindStringSubmatch(f.Description); d != nil {
					f.Default = strings.Trim(d[1], `"`)
				}
				c.Flags = append(c.Flags, f)
			}
		}
	}
	c.Interaction = AnalyzeInteraction(c)
	return c
}
func Identity(binary string) (string, string, error) {
	p, e := exec.LookPath(binary)
	if e != nil {
		return "", "", e
	}
	p, e = filepath.Abs(p)
	if e != nil {
		return "", "", e
	}
	p, e = filepath.EvalSymlinks(p)
	if e != nil {
		return "", "", e
	}
	f, e := os.Open(p)
	if e != nil {
		return "", "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", "", e
	}
	return p, hex.EncodeToString(h.Sum(nil)), nil
}
func Analyze(ctx context.Context, binary, home string, progress func(string)) (*Descriptor, error) {
	p, hash, e := Identity(binary)
	if e != nil {
		return nil, e
	}
	version, err := probeVersion(ctx, p)
	if err != nil {
		return nil, err
	}
	return analyzeBinary(ctx, p, hash, version, home, progress)
}

func probeVersion(ctx context.Context, binary string) (string, error) {
	version, err := Probe(ctx, binary, "version")
	if err != nil {
		version, err = Probe(ctx, binary, "--version")
	}
	if err != nil {
		return "", fmt.Errorf("version: %w", err)
	}
	return strings.TrimSpace(version), nil
}

// Refresh already verified the binary; reuse that identity on a cache miss.
func analyzeBinary(ctx context.Context, p, hash, version, home string, progress func(string)) (*Descriptor, error) {
	d := &Descriptor{Schema: Schema, Binary: p, Fingerprint: hash, Version: version, Home: home, RequestedHome: home, AnalyzedAt: time.Now()}
	count := 0
	var walk func([]string, int) (*Command, error)
	walk = func(path []string, depth int) (*Command, error) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		count++
		if count > 2000 || depth > 12 {
			return nil, fmt.Errorf("command discovery limit reached")
		}
		if progress != nil {
			progress(strings.Join(path, " "))
		}
		args := append(append([]string{}, path...), "--help")
		// Help only: no init, config mutation, queries or transaction execution.
		help, e := Probe(ctx, p, args...)
		if e != nil {
			return nil, e
		}
		c := ParseHelp(path, help)
		if c.Usage == "" {
			return nil, fmt.Errorf("unrecognized Cobra help for %q", strings.Join(path, " "))
		}
		for i, child := range c.Children {
			full, e := walk(child.Path, depth+1)
			if e != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				d.Warnings = append(d.Warnings, strings.Join(child.Path, " ")+": "+e.Error())
				continue
			}
			c.Children[i] = full
		}
		return c, nil
	}
	root, e := walk(nil, 0)
	d.Root = root
	if e != nil {
		return nil, e
	}
	if d.Home == "" {
		for _, f := range d.Root.Flags {
			if f.Name == "home" {
				d.Home = f.Default
			}
		}
	}
	d.Configs = ReadConfigs(d.Home)
	return d, nil
}
func Refresh(ctx context.Context, dir string, p Profile, force bool, progress func(string)) (*Descriptor, error) {
	binary, hash, e := Identity(p.Binary)
	if e != nil {
		return nil, e
	}
	var d Descriptor
	e = Load(filepath.Join(dir, "descriptor.json"), &d)
	version, err := probeVersion(ctx, binary)
	if err != nil {
		return nil, err
	}
	if !force && e == nil && d.Root != nil && d.Schema == Schema && d.Binary == binary && d.Fingerprint == hash && d.Version == version && p.Home == d.RequestedHome {
		d.Configs = ReadConfigs(d.Home)
		return &d, nil
	}
	fresh, e := analyzeBinary(ctx, binary, hash, version, p.Home, progress)
	if e != nil {
		return nil, e
	}
	if e = Save(filepath.Join(dir, "descriptor.json"), fresh); e != nil {
		return nil, e
	}
	return fresh, nil
}

var secretKey = regexp.MustCompile(`(?i)(password|passwd|secret|mnemonic|private|token|credential)`)

func redact(m map[string]any) {
	for k, v := range m {
		if secretKey.MatchString(k) {
			m[k] = "[redacted]"
		} else if child, ok := v.(map[string]any); ok {
			redact(child)
		} else if children, ok := v.([]any); ok {
			for _, child := range children {
				if m, ok := child.(map[string]any); ok {
					redact(m)
				}
			}
		}
	}
}
func ReadConfigs(home string) []Config {
	if home == "" {
		return nil
	}
	var configs []Config
	for _, name := range []string{"client.toml", "config.toml", "app.toml"} {
		p := filepath.Join(home, "config", name)
		b, e := os.ReadFile(p)
		if os.IsNotExist(e) {
			continue
		}
		c := Config{Path: p}
		if e == nil {
			e = toml.Unmarshal(b, &c.Values)
		}
		if e != nil {
			c.Error = e.Error()
		} else {
			redact(c.Values)
		}
		configs = append(configs, c)
	}
	return configs
}

// Flatten preserves descriptor order with one growing slice for the whole tree.
func Flatten(root *Command) []*Command {
	var commands []*Command
	var walk func(*Command)
	walk = func(command *Command) {
		if command == nil {
			return
		}
		commands = append(commands, command)
		for _, child := range command.Children {
			walk(child)
		}
	}
	walk(root)
	return commands
}
