package discover

import (
	"regexp"
	"sort"
	"strings"
)

// Tool is one registry candidate: a single command, or a family of
// versioned commands such as python, python3 and python3.12.
type Tool struct {
	Key     string  // registry key: family base, or the command name
	Members []Found // sorted by Name
	Tag     Tag
	Noise   bool
}

const configSuffix = "-config"

var (
	dashVersion  = regexp.MustCompile(`^(.+)-(\d+(?:\.\d+)*)$`)
	plainVersion = regexp.MustCompile(`^(.*[A-Za-z])(\d+(?:\.\d+)*)$`)
)

// Group collapses version families among installed commands and flags
// noise. Core commands are never grouped. The result is sorted by Key.
func Group(found []Found) []Tool {
	var tools []Tool
	var plain, config []Found
	taken := map[string]bool{} // names of core tools; no family key may reuse one
	for _, f := range found {
		switch {
		case f.Tag != TagInstalled:
			tools = append(tools, single(f))
			taken[f.Name] = true
		case strings.HasSuffix(f.Name, configSuffix) && f.Name != configSuffix:
			config = append(config, f)
		default:
			plain = append(plain, f)
		}
	}
	tools = append(tools, families(plain, "", taken)...)
	tools = append(tools, families(config, configSuffix, taken)...)
	sort.Slice(tools, func(i, j int) bool { return tools[i].Key < tools[j].Key })
	return tools
}

func single(f Found) Tool {
	return Tool{Key: f.Name, Members: []Found{f}, Tag: f.Tag, Noise: isNoise(f)}
}

type parsed struct {
	base, ver string
	ok        bool
}

func parseVersion(name string) parsed {
	if m := dashVersion.FindStringSubmatch(name); m != nil {
		return parsed{m[1], m[2], true}
	}
	if m := plainVersion.FindStringSubmatch(name); m != nil {
		return parsed{m[1], m[2], true}
	}
	return parsed{}
}

// families groups installed commands whose names (with suffix removed)
// share an anchored base, then re-appends suffix to each key. A base whose
// key is in taken does not form a family.
func families(found []Found, suffix string, taken map[string]bool) []Tool {
	stem := func(f Found) string { return strings.TrimSuffix(f.Name, suffix) }
	exists := map[string]bool{}
	parse := map[string]parsed{}
	for _, f := range found {
		exists[stem(f)] = true
		parse[stem(f)] = parseVersion(stem(f))
	}
	anchored := anchors(exists, parse)
	for base := range anchored {
		if taken[base+suffix] {
			delete(anchored, base)
		}
	}

	byKey := map[string][]Found{}
	for _, f := range found {
		s := stem(f)
		key := s
		if p := parse[s]; !anchored[s] && p.ok && anchored[p.base] {
			key = p.base
		}
		byKey[key] = append(byKey[key], f)
	}

	var tools []Tool
	for key, members := range byKey {
		if len(members) == 1 {
			t := single(members[0])
			t.Key = key + suffix
			tools = append(tools, t)
			continue
		}
		sort.Slice(members, func(i, j int) bool { return members[i].Name < members[j].Name })
		tools = append(tools, Tool{Key: key + suffix, Members: members, Tag: TagInstalled})
	}
	return tools
}

// anchors returns the bases that form a family: base B is anchored when the
// bare name B exists and another name parses to B, or when a name B<major>
// exists and another name parses to B with a version starting "<major>.".
func anchors(exists map[string]bool, parse map[string]parsed) map[string]bool {
	anchored := map[string]bool{}
	for name, p := range parse {
		if !p.ok {
			continue
		}
		if exists[p.base] {
			anchored[p.base] = true
			continue
		}
		major, _, dotted := strings.Cut(p.ver, ".")
		if dotted && exists[p.base+major] && p.base+major != name {
			anchored[p.base] = true
		}
	}
	return anchored
}

// isNoise reports commands hidden from the sync prompt.
func isNoise(f Found) bool {
	n := f.Name
	switch {
	case strings.HasPrefix(n, "_"):
		return true
	case strings.HasSuffix(n, ".sh"), strings.HasSuffix(n, ".py"), strings.HasSuffix(n, "_completer"):
		return true
	case (n == "activate" || n == "deactivate") && f.Shim == "pyenv":
		return true
	}
	return false
}
