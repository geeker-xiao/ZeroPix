// Vector icons for the snip toolbar. Each one is a 24x24 stroke drawing in
// Lucide's style, painted with the current color by ui.Painter.Icon.

package main

import "github.com/egoist/mygo/ui"

const iconAttrs = `fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"`

func iconSVG(body string) []byte {
	return []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" ` +
		iconAttrs + `>` + body + `</svg>`)
}

// iconSet is built once; a nil entry means the SVG failed to parse and the
// button draws nothing rather than crashing.
var iconSet = buildIcons()

func buildIcons() map[string]*ui.SVG {
	src := map[string]string{
		// A rounded outline, the "描边" tool of Snipaste.
		"rect": `<rect x="3" y="3" width="18" height="18" rx="2"/>`,

		"ellipse": `<ellipse cx="12" cy="12" rx="9" ry="7"/>`,

		"circle": `<circle cx="12" cy="12" r="9"/>`,

		"line": `<path d="M5 19 19 5"/>`,

		"brush": `<path d="m9.06 11.9 8.07-8.06a2.85 2.85 0 1 1 4.03 4.03l-8.06 8.08"/>` +
			`<path d="M7.07 14.94c-1.66 0-3 1.35-3 3.02 0 1.33-2.5 1.52-2 2.02 1.08 1.1 2.49 2.02 4 2.02 2.2 0 4-1.8 4-4.04a3.01 3.01 0 0 0-3-3.02z"/>`,

		"highlighter": `<path d="m9 11-6 6v3h9l3-3"/><path d="m22 12-4.6 4.6a2 2 0 0 1-2.8 0l-5.2-5.2a2 2 0 0 1 0-2.8L14 4"/>`,

		// A droplet: the region is softened, as water would.
		"blur": `<path d="M7 16.3c2.2 0 4-1.83 4-4.05 0-1.16-.57-2.26-1.71-3.19S7.29 6.75 7 5.3c-.29 1.45-1.14 2.84-2.29 3.76S3 11.1 3 12.25c0 2.22 1.8 4.05 4 4.05z"/>` +
			`<path d="M17 18.3c1.7 0 3-1.4 3-3.05 0-.87-.43-1.7-1.28-2.39S17.2 11 17 9.9c-.2 1.1-.85 2.13-1.72 2.82S14 13.4 14 14.25c0 1.65 1.35 3.05 3 3.05z"/>`,

		"spotlight": `<circle cx="12" cy="12" r="4"/><path d="M12 2v2"/>` +
			`<path d="M12 20v2"/><path d="m4.9 4.9 1.4 1.4"/><path d="m17.7 17.7 1.4 1.4"/>` +
			`<path d="M2 12h2"/><path d="M20 12h2"/><path d="m6.3 17.7-1.4 1.4"/><path d="m19.1 4.9-1.4 1.4"/>`,

		"dimension": `<path d="M3 12h18"/><path d="M3 7v10"/><path d="M21 7v10"/>` +
			`<path d="m8 9-2 3 2 3"/><path d="m16 9 2 3-2 3"/>`,

		"ruler": `<path d="M21.3 15.3a2.4 2.4 0 0 1 0 3.4l-2.6 2.6a2.4 2.4 0 0 1-3.4 0L2.7 8.7a2.4 2.4 0 0 1 0-3.4l2.6-2.6a2.4 2.4 0 0 1 3.4 0Z"/>` +
			`<path d="m14.5 12.5 2-2"/><path d="m11.5 9.5 2-2"/><path d="m8.5 6.5 2-2"/><path d="m17.5 15.5 2-2"/>`,

		"eraser": `<path d="m7 21-4.3-4.3c-1-1-1-2.5 0-3.4l9.6-9.6c1-1 2.5-1 3.4 0l5.6 5.6c1 1 1 2.5 0 3.4L13 21"/>` +
			`<path d="M22 21H7"/><path d="m5 11 9 9"/>`,

		// An eye struck through: the text under it is hidden.
		"mask": `<path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z"/>` +
			`<path d="M4 4l16 16"/>`,

		"settings": `<path d="M4 21v-7"/><path d="M4 10V3"/><path d="M12 21v-9"/>` +
			`<path d="M12 8V3"/><path d="M20 21v-5"/><path d="M20 12V3"/>` +
			`<path d="M1 14h6"/><path d="M9 8h6"/><path d="M17 16h6"/>`,

		"arrow": `<path d="M7 17 17 7"/><path d="M7 7h10v10"/>`,

		// A 3x3 grid of blocks: pixelation.
		"mosaic": `<g fill="currentColor" stroke="none">` +
			`<rect x="3" y="3" width="5" height="5" rx="1"/>` +
			`<rect x="9.5" y="3" width="5" height="5" rx="1"/>` +
			`<rect x="16" y="3" width="5" height="5" rx="1"/>` +
			`<rect x="3" y="9.5" width="5" height="5" rx="1"/>` +
			`<rect x="9.5" y="9.5" width="5" height="5" rx="1"/>` +
			`<rect x="16" y="9.5" width="5" height="5" rx="1"/>` +
			`<rect x="3" y="16" width="5" height="5" rx="1"/>` +
			`<rect x="9.5" y="16" width="5" height="5" rx="1"/>` +
			`<rect x="16" y="16" width="5" height="5" rx="1"/></g>`,

		"text": `<path d="M4 7V5a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v2"/>` +
			`<path d="M12 3v18"/><path d="M8 21h8"/>`,

		"number": `<circle cx="12" cy="12" r="9"/><path d="M12 8v8"/><path d="M9.6 10.4 12 8"/>`,

		"undo": `<path d="M9 14 4 9l5-5"/><path d="M4 9h10.5a5.5 5.5 0 0 1 0 11H11"/>`,
		"redo": `<path d="m15 14 5-5-5-5"/><path d="M20 9H9.5a5.5 5.5 0 0 0 0 11H13"/>`,

		"scan": `<path d="M3 7V5a2 2 0 0 1 2-2h2"/><path d="M17 3h2a2 2 0 0 1 2 2v2"/>` +
			`<path d="M21 17v2a2 2 0 0 1-2 2h-2"/><path d="M7 21H5a2 2 0 0 1-2-2v-2"/>` +
			`<path d="M7 8h8"/><path d="M7 12h10"/><path d="M7 16h6"/>`,

		"translate": `<path d="m5 8 6 6"/><path d="m4 14 6-6 2-3"/><path d="M2 5h12"/>` +
			`<path d="M7 2h1"/><path d="m22 22-5-10-5 10"/><path d="M14 18h6"/>`,

		"video": `<path d="m16 10.5 5-3.5v10l-5-3.5"/><rect x="2" y="6" width="14" height="12" rx="2"/>`,

		"close": `<path d="M18 6 6 18"/><path d="m6 6 12 12"/>`,

		// The tick of a checked box in the settings dialog.
		"check": `<path d="m5 12.5 4.5 4.5L19 7"/>`,

		"pin": `<path d="M12 17v5"/>` +
			`<path d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V7a1 1 0 0 1 1-1 2 2 0 0 0 0-4H8a2 2 0 0 0 0 4 1 1 0 0 1 1 1z"/>`,

		"save": `<path d="M15.2 3a2 2 0 0 1 1.4.6l3.8 3.8a2 2 0 0 1 .6 1.4V19a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z"/>` +
			`<path d="M17 21v-7a1 1 0 0 0-1-1H8a1 1 0 0 0-1 1v7"/><path d="M7 3v4a1 1 0 0 0 1 1h7"/>`,

		"copy": `<rect width="14" height="14" x="8" y="8" rx="2"/>` +
			`<path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/>`,

		"stop": `<rect x="6" y="6" width="12" height="12" rx="2" fill="currentColor" stroke="none"/>`,

		"fullscreen": `<path d="M8 3H5a2 2 0 0 0-2 2v3"/><path d="M21 8V5a2 2 0 0 0-2-2h-3"/>` +
			`<path d="M3 16v3a2 2 0 0 0 2 2h3"/><path d="M16 21h3a2 2 0 0 0 2-2v-3"/>`,

		// Group titles of the settings dialog.
		"keyboard": `<rect x="2" y="4" width="20" height="16" rx="2"/>` +
			`<path d="M6 8h.01"/><path d="M10 8h.01"/><path d="M14 8h.01"/><path d="M18 8h.01"/>` +
			`<path d="M6 12h.01"/><path d="M10 12h.01"/><path d="M14 12h.01"/><path d="M18 12h.01"/>` +
			`<path d="M8 16h8"/>`,

		// The arrows of the Red, Green and Blue spinners.
		"chevron-up":   `<path d="m18 15-6-6-6 6"/>`,
		"chevron-down": `<path d="m6 9 6 6 6-6"/>`,

		// An i in a circle: the about page.
		"info": `<circle cx="12" cy="12" r="9"/><path d="M12 16v-4"/>` +
			`<path d="M12 8h.01"/>`,

		// The eyedropper of a colour picker: pick another colour off the screen.
		"pipette": `<path d="m2 22 1-1h3l9-9"/><path d="M3 21v-3l9-9"/>` +
			`<path d="m15 6 3.4-3.4a2.1 2.1 0 1 1 3 3L18 9l.4.4a2.1 2.1 0 1 1-3 3l-3.8-3.8a2.1 2.1 0 1 1 3-3l.4.4Z"/>`,

		"palette": `<circle cx="13.5" cy="6.5" r=".6" fill="currentColor" stroke="none"/>` +
			`<circle cx="17.5" cy="10.5" r=".6" fill="currentColor" stroke="none"/>` +
			`<circle cx="8.5" cy="7.5" r=".6" fill="currentColor" stroke="none"/>` +
			`<circle cx="6.5" cy="12.5" r=".6" fill="currentColor" stroke="none"/>` +
			`<path d="M12 2C6.5 2 2 6.5 2 12s4.5 10 10 10c.9 0 1.65-.75 1.65-1.69 0-.44-.18-.83-.44-1.12` +
			`-.29-.29-.44-.65-.44-1.13a1.64 1.64 0 0 1 1.67-1.66h2c3.05 0 5.55-2.5 5.55-5.56C21.97 6.01 17.46 2 12 2z"/>`,
	}
	out := make(map[string]*ui.SVG, len(src))
	for name, body := range src {
		if s, err := ui.ParseSVG(iconSVG(body)); err == nil {
			out[name] = s
		}
	}
	return out
}

// drawIcon paints a named icon fitted into r, in color c. The "gif" badge is
// drawn as a rounded box with its letters, as Snipaste's is.
func drawIcon(p *ui.Painter, name string, r ui.Rect, c ui.Color) {
	if name == "gif" {
		p.Stroke(r, c, 3, 1.6)
		p.Text(r.X+3.5, r.Y+r.H/2-5.5, "GIF", 9, c)
		return
	}
	if s := iconSet[name]; s != nil {
		p.Icon(s, r, c)
	}
}
