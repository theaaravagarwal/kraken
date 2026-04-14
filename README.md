# kraken

> Smart compilation wrapper — compile and run code with one command.

## Why

Makefiles are great for complex build pipelines, but overkill when you just want to compile a single file with the right flags. **kraken** eliminates boilerplate: it auto-detects the language from the file extension, applies pre-configured compiler flags, and optionally runs the output — all in one command.

## Quick Start

### Install

```bash
# One-line install (downloads latest GitHub Release)
curl -sSL https://raw.githubusercontent.com/theaaravagarwal/kraken/main/install.sh | bash
```

### Use

```bash
# Initialize default config
kraken --init

# Compile and run a file (smart root mode)
kraken main.cpp

# Compile and run immediately
kraken run main.go

# Compile and run without leaving a binary behind
kraken run --temp main.cpp

# Watch/rebuild/restart on save
kraken watch main.cpp

# Parallel judge from tests/*.in + tests/*.out
kraken test solution.cpp tests

# Flag-first run form (also supported)
kraken run --debug main.cpp

# Print exact executed compiler command
kraken --verbose run main.cpp

# Check environment health
kraken --doctor
```

### Build from Source

```bash
git clone https://github.com/theaaravagarwal/kraken.git
cd kraken
./build.sh
```

## Commands

| Command | Description |
|---|---|
| `kraken <file>` | Compile and run a file using the appropriate compiler |
| `kraken <file> --debug` | Compile and pass extra flags to the compiler |
| `kraken run <file>` | Compile and run the output immediately |
| `kraken run --temp <file>` | Compile/run in a temp dir and clean up automatically |
| `kraken watch <file>` | Debounced watch mode with process-group restart |
| `kraken test <file> [tests-dir]` | Parallel testcase runner with normalized diff |
| `kraken run --debug <file>` | Compile and run with flag-first extra args |
| `kraken --verbose run <file>` | Print executed compiler command (`[EXEC]: ...`) |
| `kraken list` | Show available compilers and their status |
| `kraken init` | Generate the default config file |
| `kraken doctor` | Check environment health (compilers, config, permissions) |
| `kraken version` | Show version info |
| `kraken help` | Show help text |

## Configuration

kraken reads from `~/.config/kraken/config.yaml`. Run `kraken --init` to generate a default config with profiles for C, C++, Go, Rust, Java, Zig, D, Nim, V, and Haskell.

### Config Keys

| Key | Type | Description | Default |
|---|---|---|---|
| `languages.<ext>.compiler` | string | The compiler binary to invoke | varies by language |
| `languages.<ext>.flags` | []string | Flags passed to the compiler before the input file | `["-O2", "-Wall"]` (varies) |
| `languages.<ext>.args` | []string | Alternative to `flags`; used for compilers with non-standard CLI (e.g. `go build -o`) | varies |
| `languages.<ext>.output_flag` | string | Flag used to specify the output file (e.g. `-o`) | `-o` |
| `languages.<ext>.output_ext` | string | Extension for the output binary (empty = no extension) | `""` |
| `options.auto_run` | bool | Automatically run the compiled binary after successful compilation | `false` |
| `options.verbose` | bool | Print the compile command and status messages | `true` |

### Example Custom Profile

```yaml
languages:
  py:
    compiler: python3
    args: []
  ts:
    compiler: tsc
    flags: ["--target", "ES2022"]
    output_flag: "--outFile"
```

## kraken --doctor

The `doctor` command diagnoses your setup:

1. **Compilers in PATH** — verifies each configured compiler binary exists and is executable.
2. **Required toolchains** — checks `g++`, `clang`, and `go` are installed.
3. **Config health** — checks the YAML file is valid and readable.

## Versioning

kraken uses [Semantic Versioning](https://semver.org/). Current version: **v1.0.0**.

## License

[MIT](LICENSE)
