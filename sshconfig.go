package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Host is a concrete Host entry of an ssh config: an alias ssh connects to
// by name, with the settings shown beside it.
type Host struct {
	Alias    string
	HostName string
	User     string
	Port     string
}

// Address returns user@hostname:port as far as the config sets them.
func (h Host) Address() string {
	s := h.HostName
	if s == "" {
		s = h.Alias
	}
	if h.User != "" {
		s = h.User + "@" + s
	}
	if h.Port != "" && h.Port != "22" {
		s += ":" + h.Port
	}
	return s
}

// defaultSSHConfig returns the path of the user's ssh config.
func defaultSSHConfig() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "config")
}

// loadHosts reads the hosts of the ssh config at path, following its
// Include directives. Wildcard patterns are skipped, as they name no host
// to connect to.
func loadHosts(path string) ([]Host, error) {
	p := &configParser{dir: filepath.Dir(path), seen: map[string]bool{}, index: map[string]int{}}
	if err := p.parseFile(path); err != nil {
		return nil, err
	}
	return p.hosts, nil
}

type configParser struct {
	dir   string // relative Include paths resolve against ~/.ssh
	seen  map[string]bool
	hosts []Host
	index map[string]int // alias → position in hosts
	block []int          // the hosts the current Host line names
}

func (p *configParser) parseFile(path string) error {
	if p.seen[path] {
		return nil
	}
	p.seen[path] = true
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		key, args := splitDirective(sc.Text())
		switch key {
		case "":
		case "include":
			for _, pattern := range args {
				p.include(pattern)
			}
		case "host":
			p.block = nil
			for _, alias := range args {
				if strings.ContainsAny(alias, "*?!") {
					continue
				}
				i, ok := p.index[alias]
				if !ok {
					i = len(p.hosts)
					p.index[alias] = i
					p.hosts = append(p.hosts, Host{Alias: alias})
				}
				p.block = append(p.block, i)
			}
		case "match":
			p.block = nil
		default:
			if len(args) > 0 {
				p.set(key, args[0])
			}
		}
	}
	return sc.Err()
}

func (p *configParser) include(pattern string) {
	if strings.HasPrefix(pattern, "~/") {
		home, _ := os.UserHomeDir()
		pattern = filepath.Join(home, pattern[2:])
	} else if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(p.dir, pattern)
	}
	matches, _ := filepath.Glob(pattern)
	block := p.block
	for _, m := range matches {
		_ = p.parseFile(m) // a missing or unreadable include is skipped, as ssh does
	}
	p.block = block
}

// set applies a setting to the hosts of the current block; as in ssh, the
// first value obtained wins.
func (p *configParser) set(key, value string) {
	for _, i := range p.block {
		h := &p.hosts[i]
		var field *string
		switch key {
		case "hostname":
			field = &h.HostName
		case "user":
			field = &h.User
		case "port":
			field = &h.Port
		default:
			return
		}
		if *field == "" {
			*field = value
		}
	}
}

// splitDirective splits a config line into its lowercased keyword and its
// arguments, accepting "Key value" and "Key=value" and quoted arguments.
func splitDirective(line string) (string, []string) {
	line = strings.TrimSpace(line)
	if line == "" || line[0] == '#' {
		return "", nil
	}
	end := strings.IndexAny(line, " \t=")
	if end < 0 {
		return strings.ToLower(line), nil
	}
	key := strings.ToLower(line[:end])
	rest := strings.TrimLeft(line[end:], " \t")
	rest = strings.TrimLeft(strings.TrimPrefix(rest, "="), " \t")
	var args []string
	for rest != "" {
		var arg string
		if rest[0] == '"' {
			if j := strings.IndexByte(rest[1:], '"'); j >= 0 {
				arg, rest = rest[1:j+1], rest[j+2:]
			} else {
				arg, rest = rest[1:], ""
			}
		} else if j := strings.IndexAny(rest, " \t"); j >= 0 {
			arg, rest = rest[:j], rest[j:]
		} else {
			arg, rest = rest, ""
		}
		args = append(args, arg)
		rest = strings.TrimLeft(rest, " \t")
	}
	return key, args
}
