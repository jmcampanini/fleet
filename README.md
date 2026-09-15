# fleet

Fleet clones the primary Git checkouts of a configured repository inventory into `$CODE_DIR/server/org/repo`, keeps them on their intended branches with fast-forward updates, and lists issues and pull requests across the same inventory. `clone` and `sync` are separate commands: sync never clones, and clone never updates an existing checkout. Fleet never stashes, resets, rebases, pushes, or deletes local work.

Command help is the canonical reference: `fleet --help` and each command's `--help` describe every user-facing contract, `fleet config --help` describes the TOML format, discovery, and precedence, `fleet help exit-codes` describes exit statuses, and `fleet help json-reports` describes the fields of every `--json` report.

## Install

Fleet distributes from HEAD only; there is no release channel or tagged binary.

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
make build
# then copy ./build/fleet to a directory on your PATH
```

## Representative commands

| Command | Result |
|---|---|
| `fleet clone --all` | Clone every configured repository that is not checked out yet. |
| `fleet clone --group clis` | Clone the members of one group. |
| `fleet sync` | Fast-forward every existing checkout to its target branch and warn about missing ones. |
| `fleet sync gibson molly --dry-run` | Show what sync would do to two repositories without changing them. |
| `fleet issues --issues-limit 30` | List the 30 newest open issues across the inventory. |
| `fleet prs --state merged --json` | Emit the most recently merged pull requests as one JSON report. |
| `fleet groups --group agents` | List the resolved repository members of a configured group. |
| `fleet config --provenance` | Print the effective configuration with each field's source. |

## Required external programs

Fleet runs `git` for `clone` and `sync`, and `gh` for `issues` and `prs`. Both must be on `PATH`, and `gh` must already be authenticated; Fleet never prompts or reads credentials from stdin. `clone` and `sync` also require `CODE_DIR` to name an absolute, existing directory. Fleet supports macOS and Linux.

## Configuration

Fleet reads `$XDG_CONFIG_HOME/fleet/fleet.toml`, or `~/.config/fleet/fleet.toml` when `XDG_CONFIG_HOME` is empty; `--config PATH` replaces discovery, and the file must exist. The inventory and groups come only from the TOML file. The issue and pull request limits can also come from `FLEET_ISSUES_LIMIT`, `FLEET_PRS_LIMIT`, `--issues-limit`, and `--prs-limit`. A minimal file:

```toml
[repos."github.com/jmcampanini/gibson"]

[repos."github.com/jmcampanini/molly"]
branch = "develop"

[groups]
agents = ["gibson", "molly"]
```

`fleet config --help` documents the format and precedence, and `fleet config` prints the values in effect.
