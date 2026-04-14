# kraken

> Smart compilation wrapper — compile and run code with one command.

## Why

Makefiles are great for complex build pipelines, but overkill when you just want to compile a single file with the right flags. **kraken** eliminates boilerplate: it auto-detects the language from the file extension, applies pre-configured compiler flags, and optionally runs the output — all in one command.

## Features

- **One-command compile & run** — `kraken main.cpp` auto-detects language and runs the result
- **Watch mode with hot-reload** — `kraken watch main.go` rebuilds and restarts on file changes
- **Parallel test suite runner** — `kraken test solution.cpp tests/` runs all `*.in` / `*.out` pairs in parallel
- **Configurable language profiles** — Built-in support for C, C++, Go, Rust, Java, Zig, D, Nim, V, Haskell (easy to add more)
- **Smart dependency tracking** — For C/C++, tracks includes and skips unnecessary rebuilds
- **Colorized output with customizable themes** — 8 color palettes (ocean, fire, forest, twilight, sunset, arctic, neon, vintage)
- **Custom banner fonts** — Use figlet fonts (big, small, banner, slant, etc.) for stylized banners
- **Temporary binary mode** — `kraken run --temp main.cpp` compiles in `/tmp` and cleans up automatically
- **Verbose mode** — See exact compiler commands with `kraken --verbose`
- **Environment diagnostics** — `kraken doctor` checks compiler availability and config health

## Install

### Method 1: One-line installer (Recommended)

```bash
curl -sSL https://raw.githubusercontent.com/theaaravagarwal/kraken/main/install.sh | bash
```

This downloads a prebuilt binary if available, or automatically builds from source if you have Go installed. Installs to `~/.local/bin` by default.

Customize the install location:

```bash
export KRAKEN_INSTALL_DIR=/usr/local/bin
curl -sSL https://raw.githubusercontent.com/theaaravagarwal/kraken/main/install.sh | bash
```

### Method 2: Go install (requires Go 1.21+)

```bash
go install github.com/theaaravagarwal/kraken@latest
```

### Method 3: Build from source

```bash
git clone https://github.com/theaaravagarwal/kraken.git
cd kraken
go build -o kraken .
./kraken
```

All methods install the `kraken` binary. Make sure the install directory is on your `$PATH`.

## Publishing a Release

To publish prebuilt binaries for the one-line installer:

```bash
# Tag the release
git tag v1.0.0
git push origin v1.0.0

# Build all platform binaries
./build.sh

# Upload the tar.gz files from dist/ to GitHub Releases:
# https://github.com/theaaravagarwal/kraken/releases/new
```

The naming format must be: `kraken_<os>_<arch>.tar.gz` (e.g. `kraken_darwin_arm64.tar.gz`)

## Quick Start

```bash
# Initialize default config
kraken init

# Compile and run a file (auto-detects language)
kraken main.cpp

# Compile and run immediately
kraken run main.go

# Compile and run without leaving a binary behind
kraken run --temp main.cpp

# Watch/rebuild/restart on file changes
kraken watch main.cpp

# Run parallel test suite (reads tests/*.in + tests/*.out)
kraken test solution.cpp tests

# View available color themes
kraken themes

# Print the exact compiler command being executed
kraken --verbose run main.cpp

# Diagnose your environment
kraken doctor

# Show available compilers and their status
kraken list

# Display version information
kraken version
```

## Commands

| Command | Description |
|---|---|
| `kraken <file>` | Compile and run a file (auto-detects language from extension) |
| `kraken run <file>` | Compile and run the output immediately |
| `kraken run --temp <file>` | Compile and run in a temp directory, then clean up |
| `kraken run --debug <file>` | Compile and run with debug flags |
| `kraken watch <file>` | Watch for file changes and auto-rebuild/restart |
| `kraken test <file> [dir]` | Run parallel test suite against input/output files |
| `kraken list` | Show supported languages and compiler status |
| `kraken themes` | Show available color themes and configuration examples |
| `kraken init` | Generate default configuration file |
| `kraken doctor` | Diagnose environment health |
| `kraken version` | Show version information |
| `kraken help` | Show help text |

### Flags

| Flag | Description |
|---|---|
| `--verbose` | Print executed compiler commands |
| `--debug` | Pass debug flags to the compiler |
| `--temp` | Use a temporary output directory (cleaned up after run) |
| `--init` | Generate default config file |
| `--doctor` | Run environment health check |

## Configuration

kraken reads configuration from `~/.config/kraken/config.yaml`. Run `kraken init` to generate a default config with profiles for C, C++, Go, Rust, Java, Zig, D, Nim, V, and Haskell.

### Language Configuration

| Key | Type | Description | Default |
|---|---|---|---|
| `languages.<ext>.compiler` | `string` | Compiler binary to invoke | varies |
| `languages.<ext>.flags` | `[]string` | Flags passed to the compiler | `["-O2", "-Wall"]` (varies) |
| `languages.<ext>.args` | `[]string` | Alternative to `flags` for non-standard CLIs (e.g. `go build -o`) | varies |
| `languages.<ext>.output_flag` | `string` | Flag to specify output file (e.g. `-o`) | `-o` |
| `languages.<ext>.output_ext` | `string` | Extension for the output binary | `""` |

### Global Options

| Key | Type | Description | Default |
|---|---|---|---|
| `options.auto_run` | `bool` | Auto-run binary after successful compilation | `false` |
| `options.verbose` | `bool` | Print compile command and status messages | `true` |

### UI & Theming

| Key | Type | Description | Default |
|---|---|---|---|
| `ui.colors` | `bool` | Enable/disable colored output | `true` |
| `ui.theme` | `string` | UI theme name | `auto` |
| `ui.banner_font` | `string` | Figlet font for the banner (big, small, banner, slant, etc.) | `""` (uses default ASCII art) |
| `ui.randomize_colors` | `bool` | Randomize color palette on each run | `false` |
| `ui.color_palette` | `string` | Specific color palette to use | `ocean` |

**Available color palettes:** `ocean` (default), `fire`, `forest`, `twilight`, `sunset`, `arctic`, `neon`, `vintage`

### Example Configuration

```yaml
options:
  verbose: true
  auto_run: false

languages:
  py:
    compiler: python3
    args: []
  ts:
    compiler: tsc
    flags: ["--target", "ES2022"]
    output_flag: "--outFile"

ui:
  colors: true
  color_palette: fire
  randomize_colors: false
  banner_font: "big"
```

### Color Theme Examples

View all available themes with their color samples:

```bash
kraken themes           # List all themes
kraken --verbose themes # Show color samples for each theme
```

To use a specific theme, set `ui.color_palette` in `~/.config/kraken/config.yaml`:

```yaml
ui:
  color_palette: neon  # Changes all output colors to the neon theme
```

To randomize colors on each run:

```yaml
ui:
  randomize_colors: true  # Picks a random palette each time
```

To use custom figlet fonts (if figlet is installed):

```yaml
ui:
  banner_font: "slant"  # Try: big, small, banner, slant, standard, etc.
```

## Environment Health

The `kraken --doctor` command diagnoses your setup:

- **Compilers in PATH** — verifies each configured compiler binary exists and is executable
- **Required toolchains** — checks `g++`, `clang`, and `go` are installed
- **Config health** — validates the YAML configuration file

## Versioning

kraken uses [Semantic Versioning](https://semver.org/). Current version: **v1.0.0**.

## License

[MIT](LICENSE)
