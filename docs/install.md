# Installation

Fleet's Homebrew formula follows Grove and Namo's HEAD-only source build. It builds the `main` branch, injects Homebrew's commit-derived `HEAD-...` version into `cmd.Version`, and generates shell completions. It installs Git and GitHub CLI as runtime dependencies.

After the formula is on `main`, install it with an explicit tap URL:

```sh
brew tap jmcampanini/fleet https://github.com/jmcampanini/fleet
brew install --HEAD jmcampanini/fleet/fleet
```

Upgrade the same HEAD installation with:

```sh
brew update
brew upgrade --fetch-HEAD jmcampanini/fleet/fleet
```

The formula matches the existing CLI build and completion pattern. It has not been installed, audited, or tested, as requested. Its `test` block is supplied for consistency with the other CLIs and has not been executed. Fleet has no declared repository license, so the formula omits a license declaration.

For a source build, use `make build` and run `build/fleet --help`. Runtime configuration and authentication are separate from installation. See `fleet config --help` to create the required TOML file and `fleet sync --help` for `CODE_DIR` and checkout safeguards.
