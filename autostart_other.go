// Starting with the session is a Windows feature; elsewhere the switch says
// so when it is turned on.

//go:build !windows

package main

import "errors"

func setAutoStart(on bool) error {
	if !on {
		return nil
	}
	return errors.New("仅 Windows 支持开机自启")
}

func autoStartEnabled() bool { return false }

// adoptLegacyAutoStart has nothing to adopt off Windows.
func adoptLegacyAutoStart() bool { return false }
