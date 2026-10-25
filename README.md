# fleet

Fleet clones the primary Git checkouts of a configured repository inventory into `$CODE_DIR/server/org/repo`, keeps them on their intended branches with fast-forward updates, describes the inventory with each repository's GitHub topics, and lists issues and pull requests across the same inventory. `clone` and `sync` are separate commands: sync never clones, and clone never updates an existing checkout. Fleet never stashes, resets, rebases, pushes, or deletes local work, and it never prompts or reads credentials from stdin.


## Install

Fleet supports macOS and Linux. It distributes from HEAD only; there is no release channel or tagged binary.


### Homebrew

```sh
brew tap jmcampanini/fleet https://github.com/jmcampanini/fleet
brew install --HEAD jmcampanini/fleet/fleet
```

Upgrade to the latest commit:

```sh
brew upgrade --fetch-HEAD fleet
```


### From source

```sh
brew install git mise
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

The canonical reference is `fleet --help`, each command's `--help`, and the help topics `fleet exit-codes` and `fleet json-reports`.


## Required external programs

- `git` must be on `PATH` and already authenticated for `clone` and `sync`.
- `gh` must be on `PATH` and already authenticated for `repos`, `issues`, and `prs`.


## Configuration

Fleet reads `$XDG_CONFIG_HOME/fleet/fleet.toml`, or `~/.config/fleet/fleet.toml` when `XDG_CONFIG_HOME` is empty. A minimal file:

```toml
[repos."github.com/jmcampanini/gibson"]

[repos."github.com/jmcampanini/molly"]
branch = "develop"

[groups]
agents = ["gibson", "molly"]
```

`fleet config --help` documents the format and precedence.
