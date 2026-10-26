# fleet

Fleet clones the primary Git checkouts of a configured repository inventory into `$CODE_DIR/server/org/repo`, keeps them on their intended branches with fast-forward updates, describes the inventory with each repository's GitHub topics, and lists issues and pull requests across the same inventory. `clone` and `sync` are separate commands: sync never clones, and clone never updates an existing checkout. Fleet never stashes, resets, rebases, pushes, or deletes local work, and it never prompts or reads credentials from stdin.


## Install

Fleet's release distribution provides prebuilt binaries for Linux and macOS on amd64 and arm64. The first release has not been published yet; until then, use the HEAD formula or build from source below.


```sh
brew install --cask jmcampanini/tap/fleet # Install with Homebrew
brew upgrade --cask jmcampanini/tap/fleet # Upgrade with Homebrew
mise use -g github:jmcampanini/fleet     # Install with mise
mise upgrade github:jmcampanini/fleet    # Upgrade with mise
```

Homebrew installs Git, gh, and bash, zsh, and fish completions. For mise or archive installations, install Git and gh separately. Authentication prerequisites are listed under required external programs.

Or download an archive for your OS and architecture from the [releases page](https://github.com/jmcampanini/fleet/releases), extract it, and copy `fleet` onto your `PATH`. To upgrade, replace the binary with the new release. Releases include SHA-256 checksums and GitHub build provenance attestations. On macOS, if a browser download is quarantined, run `xattr -d com.apple.quarantine fleet` on the extracted binary before running it.


### Homebrew from HEAD

The existing HEAD formula remains an alternative that builds the latest source:

```sh
brew tap jmcampanini/fleet https://github.com/jmcampanini/fleet
brew install --HEAD jmcampanini/fleet/fleet
```

Upgrade to the latest commit:

```sh
brew upgrade --fetch-HEAD jmcampanini/fleet/fleet
```

The HEAD formula and release cask both install `fleet`; choose one. To switch to the cask, first run `brew uninstall --formula jmcampanini/fleet/fleet`, then install the cask above.


### From source

```sh
brew install git gh mise
git clone https://github.com/jmcampanini/fleet
cd fleet
mise trust
mise install
mise run build
# then copy ./build/fleet to a directory on your PATH
```


## Representative commands

| Command | Result |
|---|---|
| `fleet clone --all` | Clone every configured repository that is not checked out yet. |
| `fleet clone --group clis` | Clone the members of one group. |
| `fleet sync` | Fast-forward every existing checkout to its target branch and warn about missing ones. |
| `fleet sync gibson molly --dry-run` | Show what sync would do to two repositories without changing them. |
| `fleet repos --json` | Describe every configured repository with its path, groups, and GitHub topics. |
| `fleet repos --no-topics` | List the inventory from the TOML file without contacting GitHub. |
| `fleet issues --issues-limit 30` | List the 30 newest open issues across the inventory. |
| `fleet prs --state merged --json` | Emit the most recently merged pull requests as one JSON report. |
| `fleet groups --group agents` | List the resolved repository members of a configured group. |
| `fleet config --provenance` | Print the effective configuration with each field's source. |

The canonical reference is `fleet --help`, each command's `--help`, and the help topics `fleet help exit-codes` and `fleet help json-reports`.


## Required external programs

- `git` must be on `PATH` and already authenticated for `clone` and `sync`.
- `gh` must be on `PATH` and already authenticated for `repos`, `issues`, and `prs`.


## Configuration

Fleet reads `$XDG_CONFIG_HOME/fleet/config.toml`, or `~/.config/fleet/config.toml` when `XDG_CONFIG_HOME` is empty. A minimal file:

```toml
[repos."github.com/jmcampanini/gibson"]

[repos."github.com/jmcampanini/molly"]
branch = "develop"

[groups]
agents = ["gibson", "molly"]
```

`fleet config --help` documents the format and precedence.
