package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type uiTheme struct {
	baseEnabled bool
	enabled     bool
	asciiArt    string
}

func newUITheme() uiTheme {
	if os.Getenv("NO_COLOR") != "" {
		return uiTheme{baseEnabled: false, enabled: false}
	}
	base := isTTY(os.Stdout) && isTTY(os.Stderr)
	return uiTheme{baseEnabled: base, enabled: base}
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
	return code + s + "\033[0m"
}

func (u uiTheme) title(s string) string {
	return u.wrap("\033[1;96m", s)
}

func (u uiTheme) success(s string) string {
	return u.wrap("\033[32m", s)
}

func (u uiTheme) failure(s string) string {
	return u.wrap("\033[31m", s)
}

func (u uiTheme) warning(s string) string {
	return u.wrap("\033[33m", s)
}

func (u uiTheme) muted(s string) string {
	return u.wrap("\033[2;37m", s)
}

func (u uiTheme) bold(s string) string {
	return u.wrap("\033[1m", s)
}

func (u uiTheme) printSessionHeader(section string) {
	if u.asciiArt != "" {
		fmt.Println(u.title(u.asciiArt))
		fmt.Println()
	}
	fmt.Println(u.title(section))
}

func (u *uiTheme) applyConfig(colors *bool, bannerFont string) {
	if colors == nil {
		u.enabled = u.baseEnabled
	} else {
		u.enabled = u.baseEnabled && *colors
	}

	if bannerFont != "" {
		u.asciiArt = generateFigletBanner(bannerFont)
	}
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
