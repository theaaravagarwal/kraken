# kraken

> Smart compilation wrapper — compile and run code with one command.

## Why

Makefiles are great for complex build pipelines, but overkill when you just want to compile a single file with the right flags. **kraken** eliminates boilerplate: it auto-detects the language from the file extension, applies pre-configured compiler flags, and optionally runs the output — all in one command.

## Quick Start

### Install

```bash
# One-line install (requires Go)
curl -sSL https://raw.githubusercontent.com/theaaravagarwal/kraken/main/install.sh | bash
```

### Use

```bash
# Initialize default config
kraken --init

# Compile a file
kraken main.cpp

# Compile and run immediately
kraken --run main.go

# Check environment health
kraken --doctor
```

### Build from Source

```bash
git clone https://github.com/theaaravagarwal/kraken.git
cd kraken
go install .
```

## Commands

| Command | Description |
|---|---|
| `kraken <file>` | Compile a file using the appropriate compiler |
| `kraken <file> --debug` | Compile and pass extra flags to the compiler |
| `kraken --run <file>` | Compile and run the output immediately |
| `kraken --list` | Show available compilers and their status |
| `kraken --init` | Generate the default config file |
| `kraken --doctor` | Check environment health (compilers, config, permissions) |
| `kraken --version` | Show version info |
| `kraken --help` | Show help text |

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
2. **Config health** — checks the YAML file is valid and readable.
3. **Permissions** — confirms kraken can write to `~/.config/kraken/`.

## Versioning

kraken uses [Semantic Versioning](https://semver.org/). Current version: **v1.0.0**.

## License

[MIT](LICENSE)
