package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The bar is a sheet of the appearance, not a black slab laid over a light
// screen: white in the light theme, ink in the dark one, with icons that
// read on whichever it is.
func TestToolbarFollowsTheTheme(t *testing.T) {
	light, dark := &ui.Theme{}, &ui.Theme{Dark: true}

	if c := tbBarColor(light); c.R < 230 || c.G < 230 || c.B < 230 {
		t.Errorf("the bar is not light in the light theme: %v", c)
	}
	if c := tbBarColor(dark); c.R > 60 || c.G > 60 || c.B > 60 {
		t.Errorf("the bar is not ink in the dark theme: %v", c)
	}
	if f, b := tbFore(light), tbBarColor(light); f.R >= b.R {
		t.Errorf("the light bar puts %v icons on its %v sheet", f, b)
	}
	if f, b := tbFore(dark), tbBarColor(dark); f.R <= b.R {
		t.Errorf("the dark bar puts %v icons on its %v sheet", f, b)
	}
	// A divider has to be visible against both sheets.
	for name, c := range map[string]ui.Color{
		"light": tbDivider(light),
		"dark":  tbDivider(dark),
	} {
		if c.A == 0 {
			t.Errorf("the %s divider is invisible", name)
		}
	}
}
