//go:build windows

package main

import "testing"

// Windows names a family and its UI variant in one registry entry, and each
// face in an entry of its own; the editor wants the family alone.
func TestFamilyFromFontEntry(t *testing.T) {
	cases := map[string]string{
		"Microsoft YaHei & Microsoft YaHei UI (TrueType)": "Microsoft YaHei",
		"SimSun & NSimSun (TrueType)":                     "SimSun",
		"Arial (TrueType)":                                "Arial",
		"Arial Bold (TrueType)":                           "",
		"Segoe UI Semibold (TrueType)":                    "",
		"Noto Sans CJK SC (OpenType)":                     "Noto Sans CJK SC",
	}
	for entry, want := range cases {
		if got := familyFromFontEntry(entry); got != want {
			t.Errorf("%q -> %q, want %q", entry, got, want)
		}
	}
}
