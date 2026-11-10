package tui

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// The window paints with exact RGB colours (the desktop app's blue). A terminal
// that has only 8 or 16 colours cannot show them: it uses the nearest palette
// colour, and for these dark blues that is a bright teal, so buttons and fields
// became teal blocks with text that could hardly be read. On such a terminal
// the window uses the plain ANSI colours instead (white on blue, on black),
// which every terminal theme draws legibly.

// basicColors is set while the ANSI palette is in use; tag then names colours
// instead of giving their RGB value.
var basicColors bool

// colWhite is the text on a coloured button or row: white, or the nearest an
// 8 colour terminal has.
var colWhite = tcell.ColorWhite

// colorDepth says how many colours the terminal can show, the way tcell
// decides: 16777216 for true colour, 256, 16, or 8. SALVANZA_TUI_COLORS (8, 16,
// 256 or truecolor) overrides it, for a terminal that does not say what it can
// do.
func colorDepth() int {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SALVANZA_TUI_COLORS"))) {
	case "8", "basic":
		return 8
	case "16":
		return 16
	case "256":
		return 256
	case "true", "truecolor", "24bit", "16777216":
		return 1 << 24
	}
	if runtime.GOOS == "windows" {
		return 1 << 24 // the Windows console takes RGB
	}
	if ct := strings.ToLower(os.Getenv("COLORTERM")); ct == "truecolor" || ct == "24bit" {
		return 1 << 24
	}
	term := strings.ToLower(os.Getenv("TERM"))
	switch {
	case term == "":
		return 256
	case strings.Contains(term, "direct"):
		return 1 << 24
	case strings.Contains(term, "256color"), strings.Contains(term, "kitty"), strings.Contains(term, "alacritty"), strings.Contains(term, "wezterm"):
		return 256
	case strings.Contains(term, "16color"):
		return 16
	}
	return 8 // xterm, screen, vt100 and the like without 256color
}

// usePalette sets the colours for a terminal of that depth: the RGB palette from
// 256 colours up, the ANSI palette below.
func usePalette(colors int) {
	// The style tags in the text say [white]; on an 8 colour terminal that has to
	// be the white it has (index 7, bold for the bright one), not colour 15.
	if colors < 16 {
		tcell.ColorNames["white"] = tcell.ColorSilver
	} else {
		tcell.ColorNames["white"] = tcell.ColorWhite
	}
	if colors >= 256 {
		basicColors = false
		colBg, colPanel = tcell.NewRGBColor(18, 23, 30), tcell.NewRGBColor(24, 31, 40)
		colBorder, colAccent, colSel = tcell.NewRGBColor(58, 74, 92), tcell.NewRGBColor(15, 98, 168), tcell.NewRGBColor(38, 79, 120)
		colText, colDim, colHead = tcell.NewRGBColor(214, 221, 230), tcell.NewRGBColor(111, 130, 150), tcell.NewRGBColor(108, 195, 255)
		colKey, colOK, colWarn, colErr = tcell.NewRGBColor(255, 209, 102), tcell.NewRGBColor(91, 214, 138), tcell.NewRGBColor(255, 180, 84), tcell.NewRGBColor(255, 107, 107)
		colWhite = tcell.ColorWhite
		return
	}
	basicColors = true
	colBg, colPanel = tcell.ColorBlack, tcell.ColorBlack
	colAccent, colSel = tcell.ColorNavy, tcell.ColorNavy // white on blue
	if colors >= 16 {
		colWhite = tcell.ColorWhite
		colBorder, colText, colDim = tcell.ColorGray, tcell.ColorSilver, tcell.ColorGray
		colHead, colKey, colOK, colWarn, colErr = tcell.ColorAqua, tcell.ColorYellow, tcell.ColorLime, tcell.ColorYellow, tcell.ColorRed
		return
	}
	// 8 colours: only the first eight, which every terminal has
	colWhite = tcell.ColorSilver
	colBorder, colText, colDim = tcell.ColorTeal, tcell.ColorSilver, tcell.ColorTeal
	colHead, colKey, colOK, colWarn, colErr = tcell.ColorTeal, tcell.ColorOlive, tcell.ColorGreen, tcell.ColorOlive, tcell.ColorMaroon
}

// basicName is how tag names a colour of the ANSI palette.
var basicName = map[tcell.Color]string{
	tcell.ColorBlack: "black", tcell.ColorMaroon: "maroon", tcell.ColorGreen: "green", tcell.ColorOlive: "olive",
	tcell.ColorNavy: "navy", tcell.ColorPurple: "purple", tcell.ColorTeal: "teal", tcell.ColorSilver: "silver",
	tcell.ColorGray: "gray", tcell.ColorRed: "red", tcell.ColorLime: "lime", tcell.ColorYellow: "yellow",
	tcell.ColorBlue: "blue", tcell.ColorFuchsia: "fuchsia", tcell.ColorAqua: "aqua", tcell.ColorWhite: "white",
}

// onColour is the style of text that is meant to be white on a coloured ground
// (a button, the selected row, a field). In the ANSI palette it is bold: the Linux
// console draws its "white" as a light grey, and bold turns it into bright white.
func onColour(bg tcell.Color) tcell.Style {
	s := tcell.StyleDefault.Background(bg).Foreground(colWhite)
	if basicColors {
		s = s.Bold(true)
	}
	return s
}

// spec is a colour as tview's style tags write it: its name in the ANSI
// palette, else its RGB value.
func spec(c tcell.Color) string {
	if c == tcell.ColorDefault {
		return "-"
	}
	if basicColors {
		if n, ok := basicName[c]; ok {
			return n
		}
	}
	return fmt.Sprintf("#%06x", c.Hex())
}
