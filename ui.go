package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// ANSI color codes
const (
	// Emphasis codes
	bold  = "\033[1m"
	dim   = "\033[2m"
	reset = "\033[0m"
	// Foreground
	black   = "\033[30m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	white   = "\033[37m"
	// Bright foreground
	brightRed     = "\033[91m"
	brightGreen   = "\033[92m"
	brightYellow  = "\033[93m"
	brightBlue    = "\033[94m"
	brightMagenta = "\033[95m"
	brightCyan    = "\033[96m"
	brightWhite   = "\033[97m"
)

// ColorPalette defines a set of colors for different output types
type ColorPalette struct {
	Name    string
	Logo    string // Color for the logo/banner
	Title   string // Color for titles and headers
	Success string // Color for success messages
	Failure string // Color for errors
	Warning string // Color for warnings
	Muted   string // Color for muted/dim text
	Accent1 string // Additional accent color 1
	Accent2 string // Additional accent color 2
}

// ColorPalettes contains all available color schemes
var ColorPalettes = map[string]ColorPalette{
	"ocean": {
		Name:    "ocean",
		Logo:    bold + brightCyan,
		Title:   bold + cyan,
		Success: green,
		Failure: red,
		Warning: yellow,
		Muted:   dim + white,
		Accent1: brightBlue,
		Accent2: blue,
	},
	"fire": {
		Name:    "fire",
		Logo:    bold + brightRed,
		Title:   bold + red,
		Success: brightGreen,
		Failure: brightRed,
		Warning: brightYellow,
		Muted:   dim + white,
		Accent1: yellow,
		Accent2: red,
	},
	"forest": {
		Name:    "forest",
		Logo:    bold + green,
		Title:   bold + brightGreen,
		Success: green,
		Failure: red,
		Warning: yellow,
		Muted:   dim + white,
		Accent1: brightCyan,
		Accent2: cyan,
	},
	"twilight": {
		Name:    "twilight",
		Logo:    bold + brightMagenta,
		Title:   bold + magenta,
		Success: cyan,
		Failure: brightRed,
		Warning: brightYellow,
		Muted:   dim + brightWhite,
		Accent1: brightCyan,
		Accent2: magenta,
	},
	"sunset": {
		Name:    "sunset",
		Logo:    bold + brightYellow,
		Title:   bold + yellow,
		Success: green,
		Failure: red,
		Warning: brightRed,
		Muted:   dim + white,
		Accent1: brightRed,
		Accent2: yellow,
	},
	"arctic": {
		Name:    "arctic",
		Logo:    bold + brightCyan,
		Title:   bold + brightWhite,
		Success: brightGreen,
		Failure: brightRed,
		Warning: yellow,
		Muted:   dim + cyan,
		Accent1: cyan,
		Accent2: brightBlue,
	},
	"neon": {
		Name:    "neon",
		Logo:    bold + brightMagenta,
		Title:   bold + brightCyan,
		Success: brightGreen,
		Failure: brightRed,
		Warning: brightYellow,
		Muted:   dim + white,
		Accent1: brightMagenta,
		Accent2: brightCyan,
	},
	"vintage": {
		Name:    "vintage",
		Logo:    bold + yellow,
		Title:   bold + white,
		Success: green,
		Failure: red,
		Warning: yellow,
		Muted:   dim + white,
		Accent1: yellow,
		Accent2: white,
	},
}

// Default ASCII art logo for Kraken
const defaultKrakenLogo = `
 __                __
|  |--.----.---.-.|  |--.-----.-----.
|    <|   _|  _  ||    <|  -__|     |
|__|__|__| |___._||__|__|_____|__|__|
`

// uiTheme manages all UI output coloring
type uiTheme struct {
	baseEnabled bool
	enabled     bool
	asciiArt    string
	palette     ColorPalette
}

func newUITheme() uiTheme {
	if os.Getenv("NO_COLOR") != "" {
		return uiTheme{
			baseEnabled: false,
			enabled:     false,
			palette:     ColorPalettes["ocean"], // default palette even if disabled
			asciiArt:    defaultKrakenLogo,
		}
	}
	base := isTTY(os.Stdout) && isTTY(os.Stderr)
	return uiTheme{
		baseEnabled: base,
		enabled:     base,
		palette:     ColorPalettes["ocean"], // default palette
		asciiArt:    defaultKrakenLogo,
	}
}

func isTTY(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func (u uiTheme) wrap(code, s string) string {
	if !u.enabled {
		return s
	}
	return code + s + reset
}

// Title displays text with the title color from the current palette
func (u uiTheme) title(s string) string {
	return u.wrap(u.palette.Title, s)
}

// Success displays text with the success color from the current palette
func (u uiTheme) success(s string) string {
	return u.wrap(u.palette.Success, s)
}

// Failure displays text with the failure color from the current palette
func (u uiTheme) failure(s string) string {
	return u.wrap(u.palette.Failure, s)
}

// Warning displays text with the warning color from the current palette
func (u uiTheme) warning(s string) string {
	return u.wrap(u.palette.Warning, s)
}

// Muted displays text with the muted color from the current palette
func (u uiTheme) muted(s string) string {
	return u.wrap(u.palette.Muted, s)
}

// Bold displays text in bold version of the title color
func (u uiTheme) bold(s string) string {
	return u.wrap(u.palette.Title, s)
}

// Info displays text with accent color 1 (for additional info/highlights)
func (u uiTheme) info(s string) string {
	return u.wrap(u.palette.Accent1, s)
}

// Accent displays text with accent color 2 (for decorations/secondary accents)
func (u uiTheme) accent(s string) string {
	return u.wrap(u.palette.Accent2, s)
}

func (u uiTheme) printSessionHeader(section string) {
	if u.asciiArt != "" {
		// Print logo with the palette's logo color
		fmt.Println(u.wrap(u.palette.Logo, u.asciiArt))
		fmt.Println()
	}
	fmt.Println(u.title(section))
}

// getPaletteNames returns slice of all available palette names
func getPaletteNames() []string {
	names := make([]string, 0, len(ColorPalettes))
	for name := range ColorPalettes {
		names = append(names, name)
	}
	// Sort for consistency
	sort.StringSlice(names).Sort()
	return names
}

// selectRandomPalette returns a random palette
func selectRandomPalette() string {
	names := getPaletteNames()
	if len(names) == 0 {
		return "ocean"
	}
	return names[rand.Intn(len(names))]
}

func (u *uiTheme) applyConfig(colors *bool, bannerFont string, randomizeColors *bool, paletteName string) {
	if colors == nil {
		u.enabled = u.baseEnabled
	} else {
		u.enabled = u.baseEnabled && *colors
	}

	if bannerFont != "" {
		generated := generateFigletBanner(bannerFont)
		if generated != "" {
			u.asciiArt = generated
		}
		// If figlet fails, keep the default logo
	}

	// Handle palette selection
	selectedPalette := "ocean" // default

	// If color randomization is enabled, select a random palette
	if randomizeColors != nil && *randomizeColors && u.enabled {
		rand.Seed(time.Now().UnixNano())
		selectedPalette = selectRandomPalette()
	} else if paletteName != "" && paletteName != "random" {
		// If a specific palette is requested, use it
		if p, exists := ColorPalettes[paletteName]; exists {
			selectedPalette = p.Name
		}
	}

	u.palette = ColorPalettes[selectedPalette]
}

func generateFigletBanner(font string) string {
	cmd := exec.Command("figlet", "-f", font, "kraken")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// figlet not installed or font not found — return empty
		return ""
	}

	result := strings.TrimSpace(stdout.String())
	if result == "" {
		return ""
	}
	return result
}
