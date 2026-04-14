package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

const (
	version    = "1.0.0"
	configDir  = ".config/kraken"
	configFile = "config.yaml"
	cacheDir   = ".cache/kraken"
	depsFile   = "deps.json"
)

var includeRegex = regexp.MustCompile(`^\s*#include\s+"([^"]+)"`)
var ui = newUITheme()

// LanguageProfile defines how a language is compiled.
type LanguageProfile struct {
	Compiler   string   `yaml:"compiler"`
	Flags      []string `yaml:"flags,omitempty"`
	Args       []string `yaml:"args,omitempty"`
	OutputFlag string   `yaml:"output_flag,omitempty"`
	OutputExt  string   `yaml:"output_ext,omitempty"`
}

// Options holds global settings.
type Options struct {
	AutoRun bool `yaml:"auto_run,omitempty"`
	Verbose bool `yaml:"verbose,omitempty"`
}

type UIConfig struct {
	// Enable/disable colored output (true/false). Auto-detected based on TTY if nil.
	Colors *bool `yaml:"colors,omitempty"`
	// Theme for UI presentation (auto, minimal, etc.)
	Theme string `yaml:"theme,omitempty"`
	// Figlet font for the banner (if figlet is installed). Leave empty to use default ASCII art.
	// Example: "big", "small", "banner"
	BannerFont string `yaml:"banner_font,omitempty"`
	// Randomize the color palette on each run (true/false). Requires colors enabled.
	// Available palettes: ocean, fire, forest, twilight, sunset, arctic, neon, vintage
	RandomizeColors *bool `yaml:"randomize_colors,omitempty"`
	// Specific color palette to use (ocean, fire, forest, twilight, sunset, arctic, neon, vintage)
	// If empty and randomize_colors is true, a random palette is selected each run.
	// If empty and randomize_colors is false, defaults to "ocean".
	ColorPalette string `yaml:"color_palette,omitempty"`
}

// Config represents the full configuration.
type Config struct {
	Languages map[string]*LanguageProfile `yaml:"languages"`
	Options   Options                     `yaml:"options"`
	UI        UIConfig                    `yaml:"ui,omitempty"`
}

type Command interface {
	Execute(args []string) error
}

type commandFunc func(args []string) error

func (f commandFunc) Execute(args []string) error {
	return f(args)
}

type depEntry struct {
	ModUnix  int64    `json:"mod_unix"`
	Includes []string `json:"includes"`
}

type depCache struct {
	Files map[string]depEntry `json:"files"`
}

type testCase struct {
	Name         string
	InputPath    string
	ExpectedPath string
}

type testResult struct {
	Name      string
	Pass      bool
	Duration  time.Duration
	Expected  string
	Actual    string
	Stderr    string
	RunErr    error
	TimedOut  bool
	InputPath string
}

func defaultConfig() *Config {
	defaultColors := true
	defaultRandomize := false
	return &Config{
		Languages: map[string]*LanguageProfile{
			"c": {
				Compiler:   "gcc",
				Flags:      []string{"-O2", "-Wall"},
				OutputFlag: "-o",
				OutputExt:  "",
			},
			"cpp": {
				Compiler:   "g++",
				Flags:      []string{"-O3", "-Wall", "-std=c++20"},
				OutputFlag: "-o",
				OutputExt:  "",
			},
			"go": {
				Compiler:  "go",
				Args:      []string{"build", "-o"},
				OutputExt: "",
			},
			"rs": {
				Compiler:   "rustc",
				Flags:      []string{"-C", "opt-level=3"},
				OutputFlag: "-o",
				OutputExt:  "",
			},
			"java": {
				Compiler: "javac",
				Flags:    []string{},
			},
			"zig": {
				Compiler: "zig",
				Args:     []string{"build-exe", "-O", "ReleaseFast", "-femit-bin="},
			},
			"d": {
				Compiler:   "dmd",
				Flags:      []string{"-O", "-release"},
				OutputFlag: "-of",
			},
			"nim": {
				Compiler: "nim",
				Args:     []string{"c", "-d:release", "--outdir:"},
			},
			"v": {
				Compiler:   "v",
				Flags:      []string{"-prod"},
				OutputFlag: "-o",
			},
			"hs": {
				Compiler:   "ghc",
				Flags:      []string{"-O2"},
				OutputFlag: "-o",
			},
		},
		Options: Options{
			AutoRun: false,
			Verbose: true,
		},
		UI: UIConfig{
			Colors:          &defaultColors,
			Theme:           "auto",
			BannerFont:      "", // Leave empty for default ASCII art, or try "big", "small", "banner" with figlet
			RandomizeColors: &defaultRandomize,
			ColorPalette:    "ocean", // Options: ocean, fire, forest, twilight, sunset, arctic, neon, vintage
		},
	}
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get home directory: %w", err)
	}
	return filepath.Join(home, configDir, configFile), nil
}

func cachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get home directory: %w", err)
	}
	return filepath.Join(home, cacheDir, depsFile), nil
}

func ensureConfigDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, configDir)
	return os.MkdirAll(dir, 0755)
}

func ensureCacheDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, cacheDir)
	return os.MkdirAll(dir, 0755)
}

func loadConfig() (*Config, error) {
	cfgPath, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(), nil
		}
		return nil, fmt.Errorf("could not read config: %w", err)
	}

	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("could not parse config: %w", err)
	}
	if cfg.Languages == nil {
		cfg.Languages = make(map[string]*LanguageProfile)
	}
	return &cfg, nil
}

func applyUIFromConfig(cfg *Config) {
	if cfg == nil {
		return
	}
	ui.applyConfig(cfg.UI.Colors, cfg.UI.BannerFont, cfg.UI.RandomizeColors, cfg.UI.ColorPalette)
}

func saveDefaultConfig() error {
	if err := ensureConfigDir(); err != nil {
		return err
	}
	cfgPath, err := configPath()
	if err != nil {
		return err
	}

	cfg := defaultConfig()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("could not marshal config: %w", err)
	}
	return os.WriteFile(cfgPath, data, 0644)
}

func loadDepCache() (*depCache, error) {
	path, err := cachePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &depCache{Files: map[string]depEntry{}}, nil
		}
		return nil, err
	}
	var c depCache
	if err := json.Unmarshal(data, &c); err != nil {
		return &depCache{Files: map[string]depEntry{}}, nil
	}
	if c.Files == nil {
		c.Files = map[string]depEntry{}
	}
	return &c, nil
}

func saveDepCache(c *depCache) error {
	if err := ensureCacheDir(); err != nil {
		return err
	}
	path, err := cachePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".deps-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func acquireFileLock(path string, timeout time.Duration) (func(), error) {
	deadline := time.Now().Add(timeout)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		if info, statErr := os.Stat(path); statErr == nil {
			if time.Since(info.ModTime()) > 30*time.Minute {
				_ = os.Remove(path)
				continue
			}
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for lock: %s", path)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func findCompiler(name string) (string, error) {
	return exec.LookPath(name)
}

func getCompilerVersion(path string) string {
	if strings.HasSuffix(path, "go") || strings.HasSuffix(path, "go.exe") {
		cmd := exec.Command(path, "version")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "unknown"
		}
		return strings.TrimSpace(string(out))
	}
	cmd := exec.Command(path, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "unknown"
	}
	line := strings.SplitN(string(out), "\n", 2)[0]
	return line
}

func lookupByExt(ext string) string {
	return strings.TrimPrefix(ext, ".")
}

func buildCommand(profile *LanguageProfile, inputFile, outputFile string, extraArgs []string) []string {
	var args []string
	if len(profile.Args) > 0 {
		args = append(args, profile.Args...)
		lastArg := args[len(args)-1]
		if strings.HasSuffix(lastArg, "=") {
			args[len(args)-1] = lastArg + outputFile
		} else if outputFile != "" {
			args = append(args, outputFile)
		}
	} else {
		if len(profile.Flags) > 0 {
			args = append(args, profile.Flags...)
		}
		if profile.OutputFlag != "" && outputFile != "" {
			args = append(args, profile.OutputFlag, outputFile)
		}
	}
	args = append(args, inputFile)
	if len(extraArgs) > 0 {
		args = append(args, extraArgs...)
	}
	return args
}

func printVersion() {
	fmt.Println(ui.title(fmt.Sprintf("kraken v%s (%s/%s)", version, runtime.GOOS, runtime.GOARCH)))
}

func printUsage() {
	ui.printSessionHeader("help")
	fmt.Println()
	fmt.Println(ui.bold("Usage"))
	fmt.Println("  kraken [--verbose] <file>")
	fmt.Println("  kraken run [--temp] <file> [flags...]")
	fmt.Println("  kraken watch <file> [flags...]")
	fmt.Println("  kraken test <file> [tests-dir]")
	fmt.Println("  kraken list | init | doctor | themes | version | help")
	fmt.Println()
	fmt.Println(ui.bold("Commands"))
	fmt.Printf("  %s  %s\n", ui.title("run"), "Compile and run a source file")
	fmt.Printf("  %s %s\n", ui.title("watch"), "Watch and rebuild on changes")
	fmt.Printf("  %s  %s\n", ui.title("test"), "Run parallel judge over *.in/*.out")
	fmt.Printf("  %s  %s\n", ui.title("list"), "Show compiler availability")
	fmt.Printf("  %s  %s\n", ui.title("init"), "Generate default config")
	fmt.Printf("  %s %s\n", ui.title("doctor"), "Check environment health")
	fmt.Printf("  %s %s\n", ui.title("themes"), "Show available color themes")
	fmt.Printf("  %s %s\n", ui.title("version"), "Show version")
	fmt.Printf("  %s  %s\n", ui.title("help"), "Show this help")
	fmt.Println()
	fmt.Println(ui.bold("Examples"))
	fmt.Println("  kraken run --temp main.cpp")
	fmt.Println("  kraken watch main.cpp")
	fmt.Println("  kraken test solution.cpp tests")
	fmt.Println("  kraken themes")
}

func parseCompileInvocation(args []string) (string, []string, error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("missing source file argument")
	}
	file := ""
	extraArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if file == "" && !strings.HasPrefix(arg, "-") {
			file = arg
			continue
		}
		extraArgs = append(extraArgs, arg)
	}
	if file == "" {
		return "", nil, fmt.Errorf("missing source file argument")
	}
	return file, extraArgs, nil
}

func isCCLike(lang string) bool {
	switch lang {
	case "c", "cpp", "cc", "cxx":
		return true
	default:
		return false
	}
}

func scanIncludes(absFile string) ([]string, error) {
	f, err := os.Open(absFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	includes := []string{}
	for s.Scan() {
		m := includeRegex.FindStringSubmatch(s.Text())
		if len(m) == 2 {
			includes = append(includes, m[1])
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return includes, nil
}

func collectDepsBFS(root string, cache *depCache) ([]string, error) {
	queue := []string{root}
	visited := map[string]bool{}
	deps := []string{}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if visited[cur] {
			continue
		}
		visited[cur] = true
		deps = append(deps, cur)

		st, err := os.Stat(cur)
		if err != nil || st.IsDir() {
			continue
		}
		mod := st.ModTime().UnixNano()
		entry, ok := cache.Files[cur]
		includes := entry.Includes
		if !ok || entry.ModUnix != mod {
			includes, err = scanIncludes(cur)
			if err != nil {
				continue
			}
			cache.Files[cur] = depEntry{ModUnix: mod, Includes: includes}
		}
		base := filepath.Dir(cur)
		for _, inc := range includes {
			next := filepath.Clean(filepath.Join(base, inc))
			if _, err := os.Stat(next); err == nil {
				queue = append(queue, next)
			}
		}
	}

	return deps, nil
}

func shouldRebuild(absFile, output string, cache *depCache) (bool, error) {
	outStat, err := os.Stat(output)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return true, err
	}
	outMod := outStat.ModTime()

	deps, err := collectDepsBFS(absFile, cache)
	if err != nil {
		return true, nil
	}
	driftThreshold := time.Now().Add(2 * time.Second)
	for _, dep := range deps {
		st, err := os.Stat(dep)
		if err != nil {
			return true, nil
		}
		if st.ModTime().After(driftThreshold) {
			fmt.Fprintf(os.Stderr, "%s\n", ui.warning(fmt.Sprintf("[WARN]: %s has a future timestamp. Clock drift detected; rebuilds may persist.", filepath.Base(dep))))
			return true, nil
		}
		if st.ModTime().After(outMod) {
			return true, nil
		}
	}
	return false, nil
}

func resolveCompile(cfg *Config, file string, extraArgs []string, outputOverride string, useDirtyCheck bool) (string, string, string, error) {
	absFile, err := filepath.Abs(file)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid file path: %w", err)
	}
	if _, err := os.Stat(absFile); os.IsNotExist(err) {
		return "", "", "", fmt.Errorf("file not found: %s", file)
	}

	ext := filepath.Ext(absFile)
	if ext == "" {
		return "", "", "", fmt.Errorf("file has no extension, cannot determine language: %s", file)
	}
	lang := lookupByExt(ext)
	profile, ok := cfg.Languages[lang]
	if !ok {
		return "", "", "", fmt.Errorf("no language profile for extension '%s'", lang)
	}
	compilerPath, err := findCompiler(profile.Compiler)
	if err != nil {
		return "", "", "", fmt.Errorf("compiler '%s' not found in PATH", profile.Compiler)
	}

	baseName := strings.TrimSuffix(filepath.Base(absFile), ext)
	outputPath := outputOverride
	if outputPath == "" && lang != "java" {
		outputPath = filepath.Join(filepath.Dir(absFile), baseName+profile.OutputExt)
	}

	if outputPath != "" {
		unlock, err := acquireFileLock(outputPath+".kraken.lock", 45*time.Second)
		if err != nil {
			return "", "", "", err
		}
		defer unlock()
	}

	if useDirtyCheck && isCCLike(lang) && outputPath != "" {
		c, _ := loadDepCache()
		if c != nil {
			dirty, _ := shouldRebuild(absFile, outputPath, c)
			_ = saveDepCache(c)
			if !dirty {
				if cfg.Options.Verbose {
					fmt.Println(ui.muted("[EXEC]: skipped compile (dependency graph unchanged)"))
				}
				return outputPath, absFile, lang, nil
			}
		}
	}

	cmdArgs := buildCommand(profile, absFile, outputPath, extraArgs)
	if cfg.Options.Verbose {
		fmt.Println(ui.muted(fmt.Sprintf("[EXEC]: %s %s", profile.Compiler, strings.Join(cmdArgs, " "))))
		fmt.Println(ui.muted(strings.Repeat("─", 60)))
	}

	cmd := exec.Command(compilerPath, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return "", "", "", err
	}
	if cfg.Options.Verbose {
		fmt.Println(ui.success("compilation successful"))
	}
	return outputPath, absFile, lang, nil
}

func setPGID(cmd *exec.Cmd) {
	if !isWindows() {
		setPGIDUnix(cmd)
	}
}

func killProcessGroup(pid int) error {
	if pid <= 0 {
		return nil
	}
	if isWindows() {
		p, err := os.FindProcess(pid)
		if err != nil {
			return err
		}
		return p.Kill()
	}
	return killUnix(-pid)
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}

func runBinary(lang, absFile, runPath string, workDir string) error {
	if lang == "java" {
		javaPath, err := findCompiler("java")
		if err != nil {
			return fmt.Errorf("java runtime not found")
		}
		base := strings.TrimSuffix(filepath.Base(absFile), filepath.Ext(absFile))
		fmt.Println(ui.title(fmt.Sprintf("running: java %s", base)))
		cmd := exec.Command(javaPath, "-cp", filepath.Dir(absFile), base)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if workDir != "" {
			cmd.Dir = workDir
		}
		return runWithSignalCleanup(cmd, false)
	}

	fmt.Println(ui.title(fmt.Sprintf("running: %s", runPath)))
	cmd := exec.Command(runPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if workDir != "" {
		cmd.Dir = workDir
	}
	return runWithSignalCleanup(cmd, false)
}

func runWithSignalCleanup(cmd *exec.Cmd, useProcessGroup bool) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-sigCh:
		if useProcessGroup {
			_ = killProcessGroup(cmd.Process.Pid)
		} else if cmd.Process != nil {
			_ = cmd.Process.Signal(os.Interrupt)
		}
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			if useProcessGroup {
				_ = killProcessGroup(cmd.Process.Pid)
			} else if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			<-done
		}
		return nil
	}
}

func cmdCompile(cfg *Config, file string, extraArgs []string, runAfter bool) error {
	runPath, absFile, lang, err := resolveCompile(cfg, file, extraArgs, "", true)
	if err != nil {
		if cfg.Options.Verbose {
			fmt.Printf("\n%s\n", ui.failure(fmt.Sprintf("compilation failed (exit code: %v)", err)))
		}
		return err
	}
	if runAfter || cfg.Options.AutoRun {
		return runBinary(lang, absFile, runPath, filepath.Dir(absFile))
	}
	return nil
}

func parseLeadingGlobalFlags(args []string) ([]string, *bool) {
	var verboseOverride *bool
	i := 0
	for i < len(args) {
		switch args[i] {
		case "--verbose":
			v := true
			verboseOverride = &v
			i++
		default:
			return args[i:], verboseOverride
		}
	}
	return []string{}, verboseOverride
}

func isLikelySourceFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func handleCompileCommand(args []string, runAfter bool, verboseOverride *bool) error {
	file, extraArgs, err := parseCompileInvocation(args)
	if err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}
	applyUIFromConfig(cfg)
	if verboseOverride != nil {
		cfg.Options.Verbose = *verboseOverride
	}
	return cmdCompile(cfg, file, extraArgs, runAfter)
}

func handleRun(args []string, verboseOverride *bool) error {
	temp := false
	filtered := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--temp" {
			temp = true
			continue
		}
		filtered = append(filtered, a)
	}
	if !temp {
		ui.printSessionHeader("run")
		return handleCompileCommand(filtered, true, verboseOverride)
	}

	file, extraArgs, err := parseCompileInvocation(filtered)
	if err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	applyUIFromConfig(cfg)
	if verboseOverride != nil {
		cfg.Options.Verbose = *verboseOverride
	}
	ui.printSessionHeader("run --temp")
	tempDir, err := os.MkdirTemp("", "kraken-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	tempBin := filepath.Join(tempDir, base)
	runPath, absFile, lang, err := resolveCompile(cfg, file, extraArgs, tempBin, false)
	if err != nil {
		return err
	}
	if cfg.Options.Verbose {
		fmt.Println(ui.muted(fmt.Sprintf("[temp] %s", tempDir)))
	}
	return runBinary(lang, absFile, runPath, filepath.Dir(absFile))
}

func clearScreen() {
	if !ui.enabled {
		return
	}
	fmt.Print("\033[2J\033[H")
}

func handleWatch(args []string, verboseOverride *bool) error {
	file, extraArgs, err := parseCompileInvocation(args)
	if err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	applyUIFromConfig(cfg)
	if verboseOverride != nil {
		cfg.Options.Verbose = *verboseOverride
	}
	absFile, err := filepath.Abs(file)
	if err != nil {
		return err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()
	if err := watcher.Add(filepath.Dir(absFile)); err != nil {
		return err
	}

	var running *exec.Cmd
	killRunning := func() {
		if running != nil && running.Process != nil {
			_ = killProcessGroup(running.Process.Pid)
			_, _ = running.Process.Wait()
			running = nil
		}
	}

	rebuild := func(reason string) {
		clearScreen()
		ui.printSessionHeader(fmt.Sprintf("watch | %s (%s)", file, reason))
		fmt.Println(ui.muted(strings.Repeat("=", 60)))
		killRunning()

		runPath, abs, lang, buildErr := resolveCompile(cfg, file, extraArgs, "", true)
		if buildErr != nil {
			fmt.Println(ui.failure(fmt.Sprintf("build failed: %v", buildErr)))
			return
		}
		if lang == "java" {
			javaPath, err := findCompiler("java")
			if err != nil {
				fmt.Println(ui.failure(fmt.Sprintf("run failed: %v", err)))
				return
			}
			base := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
			running = exec.Command(javaPath, "-cp", filepath.Dir(abs), base)
		} else {
			running = exec.Command(runPath)
		}
		running.Dir = filepath.Dir(abs)
		running.Stdout = os.Stdout
		running.Stderr = os.Stderr
		running.Stdin = os.Stdin
		// Watch mode acts as a process supervisor; keep child in its own group so
		// file-change restarts can terminate the full process tree.
		setPGID(running)
		if err := running.Start(); err != nil {
			fmt.Println(ui.failure(fmt.Sprintf("run failed: %v", err)))
			running = nil
			return
		}
		fmt.Println(ui.muted(fmt.Sprintf("running pid=%d", running.Process.Pid)))
	}

	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	pending := false
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	rebuild("initial build")

	for {
		select {
		case ev := <-watcher.Events:
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
				continue
			}
			pending = true
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(100 * time.Millisecond)
		case err := <-watcher.Errors:
			fmt.Println(ui.failure(fmt.Sprintf("watch error: %v", err)))
		case <-timer.C:
			if pending {
				pending = false
				rebuild("file change")
			}
		case <-sigCh:
			killRunning()
			return nil
		}
	}
}

func normalizeOutput(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

func sideBySideDiff(expected, actual string) string {
	e := strings.Split(expected, "\n")
	a := strings.Split(actual, "\n")
	max := len(e)
	if len(a) > max {
		max = len(a)
	}
	var b strings.Builder
	b.WriteString("line | expected                              | actual\n")
	b.WriteString("-----+---------------------------------------+---------------------------------------\n")
	for i := 0; i < max; i++ {
		ev := ""
		av := ""
		if i < len(e) {
			ev = e[i]
		}
		if i < len(a) {
			av = a[i]
		}
		if ev == av {
			continue
		}
		b.WriteString(fmt.Sprintf("%4d | %-37s | %-37s\n", i+1, ev, av))
	}
	return b.String()
}

func discoverTestCases(dir string) ([]testCase, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.in"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	cases := make([]testCase, 0, len(matches))
	for _, in := range matches {
		name := strings.TrimSuffix(filepath.Base(in), ".in")
		out := filepath.Join(dir, name+".out")
		if _, err := os.Stat(out); err != nil {
			continue
		}
		cases = append(cases, testCase{Name: name, InputPath: in, ExpectedPath: out})
	}
	return cases, nil
}

func runTestCase(binaryPath, workDir string, tc testCase, timeout time.Duration) testResult {
	res := testResult{Name: tc.Name, InputPath: tc.InputPath}
	input, err := os.ReadFile(tc.InputPath)
	if err != nil {
		res.RunErr = err
		return res
	}
	expectedRaw, err := os.ReadFile(tc.ExpectedPath)
	if err != nil {
		res.RunErr = err
		return res
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath)
	cmd.Dir = workDir
	cmd.Stdin = bytes.NewReader(input)
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	setPGID(cmd)

	start := time.Now()
	err = cmd.Run()
	res.Duration = time.Since(start)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		res.TimedOut = true
	}

	actualNorm := normalizeOutput(stdoutBuf.String())
	expectedNorm := normalizeOutput(string(expectedRaw))
	res.Expected = expectedNorm
	res.Actual = actualNorm
	res.Stderr = stderrBuf.String()
	res.RunErr = err
	res.Pass = !res.TimedOut && err == nil && actualNorm == expectedNorm
	return res
}

func parseTestInvocation(args []string) (string, string, []string, error) {
	file, rest, err := parseCompileInvocation(args)
	if err != nil {
		return "", "", nil, err
	}
	testsDir := "tests"
	compileFlags := rest
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		if st, err := os.Stat(rest[0]); err == nil && st.IsDir() {
			testsDir = rest[0]
			compileFlags = rest[1:]
		}
	}
	return file, testsDir, compileFlags, nil
}

func handleTest(args []string, verboseOverride *bool) error {
	file, testsDir, compileFlags, err := parseTestInvocation(args)
	if err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	applyUIFromConfig(cfg)
	if verboseOverride != nil {
		cfg.Options.Verbose = *verboseOverride
	}
	ui.printSessionHeader("test")
	if cfg.Options.Verbose {
		fmt.Println(ui.muted("[normalize]: CRLF->LF, trim trailing whitespace, trim trailing empty lines"))
	}

	tempDir, err := os.MkdirTemp("", "kraken-test-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	binaryPath := filepath.Join(tempDir, base)
	_, absFile, lang, err := resolveCompile(cfg, file, compileFlags, binaryPath, false)
	if err != nil {
		return err
	}
	if lang == "java" {
		return fmt.Errorf("kraken test currently supports native binaries only")
	}

	cases, err := discoverTestCases(testsDir)
	if err != nil {
		return err
	}
	if len(cases) == 0 {
		return fmt.Errorf("no testcases found in %s (*.in/*.out)", testsDir)
	}

	jobs := make(chan testCase, len(cases))
	results := make(chan testResult, len(cases))
	workerCount := runtime.NumCPU()
	if workerCount < 1 {
		workerCount = 1
	}

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for tc := range jobs {
				results <- runTestCase(binaryPath, filepath.Dir(absFile), tc, 2*time.Second)
			}
		}()
	}

	for _, tc := range cases {
		jobs <- tc
	}
	close(jobs)
	wg.Wait()
	close(results)

	all := make([]testResult, 0, len(cases))
	for r := range results {
		all = append(all, r)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })

	failed := 0
	totalDuration := time.Duration(0)
	for _, r := range all {
		totalDuration += r.Duration
		testLabel := fmt.Sprintf("[%s]", r.Name)
		pathLabel := fmt.Sprintf("(%s)", r.InputPath)
		if r.Pass {
			fmt.Printf("%s  %-6s %6s  %s\n", ui.success("PASS"), testLabel, r.Duration.Round(time.Millisecond), pathLabel)
			continue
		}
		failed++
		fmt.Printf("%s  %-6s %6s  %s -> %s\n", ui.failure("FAIL"), testLabel, r.Duration.Round(time.Millisecond), pathLabel, summarizeFailure(r))
	}
	fmt.Println(ui.muted(strings.Repeat("-", 31)))
	summary := fmt.Sprintf("RESULTS: %d/%d Passed (Total: %s)", len(all)-failed, len(all), totalDuration.Round(time.Millisecond))
	if failed == 0 {
		fmt.Println(ui.success(summary))
	} else {
		fmt.Println(ui.failure(summary))
	}
	if failed > 0 {
		return fmt.Errorf("%d test(s) failed", failed)
	}
	return nil
}

func summarizeFailure(r testResult) string {
	if r.TimedOut {
		return "timeout"
	}
	if r.RunErr != nil {
		return r.RunErr.Error()
	}
	exp := firstLine(r.Expected)
	act := firstLine(r.Actual)
	return fmt.Sprintf("Expected %q, got %q", exp, act)
}

func firstLine(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.SplitN(s, "\n", 2)
	return parts[0]
}

func indent(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

func handleList(verboseOverride *bool) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	applyUIFromConfig(cfg)
	if verboseOverride != nil {
		cfg.Options.Verbose = *verboseOverride
	}
	ui.printSessionHeader("list")
	fmt.Println(ui.muted(strings.Repeat("─", 60)))
	for lang, profile := range cfg.Languages {
		path, err := findCompiler(profile.Compiler)
		status := ui.failure("missing")
		version := "— not found"
		if err == nil {
			status = ui.success("available")
			version = getCompilerVersion(path)
		}
		fmt.Printf("  %-10s %-12s %s\n", lang, status, ui.title(profile.Compiler))
		if cfg.Options.Verbose && err == nil {
			fmt.Printf("             %s %s\n", ui.muted("->"), ui.muted(version))
		}
	}
	cfgPath, _ := configPath()
	fmt.Printf("\n%s %s\n", ui.bold("Config:"), cfgPath)
	return nil
}

func handleDoctor(verboseOverride *bool) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	applyUIFromConfig(cfg)
	if verboseOverride != nil {
		cfg.Options.Verbose = *verboseOverride
	}

	ui.printSessionHeader("doctor")
	fmt.Println(ui.muted(strings.Repeat("=", 60)))
	allGood := true

	fmt.Println("\n" + ui.bold("[Compilers in PATH]"))
	for lang, profile := range cfg.Languages {
		path, err := findCompiler(profile.Compiler)
		if err != nil {
			fmt.Printf("  %s %-10s %s (not in PATH)\n", ui.failure("x"), lang, profile.Compiler)
			allGood = false
		} else {
			fmt.Printf("  %s %-10s %s -> %s\n", ui.success("ok"), lang, profile.Compiler, ui.muted(getCompilerVersion(path)))
		}
	}

	fmt.Println("\n" + ui.bold("[Required Toolchains]"))
	for _, tool := range []string{"g++", "clang", "go"} {
		if _, err := exec.LookPath(tool); err != nil {
			fmt.Printf("  %s %s not found\n", ui.failure("x"), tool)
			allGood = false
		} else {
			fmt.Printf("  %s %s found\n", ui.success("ok"), tool)
		}
	}

	fmt.Println("\n" + ui.bold("[Configuration]"))
	cfgPath, err := configPath()
	if err != nil {
		fmt.Printf("  %s Could not resolve config path: %v\n", ui.failure("x"), err)
		allGood = false
	} else {
		fmt.Printf("  Config path: %s\n", cfgPath)
		data, readErr := os.ReadFile(cfgPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				fmt.Println("  " + ui.warning("! Config file does not exist (run `kraken init`)"))
			} else {
				fmt.Printf("  %s Could not read config file: %v\n", ui.failure("x"), readErr)
				allGood = false
			}
		} else {
			var testCfg Config
			dec := yaml.NewDecoder(bytes.NewReader(data))
			dec.KnownFields(true)
			if parseErr := dec.Decode(&testCfg); parseErr != nil {
				fmt.Printf("  %s Config YAML is invalid: %v\n", ui.failure("x"), parseErr)
				allGood = false
			} else {
				fmt.Printf("  %s Config YAML is valid\n", ui.success("ok"))
			}
		}
	}

	fmt.Println()
	if allGood {
		fmt.Println(ui.success("All checks passed"))
	} else {
		fmt.Println(ui.failure("Some checks failed; review the output above"))
	}
	return nil
}

func handleInit() error {
	if err := saveDefaultConfig(); err != nil {
		return err
	}
	cfgPath, _ := configPath()
	ui.printSessionHeader("init")
	fmt.Printf("%s %s\n", ui.success("Default config created at:"), cfgPath)
	return nil
}

func handleThemes(verboseOverride *bool) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	applyUIFromConfig(cfg)
	if verboseOverride != nil {
		cfg.Options.Verbose = *verboseOverride
	}

	ui.printSessionHeader("themes")
	fmt.Println(ui.muted("Available color palettes (set in config.yaml ui.color_palette):"))
	fmt.Println()

	paletteNames := getPaletteNames()
	for _, name := range paletteNames {
		palette := ColorPalettes[name]
		// Display the palette name in its own color
		header := fmt.Sprintf("  %s", name)
		fmt.Println(ui.wrap(palette.Logo, header))

		if cfg.Options.Verbose {
			// Show sample colors from this palette
			fmt.Printf("    %s", ui.wrap(palette.Title, "title"))
			fmt.Printf(" | %s", ui.wrap(palette.Success, "success"))
			fmt.Printf(" | %s", ui.wrap(palette.Failure, "failure"))
			fmt.Printf(" | %s\n", ui.wrap(palette.Warning, "warning"))
		}
	}

	fmt.Println()
	fmt.Println(ui.muted("Configuration example (in ~/.config/kraken/config.yaml under ui section):"))
	fmt.Println()
	fmt.Println(ui.muted("  color_palette: ocean          # Use specific palette"))
	fmt.Println(ui.muted("  randomize_colors: true        # Randomize on each run"))
	fmt.Println(ui.muted("  colors: true                  # Enable/disable colors"))
	fmt.Println(ui.muted("  banner_font: big              # Use figlet font (if installed)"))
	fmt.Println()
	return nil
}

func commandRegistry(verboseOverride *bool) map[string]Command {
	return map[string]Command{
		"help": commandFunc(func(args []string) error {
			cfg, _ := loadConfig()
			applyUIFromConfig(cfg)
			printUsage()
			return nil
		}),
		"--help": commandFunc(func(args []string) error {
			cfg, _ := loadConfig()
			applyUIFromConfig(cfg)
			printUsage()
			return nil
		}),
		"-h": commandFunc(func(args []string) error {
			cfg, _ := loadConfig()
			applyUIFromConfig(cfg)
			printUsage()
			return nil
		}),
		"version":   commandFunc(func(args []string) error { printVersion(); return nil }),
		"--version": commandFunc(func(args []string) error { printVersion(); return nil }),
		"-v":        commandFunc(func(args []string) error { printVersion(); return nil }),
		"run":       commandFunc(func(args []string) error { return handleRun(args, verboseOverride) }),
		"--run":     commandFunc(func(args []string) error { return handleRun(args, verboseOverride) }),
		"-r":        commandFunc(func(args []string) error { return handleRun(args, verboseOverride) }),
		"watch":     commandFunc(func(args []string) error { return handleWatch(args, verboseOverride) }),
		"test":      commandFunc(func(args []string) error { return handleTest(args, verboseOverride) }),
		"init":      commandFunc(func(args []string) error { return handleInit() }),
		"--init":    commandFunc(func(args []string) error { return handleInit() }),
		"doctor":    commandFunc(func(args []string) error { return handleDoctor(verboseOverride) }),
		"--doctor":  commandFunc(func(args []string) error { return handleDoctor(verboseOverride) }),
		"list":      commandFunc(func(args []string) error { return handleList(verboseOverride) }),
		"--list":    commandFunc(func(args []string) error { return handleList(verboseOverride) }),
		"-l":        commandFunc(func(args []string) error { return handleList(verboseOverride) }),
		"themes":    commandFunc(func(args []string) error { return handleThemes(verboseOverride) }),
		"--themes":  commandFunc(func(args []string) error { return handleThemes(verboseOverride) }),
	}
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}
	args, verboseOverride := parseLeadingGlobalFlags(os.Args[1:])
	if len(args) == 0 {
		printUsage()
		os.Exit(0)
	}

	subcommand := args[0]
	rest := args[1:]
	registry := commandRegistry(verboseOverride)

	var err error
	if cmd, ok := registry[subcommand]; ok {
		err = cmd.Execute(rest)
	} else if isLikelySourceFile(subcommand) {
		err = handleRun(args, verboseOverride)
	} else {
		err = fmt.Errorf("unknown command or file: %s", subcommand)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", ui.failure(fmt.Sprintf("Error: %v", err)))
		os.Exit(1)
	}
}
