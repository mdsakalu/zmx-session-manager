<div align="center">

# zsm

TUI session manager for [zmx](https://github.com/neurosnap/zmx)

[<img src="https://img.shields.io/github/actions/workflow/status/mdsakalu/zmx-session-manager/ci.yaml?label=build&logo=github" />](https://github.com/mdsakalu/zmx-session-manager/actions)
[<img src="https://img.shields.io/github/v/release/mdsakalu/zmx-session-manager?label=release&logo=data%3Aimage%2Fsvg%2Bxml%3Bbase64%2CPHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAxNiAxNiIgZmlsbD0ibm9uZSIgc3Ryb2tlPSJ3aGl0ZSIgc3Ryb2tlLXdpZHRoPSIxLjUiIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIgc3Ryb2tlLWxpbmVqb2luPSJyb3VuZCI%2BCiAgPHBhdGggZD0iTTIgNyBMNyAyIEgxNCBWOSBMOSAxNCBaIi8%2BCiAgPGNpcmNsZSBjeD0iMTEiIGN5PSI1IiByPSIxIi8%2BCjwvc3ZnPg%3D%3D" />](https://github.com/mdsakalu/zmx-session-manager/releases/latest)
[<img src="https://img.shields.io/github/downloads/mdsakalu/zmx-session-manager/total?label=downloads&logo=data%3Aimage%2Fsvg%2Bxml%3Bbase64%2CPHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAxNiAxNiIgZmlsbD0ibm9uZSIgc3Ryb2tlPSJ3aGl0ZSIgc3Ryb2tlLXdpZHRoPSIxLjUiIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIgc3Ryb2tlLWxpbmVqb2luPSJyb3VuZCI%2BCiAgPHBhdGggZD0iTTggMiBWMTAiLz4KICA8cGF0aCBkPSJNNSA3IEw4IDEwIEwxMSA3Ii8%2BCiAgPHBhdGggZD0iTTMgMTMgSDEzIi8%2BCjwvc3ZnPg%3D%3D" />](https://github.com/mdsakalu/zmx-session-manager/releases)
[<img src="https://img.shields.io/badge/Homebrew-mdsakalu/tap/zsm-orange?logo=homebrew" />](https://github.com/mdsakalu/homebrew-tap)
[<img src="https://img.shields.io/github/go-mod/go-version/mdsakalu/zmx-session-manager?logo=go&logoColor=white" />](https://go.dev)
[<img src="https://img.shields.io/badge/platform-macOS-lightgrey?logo=apple" />](https://github.com/mdsakalu/zmx-session-manager)
[<img src="https://img.shields.io/badge/platform-Linux-lightgrey?logo=linux&logoColor=white" />](https://github.com/mdsakalu/zmx-session-manager)
[<img src="https://img.shields.io/github/license/mdsakalu/zmx-session-manager?logo=data%3Aimage%2Fsvg%2Bxml%3Bbase64%2CPHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAxNCAxNiI%2BPHBhdGggZmlsbD0id2hpdGUiIGZpbGwtcnVsZT0iZXZlbm9kZCIgZD0iTTcgNGMtLjgzIDAtMS41LS42Ny0xLjUtMS41UzYuMTcgMSA3IDFzMS41LjY3IDEuNSAxLjVTNy44MyA0IDcgNHptNyA2YzAgMS4xMS0uODkgMi0yIDJoLTFjLTEuMTEgMC0yLS44OS0yLTJsMi00aC0xYy0uNTUgMC0xLS40NS0xLTFIOHY4Yy40MiAwIDEgLjQ1IDEgMWgxYy40MiAwIDEgLjQ1IDEgMUgzYzAtLjU1LjU4LTEgMS0xaDFjMC0uNTUuNTgtMSAxLTFoLjAzTDYgNUg1YzAgLjU1LS40NSAxLTEgMUgzbDIgNGMwIDEuMTEtLjg5IDItMiAySDJjLTEuMTEgMC0yLS44OS0yLTJsMi00SDFWNWgzYzAtLjU1LjQ1LTEgMS0xaDRjLjU1IDAgMSAuNDUgMSAxaDN2MWgtMWwyIDR6TTIuNSA3TDEgMTBoM0wyLjUgN3pNMTMgMTBsLTEuNS0zLTEuNSAzaDN6Ii8%2BPC9zdmc%2B" />](LICENSE)
[<img src="https://img.shields.io/badge/Built_With-Bubble_Tea-blue" />](https://github.com/charmbracelet/bubbletea)

<img src="assets/screenshot.png" alt="zsm screenshot" width="600" />

</div>

## Install

### Homebrew

```
brew install mdsakalu/tap/zsm
```

### Go

```
go install github.com/mdsakalu/zmx-session-manager@latest
```

### Nix

Run directly from the flake:

```
nix run github:mdsakalu/zmx-session-manager
```

Or install it into your profile:

```
nix profile install github:mdsakalu/zmx-session-manager
```

Open a development shell with the Go toolchain and project linters:

```
nix develop github:mdsakalu/zmx-session-manager
```

## Requirements

[zmx](https://github.com/neurosnap/zmx) must be installed and available in your `PATH`. zsm is tested with zmx 0.7.0 and retains compatibility with the legacy `session_name`/`started_in` list format.

## Key Bindings

| Key | Action |
|-----|--------|
| `↑` `↓` | Navigate sessions |
| `space` | Toggle selection |
| `ctrl+a` | Select / deselect all |
| `enter` | Attach to session, then return to zsm after detaching |
| `n` | Create and attach to a new session |
| `e` | Attach by replacing zsm (legacy behavior) |
| `k` | Kill selected session(s) |
| `c` | Copy attach command |
| `s` | Cycle sort mode (name / clients / PID / memory / uptime) |
| `/` | Filter sessions |
| `[` `]` | Scroll activity log |
| `q` | Quit |
| `esc` | Quit, or clear an active filter |

## Contributors

See [CONTRIBUTORS.md](CONTRIBUTORS.md) for the people who have contributed code, reports, testing, and feedback.

## License

[MIT](LICENSE)
