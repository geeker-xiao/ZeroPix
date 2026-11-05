// The font families the machine has, read from the registry Windows keeps
// its font list in, so that the text editor can offer all of them.

//go:build windows

package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// installedFonts lists the families installed for all users and for the
// current one, each with the file that carries it.
func installedFonts() []annotationFont {
	var out []annotationFont
	seen := map[string]bool{}
	keys := []struct {
		root registry.Key
		dir  string
	}{
		// Machine-wide fonts live in the Windows font folder, per-user ones
		// in the user's own font folder since Windows 10.
		{registry.LOCAL_MACHINE, fontPath("")},
		{registry.CURRENT_USER, filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Windows", "Fonts")},
	}
	for _, k := range keys {
		key, err := registry.OpenKey(k.root,
			`SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts`, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		names, err := key.ReadValueNames(-1)
		if err != nil {
			key.Close()
			continue
		}
		for _, n := range names {
			family := familyFromFontEntry(n)
			if family == "" || seen[family] {
				continue
			}
			file := fontEntryFile(key, n, k.dir)
			if file == "" {
				continue
			}
			seen[family] = true
			out = append(out, annotationFont{Name: family, Family: family, File: file})
		}
		key.Close()
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// fontEntryFile is the path of the file a registry entry names, "" for the
// bitmap fonts and for files that are gone.
func fontEntryFile(key registry.Key, name, dir string) string {
	values := []string{}
	if s, _, err := key.GetStringValue(name); err == nil && s != "" {
		values = append(values, s)
	} else if list, _, err := key.GetStringsValue(name); err == nil {
		values = list
	}
	for _, v := range values {
		v = strings.TrimSpace(v)
		switch strings.ToLower(filepath.Ext(v)) {
		case ".ttf", ".ttc", ".otf":
		default:
			continue
		}
		if filepath.IsAbs(v) {
			if _, err := os.Stat(v); err == nil {
				return v
			}
			continue
		}
		path := filepath.Join(dir, v)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// styleWords end the registry entries of a face rather than a family: "Arial
// Bold" is a face of Arial, and the editor only offers families.
var styleWords = []string{
	"bold", "italic", "oblique", "light", "semilight", "semibold",
	"semi bold", "regular", "medium", "black", "thin", "heavy",
	"extrabold", "extra bold", "extralight", "extra light", "ultralight",
}

// familyFromFontEntry turns "Microsoft YaHei & Microsoft YaHei UI
// (TrueType)" into "Microsoft YaHei", and the entry of a single face such as
// "Arial Bold (TrueType)" into nothing, as it would only repeat its family.
func familyFromFontEntry(entry string) string {
	name := entry
	if i := strings.LastIndex(name, " ("); i > 0 && strings.HasSuffix(name, ")") {
		name = name[:i]
	}
	if i := strings.Index(name, " & "); i > 0 {
		name = name[:i]
	}
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, "(") {
		return ""
	}
	lower := strings.ToLower(name)
	for _, w := range styleWords {
		if strings.HasSuffix(lower, " "+w) {
			return ""
		}
	}
	return name
}
