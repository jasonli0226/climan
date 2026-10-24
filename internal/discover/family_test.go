package discover

import (
	"reflect"
	"testing"
)

func installed(names ...string) []Found {
	var out []Found
	for _, n := range names {
		out = append(out, Found{Name: n, Path: "/h/bin/" + n, Dir: "/h/bin", Tag: TagInstalled})
	}
	return out
}

// shape reduces tools to key → member names for comparison.
func shape(tools []Tool) map[string][]string {
	out := map[string][]string{}
	for _, t := range tools {
		var ms []string
		for _, m := range t.Members {
			ms = append(ms, m.Name)
		}
		out[t.Key] = ms
	}
	return out
}

func TestGroup(t *testing.T) {
	core := func(names ...string) []Found {
		fs := installed(names...)
		for i := range fs {
			fs[i].Tag = TagCore
		}
		return fs
	}
	tests := []struct {
		name  string
		found []Found
		want  map[string][]string
	}{
		{"bare base anchors", installed("python", "python3", "python3.12"),
			map[string][]string{"python": {"python", "python3", "python3.12"}}},
		{"major anchors without bare base", installed("pip3", "pip3.12"),
			map[string][]string{"pip": {"pip3", "pip3.12"}}},
		{"dash version, base ends in digit", installed("2to3", "2to3-3.12"),
			map[string][]string{"2to3": {"2to3", "2to3-3.12"}}},
		{"config variants", installed("python3.12-config", "python3-config", "python-config"),
			map[string][]string{"python-config": {"python-config", "python3-config", "python3.12-config"}}},
		{"no anchor", installed("base32", "base64"),
			map[string][]string{"base32": {"base32"}, "base64": {"base64"}}},
		{"core never collapses", core("diff", "diff3"),
			map[string][]string{"diff": {"diff"}, "diff3": {"diff3"}}},
		{"lone version", installed("python3.14"),
			map[string][]string{"python3.14": {"python3.14"}}},
		{"bare base with numbered siblings", installed("ping", "ping4", "ping6"),
			map[string][]string{"ping": {"ping", "ping4", "ping6"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shape(Group(tt.found)); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Group = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroupSortedByKey(t *testing.T) {
	tools := Group(installed("zoxide", "aws"))
	if len(tools) != 2 || tools[0].Key != "aws" || tools[1].Key != "zoxide" {
		t.Fatalf("keys = %v, want [aws zoxide]", shape(tools))
	}
}

func TestGroupNoise(t *testing.T) {
	found := installed("__pycache__", "x.sh", "vba_extract.py", "aws_completer", "rg")
	pyenv := installed("activate", "deactivate")
	for i := range pyenv {
		pyenv[i].Shim = "pyenv"
	}
	plainActivate := Found{Name: "activate", Path: "/other/activate", Dir: "/other", Tag: TagInstalled}

	noise := map[string]bool{}
	for _, t := range Group(append(found, pyenv...)) {
		noise[t.Key] = t.Noise
	}
	want := map[string]bool{
		"__pycache__": true, "x.sh": true, "vba_extract.py": true, "aws_completer": true,
		"activate": true, "deactivate": true, "rg": false,
	}
	if !reflect.DeepEqual(noise, want) {
		t.Fatalf("noise = %v, want %v", noise, want)
	}

	if tools := Group([]Found{plainActivate}); tools[0].Noise {
		t.Fatal("activate without a pyenv shim is noise, want not noise")
	}
}

func TestGroupFamilyKeyClashesWithCore(t *testing.T) {
	sys := Found{Name: "pip", Path: "/usr/bin/pip", Dir: "/usr/bin", Tag: TagCore}
	tools := Group(append(installed("pip3", "pip3.12"), sys))
	keys := map[string]int{}
	for _, tl := range tools {
		keys[tl.Key]++
	}
	want := map[string]int{"pip": 1, "pip3": 1, "pip3.12": 1}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %v, want %v (no family may reuse a core tool's name)", keys, want)
	}
	for _, tl := range tools {
		if tl.Key == "pip" && tl.Tag != TagCore {
			t.Fatalf("pip = %+v, want the core tool", tl)
		}
	}
}
