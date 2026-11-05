// Starting with the session: a value under the user's Run key, written
// through the registry API rather than reg.exe, which needs no console and
// no PATH to find.

//go:build windows

package main

import (
	"errors"
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	autoStartKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
	autoStartValue = appName
)

// legacyRunNames are the Run values earlier names of the app wrote.
var legacyRunNames = []string{"Pixmint", "zoo_pix"}

// adoptLegacyAutoStart carries the switch over a rename. It drops the stale
// values and reports whether one was there, so the caller can write the
// current executable under the new name instead of keeping a dead path.
func adoptLegacyAutoStart() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, autoStartKey,
		registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	if _, _, err := k.GetStringValue(autoStartValue); err == nil {
		clearLegacyRunValues(k)
		return false
	}
	found := false
	for _, old := range legacyRunNames {
		if _, _, err := k.GetStringValue(old); err == nil {
			found = true
			break
		}
	}
	clearLegacyRunValues(k)
	return found
}

func clearLegacyRunValues(k registry.Key) {
	for _, old := range legacyRunNames {
		_ = k.DeleteValue(old)
	}
}

// setAutoStart adds or removes the Run entry that starts the app at login.
func setAutoStart(on bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, autoStartKey,
		registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !on {
		if err := k.DeleteValue(autoStartValue); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	// Quoted, so that a path with spaces reaches Windows as one argument.
	return k.SetStringValue(autoStartValue, `"`+exe+`"`)
}

// autoStartEnabled reports whether the Run entry is there, whoever wrote it.
func autoStartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, autoStartKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(autoStartValue)
	return err == nil
}
