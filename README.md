# dbi

Browse Database Internals book content

`dbi` is a single pure-Go binary. It speaks to databaseinternals over plain
HTTPS, shapes the responses into clean records, and pipes into the rest of your
tools. No API key, nothing to run alongside it.

## Install

```bash
go install github.com/tamnd/databaseinternals-cli/cmd/dbi@latest
```

Or grab a prebuilt binary from the [releases](https://github.com/tamnd/databaseinternals-cli/releases), or run
the container image:

```bash
docker run --rm ghcr.io/tamnd/dbi:latest --help
```

## Usage

```bash
dbi --help
dbi version
```

This is a fresh scaffold. The command tree starts with `version`; build out the
real commands in `cli/` on top of the `databaseinternals` library package.

## Development

```
cmd/dbi/   thin main, wires cli.Root into fang
cli/                 the cobra command tree
databaseinternals/                the library: HTTP client and data models
docs/                tago documentation site
```

```bash
make build      # ./bin/dbi
make test       # go test ./...
make vet        # go vet ./...
```

## Releasing

Push a version tag and GitHub Actions runs GoReleaser, which builds the
archives, Linux packages, the multi-arch GHCR image, checksums, SBOMs, and a
cosign signature:

```bash
git tag v0.1.0
git push --tags
```

The Homebrew and Scoop steps self-disable until their tokens exist, so the first
release works with no extra secrets.

## License

Apache-2.0. See [LICENSE](LICENSE).
