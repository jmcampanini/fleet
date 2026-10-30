# fleet

Fleet clones the primary Git checkouts of a configured repository inventory into `$CODE_DIR/server/org/repo`, keeps them on their intended branches with fast-forward updates, describes the inventory with each repository's GitHub topics, and lists issues and pull requests across the same inventory. `clone` and `sync` are separate commands: sync never clones, and clone never updates an existing checkout. Fleet never stashes, resets, rebases, pushes, or deletes local work, and it never prompts or reads credentials from stdin.


## Install

Fleet publishes versioned releases with prebuilt binaries for Linux and macOS on amd64 and arm64.

Homebrew installs the release cask with Git, gh, and bash, zsh, and fish completions:

```sh
brew install --cask jmcampanini/tap/fleet
brew upgrade --cask jmcampanini/tap/fleet
```

The mise release channel requires Git and gh to be installed separately:

```sh
mise use -g github:jmcampanini/fleet
mise upgrade github:jmcampanini/fleet
```

Release archives require Git and gh; choose a version and platform from the [releases page](https://github.com/jmcampanini/fleet/releases), then repeat these steps with a newer version to upgrade:

```sh
version=0.1.0
platform=darwin_arm64 # Or darwin_amd64, linux_arm64, linux_amd64.
archive="fleet_${version}_${platform}.tar.gz"
gh release download "v${version}" --repo jmcampanini/fleet --pattern "$archive"
tar -xzf "$archive"
# On macOS, remove quarantine if the download was quarantined.
if [ "$(uname -s)" = Darwin ]; then
  xattr -dr com.apple.quarantine fleet
fi
mkdir -p "$HOME/.local/bin"
install -m 755 fleet "$HOME/.local/bin/fleet"
export PATH="$HOME/.local/bin:$PATH"
```

Homebrew's HEAD formula builds the latest source with Go and installs Git and gh; uninstall any existing Fleet Homebrew package before switching between HEAD and releases:

```sh
brew tap jmcampanini/fleet https://github.com/jmcampanini/fleet
brew install --HEAD jmcampanini/fleet/fleet
brew upgrade --fetch-HEAD jmcampanini/fleet/fleet
```

Building from source requires Git, gh, and mise; after updating the source, repeat the build and copy steps to upgrade:

```sh
git clone https://github.com/jmcampanini/fleet
cd fleet
mise trust
mise install
mise run build
mkdir -p "$HOME/.local/bin"
install -m 755 build/fleet "$HOME/.local/bin/fleet"
export PATH="$HOME/.local/bin:$PATH"
```


## Representative commands

| Command | Result |
|---|---|
| `fleet clone --all` | Clone every configured repository that is not checked out yet. |
| `fleet clone --group clis` | Clone the members of one group. |
| `fleet sync` | Fast-forward every existing checkout to its target branch and warn about missing ones. |
| `fleet sync jstack molly --dry-run` | Show what sync would do to two repositories without changing them. |
| `fleet repos --json` | Describe every configured repository with its path, groups, and GitHub topics. |
| `fleet repos --no-topics` | List the inventory from the TOML file without contacting GitHub. |
| `fleet issues --limit 30` | List the 30 newest open issues across the inventory. |
| `fleet prs --state merged --json` | Emit the most recently merged pull requests as one JSON report. |
| `fleet prs --state merged --all --csv --output-file merged.csv` | Save every merged pull request to a new CSV file. |
| `fleet groups --group agents` | List the resolved repository members of a configured group. |

The canonical reference is `fleet --help`, each command's `--help`, and the help topics `fleet help exit-codes` and `fleet help json-reports`.


## Required external programs

- `git` [2.36](https://github.com/git/git/blob/master/Documentation/RelNotes/2.36.0.adoc) or newer must be on `PATH` and already authenticated for `clone` and `sync`.
- `gh` [2.50](https://github.com/cli/cli/releases/tag/v2.50.0) or newer must be on `PATH` and already authenticated for `repos`, `issues`, and `prs`.


## Configuration

Fleet reads `$XDG_CONFIG_HOME/fleet/config.toml`, or `~/.config/fleet/config.toml` when `XDG_CONFIG_HOME` is empty or relative. A minimal file:

```toml
[repos."github.com/jmcampanini/jstack"]

[repos."github.com/jmcampanini/molly"]
branch = "develop"

[groups]
agents = ["jstack", "molly"]
```

`fleet config --help` documents the format and precedence.
