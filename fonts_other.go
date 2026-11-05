// Outside Windows there is no font list to read, so the editor falls back to
// the curated families: a nil list means just that.

//go:build !windows

package main

func installedFonts() []annotationFont { return nil }
