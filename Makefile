# Releases are published by .github/workflows/release.yml when a v* tag is
# pushed: `make release VERSION=v0.1.0`.

NIX     := nix develop path:. -c
GO      := $(NIX) env GOTOOLCHAIN=local go
VERSION ?=

.PHONY: build run test check snapshot release

build:
	$(GO) build -o gdraw .

run:
	$(GO) run .

test:
	$(GO) vet ./...
	$(GO) test ./...

# check runs the pre-release checks.
check: test
	$(GO) mod tidy
	@test -z "$$(jj diff --name-only go.mod go.sum)" || { echo "go.mod/go.sum not tidy" >&2; exit 1; }
	$(NIX) goreleaser check

# snapshot builds release archives into dist/ without publishing. The Linux
# binary links against the nix store, so it is only for local testing.
snapshot:
	$(NIX) goreleaser release --snapshot --clean

# release tags the last commit (@-) and pushes the tag, which makes CI
# build and publish the GitHub release.
release: check
	@case "$(VERSION)" in v[0-9]*.[0-9]*.[0-9]*) ;; *) echo "usage: make release VERSION=vX.Y.Z" >&2; exit 1;; esac
	@test "$$(jj log -r @ --no-graph -T empty)" = true || { echo "working copy has changes; commit them first" >&2; exit 1; }
	jj tag set $(VERSION) -r @-
	jj git push --tag $(VERSION)
