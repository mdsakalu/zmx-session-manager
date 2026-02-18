# zsm

TUI session manager for [zmx](https://github.com/nicholasgasior/zmx).

<!-- TODO: screenshot -->

## Install

### Homebrew

```
brew install mdsakalu/tap/zsm
```

### Go

```
go install github.com/mdsakalu/zmx-session-manager@latest
```

## Requirements

[zmx](https://github.com/nicholasgasior/zmx) must be installed and available in your `PATH`.

## Key Bindings

| Key | Action |
|-----|--------|
| `↑` `↓` | Navigate sessions |
| `space` | Toggle selection |
| `ctrl+a` | Select / deselect all |
| `enter` | Attach to session |
| `k` | Kill selected session(s) |
| `c` | Copy attach command |
| `s` | Cycle sort mode (name / clients / newest) |
| `/` | Filter sessions |
| `[` `]` | Scroll activity log |
| `q` | Quit |

## License

[MIT](LICENSE)
