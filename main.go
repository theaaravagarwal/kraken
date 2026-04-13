package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	version    = "1.0.0"
	configDir  = ".config/kraken"
	configFile = "config.yaml"
)

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
	AutoRun  bool `yaml:"auto_run,omitempty"`
	Verbose  bool `yaml:"verbose,omitempty"`
}

// Config represents the full configuration.
type Config struct {
	Languages map[string]*LanguageProfile `yaml:"languages"`
	Options   Options                     `yaml:"options"`
}

// defaultConfig returns a Config with sensible defaults for common languages.
func defaultConfig() *Config {
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
				Compiler: "go",
				Args:     []string{"build", "-o"},
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
				OutputFlag: "",
			},
			"zig": {
				Compiler:   "zig",
				Args:       []string{"build-exe", "-O", "ReleaseFast", "-femit-bin="},
				OutputFlag: "",
				OutputExt:  "",
			},
			"d": {
				Compiler:   "dmd",
				Flags:      []string{"-O", "-release"},
				OutputFlag: "-of",
				OutputExt:  "",
			},
			"nim": {
				Compiler:   "nim",
				Args:       []string{"c", "-d:release", "--outdir:"},
				OutputFlag: "",
				OutputExt:  "",
			},
			"v": {
				Compiler:   "v",
				Flags:      []string{"-prod"},
				OutputFlag: "-o",
				OutputExt:  "",
			},
			"hs": {
				Compiler:   "ghc",
				Flags:      []string{"-O2"},
				OutputFlag: "-o",
				OutputExt:  "",
			},
		},
		Options: Options{
			AutoRun: false,
			Verbose: true,
		},
	}
}

// configPath returns the full path to the config file.
func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get home directory: %w", err)
	}
	return filepath.Join(home, configDir, configFile), nil
}

// ensureConfigDir creates ~/.config/kraken if it doesn't exist.
func ensureConfigDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, configDir)
	return os.MkdirAll(dir, 0755)
}

// loadConfig reads the config from disk, falling back to defaults if missing.
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
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("could not parse config: %w", err)
	}

	// fill in any nil entries
	if cfg.Languages == nil {
		cfg.Languages = make(map[string]*LanguageProfile)
	}

	return &cfg, nil
}

// saveDefaultConfig writes the default config to disk.
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

// findCompiler checks if a compiler binary exists in $PATH.
func findCompiler(name string) (string, error) {
	return exec.LookPath(name)
}

// getCompilerVersion tries to get a version string from the compiler.
func getCompilerVersion(path string) string {
	// Go uses "go version" instead of "go --version"
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
	// Return just the first line
	line := strings.SplitN(string(out), "\n", 2)[0]
	return line
}

// lookupByExt maps a file extension to a language profile key.
func lookupByExt(ext string) string {
	// Strip leading dot
	ext = strings.TrimPrefix(ext, ".")
	return ext
}

// buildCommand constructs the full compile command from a profile.
func buildCommand(profile *LanguageProfile, inputFile, outputFile string, extraArgs []string) []string {
	var args []string

	// If the profile uses Args (like "go build -o"), handle that path
	if len(profile.Args) > 0 {
		args = append(args, profile.Args...)
		// For Args, the output path is typically appended after the last arg
		// e.g., ["build", "-o"] -> append outputFile after "-o"
		lastArg := args[len(args)-1]
		if strings.HasSuffix(lastArg, "=") {
			// Some compilers use --outdir=file syntax
			args[len(args)-1] = lastArg + outputFile
		} else if outputFile != "" {
			args = append(args, outputFile)
		}
	} else {
		// Standard flags + output flag
		if len(profile.Flags) > 0 {
			args = append(args, profile.Flags...)
		}
		if profile.OutputFlag != "" && outputFile != "" {
			args = append(args, profile.OutputFlag, outputFile)
		}
	}

	// Input file
	args = append(args, inputFile)

	// Extra args from command line
	if len(extraArgs) > 0 {
		args = append(args, extraArgs...)
	}

	return args
}

func printVersion() {
	fmt.Printf("kraken v%s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
}

func printUsage() {
	fmt.Println(`kraken — smart compilation wrapper

USAGE:
  kraken <file> [extra flags...]    Compile and optionally run
  kraken --list                     Show available compilers
  kraken --init                     Generate default config
  kraken --run <file>               Compile and run immediately
  kraken --doctor                   Check environment health
  kraken --version                  Show version
  kraken --help                     Show this help

EXAMPLES:
  kraken main.cpp                   Compile with configured flags
  kraken main.cpp --debug           Append --debug to the compile command
  kraken --run main.go              Build and run immediately
  kraken --list                     Check which compilers are available
  kraken --doctor                   Verify your setup is healthy

CONFIG:
  ~/.config/kraken/config.yaml      Language profiles and settings

INSTALL:
  go install github.com/theaaravagarwal/kraken@latest`)
}

func cmdList(cfg *Config) {
	fmt.Println("Compiler availability:")
	fmt.Println(strings.Repeat("─", 60))

	for lang, profile := range cfg.Languages {
		path, err := findCompiler(profile.Compiler)
		status := "✗ Missing"
		version := "— not found"

		if err == nil {
			status = "✓ Available"
			version = getCompilerVersion(path)
		}

		fmt.Printf("  %-10s %-12s %s\n", lang, status, profile.Compiler)
		if cfg.Options.Verbose && err == nil {
			fmt.Printf("             → %s\n", version)
		}
	}
	fmt.Println()

	// Show config path
	cfgPath, _ := configPath()
	fmt.Printf("Config: %s\n", cfgPath)
}

func cmdDoctor(cfg *Config) {
	fmt.Println("kraken doctor — environment health check")
	fmt.Println(strings.Repeat("=", 60))

	allGood := true

	// 1. Check compilers in PATH
	fmt.Println("\n[Compilers in PATH]")
	for lang, profile := range cfg.Languages {
		path, err := findCompiler(profile.Compiler)
		if err != nil {
			fmt.Printf("  ✗ %-10s %s (not in PATH)\n", lang, profile.Compiler)
			allGood = false
		} else {
			ver := getCompilerVersion(path)
			fmt.Printf("  ✓ %-10s %s → %s\n", lang, profile.Compiler, ver)
		}
	}

	// 2. Config health
	fmt.Println("\n[Configuration]")
	cfgPath, err := configPath()
	if err != nil {
		fmt.Printf("  ✗ Could not resolve config path: %v\n", err)
		allGood = false
	} else {
		fmt.Printf("  Config path: %s\n", cfgPath)
		_, err := os.Stat(cfgPath)
		if os.IsNotExist(err) {
			fmt.Printf("  ⚠ Config file does not exist (run `kraken --init` to create)\n")
		} else if err != nil {
			fmt.Printf("  ✗ Could not stat config file: %v\n", err)
			allGood = false
		} else {
			// Validate YAML
			data, readErr := os.ReadFile(cfgPath)
			if readErr != nil {
				fmt.Printf("  ✗ Could not read config file: %v\n", readErr)
				allGood = false
			} else {
				var testCfg Config
				if parseErr := yaml.Unmarshal(data, &testCfg); parseErr != nil {
					fmt.Printf("  ✗ Config YAML is invalid: %v\n", parseErr)
					allGood = false
				} else {
					fmt.Printf("  ✓ Config YAML is valid\n")
				}
			}
		}
	}

	// 3. Permissions
	fmt.Println("\n[Permissions]")
	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		fmt.Printf("  ✗ Could not get home directory: %v\n", homeErr)
		allGood = false
	} else {
		krakenDir := filepath.Join(home, configDir)
		// Check if we can write to the directory (create if missing)
		if err := os.MkdirAll(krakenDir, 0755); err != nil {
			fmt.Printf("  ✗ Cannot create config directory %s: %v\n", krakenDir, err)
			allGood = false
		} else {
			// Try writing a test file
			testFile := filepath.Join(krakenDir, ".write_test")
			if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
				fmt.Printf("  ✗ Cannot write to %s: %v\n", krakenDir, err)
				allGood = false
			} else {
				os.Remove(testFile)
				fmt.Printf("  ✓ Can write to %s\n", krakenDir)
			}
		}
	}

	fmt.Println()
	if allGood {
		fmt.Println("All checks passed ✓")
	} else {
		fmt.Println("Some checks failed — review the output above")
	}
}

func cmdCompile(cfg *Config, file string, extraArgs []string, runAfter bool) error {
	// Resolve the file path
	absFile, err := filepath.Abs(file)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	if _, err := os.Stat(absFile); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", file)
	}

	// Determine language from extension
	ext := filepath.Ext(file)
	if ext == "" {
		return fmt.Errorf("file has no extension, cannot determine language: %s", file)
	}

	lang := lookupByExt(ext)
	profile, ok := cfg.Languages[lang]
	if !ok {
		return fmt.Errorf("no language profile for extension '%s'\nAdd it to ~/.config/kraken/config.yaml", lang)
	}

	// Check compiler exists
	compilerPath, err := findCompiler(profile.Compiler)
	if err != nil {
		return fmt.Errorf("compiler '%s' not found in PATH", profile.Compiler)
	}

	// Determine output file name
	baseName := strings.TrimSuffix(filepath.Base(file), ext)
	outputFile := baseName

	// Handle java specially (javac produces .class files, can't rename easily)
	if lang == "java" {
		outputFile = ""
	}

	// Build the command
	var cmdArgs []string
	if outputFile != "" {
		cmdArgs = buildCommand(profile, absFile, outputFile, extraArgs)
	} else {
		cmdArgs = buildCommand(profile, absFile, "", extraArgs)
	}

	// Print command if verbose
	if cfg.Options.Verbose {
		fmt.Printf("compiling: %s %s\n", profile.Compiler, strings.Join(cmdArgs, " "))
		fmt.Println(strings.Repeat("─", 60))
	}

	// Execute
	cmd := exec.Command(compilerPath, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err = cmd.Run()
	if err != nil {
		// Compiler failed — don't run
		if cfg.Options.Verbose {
			fmt.Printf("\ncompilation failed (exit code: %s)\n", err)
		}
		return err
	}

	if cfg.Options.Verbose {
		fmt.Println("compilation successful ✓")
	}

	// Auto-run if requested
	if runAfter || cfg.Options.AutoRun {
		var runPath string
		if outputFile != "" {
			runPath = filepath.Join(filepath.Dir(absFile), outputFile)
		} else {
			// For java, try running the class file
			if lang == "java" {
				javaPath, _ := findCompiler("java")
				if javaPath == "" {
					return fmt.Errorf("'java' runtime not found in PATH")
				}
				fmt.Printf("running: java %s\n", baseName)
				runCmd := exec.Command(javaPath, "-cp", filepath.Dir(absFile), baseName)
				runCmd.Stdout = os.Stdout
				runCmd.Stderr = os.Stderr
				runCmd.Stdin = os.Stdin
				return runCmd.Run()
			}
			runPath = absFile // fallback
		}

		if lang == "go" {
			// Go outputs a binary; run it
		}

		fmt.Printf("running: %s\n", runPath)
		runCmd := exec.Command(runPath)
		runCmd.Stdout = os.Stdout
		runCmd.Stderr = os.Stderr
		runCmd.Stdin = os.Stdin
		if err := runCmd.Run(); err != nil {
			return fmt.Errorf("run failed: %w", err)
		}
	}

	return nil
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		printUsage()
		os.Exit(0)
	}

	// Handle flags
	switch args[0] {
	case "--help", "-h":
		printUsage()
		os.Exit(0)
	case "--version", "-v":
		printVersion()
		os.Exit(0)
	case "--list", "-l":
		cfg, err := loadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		cmdList(cfg)
		os.Exit(0)
	case "--init":
		if err := saveDefaultConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating config: %v\n", err)
			os.Exit(1)
		}
		cfgPath, _ := configPath()
		fmt.Printf("Default config created at: %s\n", cfgPath)
		os.Exit(0)
	case "--run", "-r":
		// --run <file> [extra...]
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Error: --run requires a file argument\n")
			os.Exit(1)
		}
		cfg, err := loadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := cmdCompile(cfg, args[1], args[2:], true); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	case "--doctor":
		cfg, err := loadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		cmdDoctor(cfg)
		os.Exit(0)
	}

	// Default: treat first arg as file
	file := args[0]
	extraArgs := args[1:]

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := cmdCompile(cfg, file, extraArgs, false); err != nil {
		os.Exit(1)
	}
}
