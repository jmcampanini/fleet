# Fleet

Fleet is a Go CLI for syncing configured primary Git checkouts and listing issues and pull requests across them.

Build with `make build`, then run `build/fleet --help`. Command help is the usage reference. Start with `fleet config --help` for the TOML format and discovery rules, `fleet sync --help` for checkout safeguards, and `fleet issues --help` or `fleet prs --help` for query behavior.

Run `make check` for formatting, module consistency, static analysis, race tests, a versioned build, and vulnerability checks. Go 1.27.1 and Git are required for development. Runtime sync requires Git; issue and PR queries require authenticated GitHub CLI. Supported operating systems are macOS and Linux.

The [automation contract](docs/automation.md) fixes JSON fields, completeness semantics, and exit codes. The [Homebrew installation guide](docs/install.md) describes the HEAD-only formula. Release automation, configuration seeding, and migration of existing scripts are outside this implementation.
