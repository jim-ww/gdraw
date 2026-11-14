# Releases are published by .github/workflows/release.yml when a v* tag is
# pushed: `make release` pushes the latest v* tag; `make release
# VERSION=v0.2.0` creates that tag on @- first if it doesn't exist.

NIX     := nix develop path:. -c
GO      := $(NIX) env GOTOOLCHAIN=local go
VERSION ?= $(shell jj tag list 'v*' -T 'name ++ "\n"' 2>/dev/null | sort -V | tail -n1)

.PHONY: build run test check snapshot release version

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

# version validates VERSION.
version:
	@test -n "$(VERSION)" || { echo "no v* tag found; create one or set VERSION=vX.Y.Z" >&2; exit 1; }
	@case "$(VERSION)" in v[0-9]*.[0-9]*.[0-9]*) ;; *) echo "invalid version $(VERSION), want vX.Y.Z" >&2; exit 1;; esac
	@echo "releasing $(VERSION)"

# release pushes the VERSION tag, creating it on the last commit (@-) if
# needed, which makes CI build and publish the GitHub release.
release: version check
	@if [ -z "$$(jj tag list '$(VERSION)' -T name 2>/dev/null)" ]; then \
		test "$$(jj log -r @ --no-graph -T empty)" = true || { echo "working copy has changes; commit them first" >&2; exit 1; }; \
		echo "jj tag set $(VERSION) -r @-"; jj tag set $(VERSION) -r @-; \
	fi
	jj git push --tag $(VERSION)
