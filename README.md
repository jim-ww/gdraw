# gdraw

A minimalist canvas for quick drawings, built on [Gio](https://gioui.org).

## Features

- Brush, line, arrow, rectangle, ellipse, text, fill, eraser, select, move and color picker
- Undo/redo, copy/cut/paste, snap to grid, ctrl-drag to constrain lines to 45° and shapes to squares/circles
- Infinite canvas with cursor-centered zoom
- Tabs, recent files, crash recovery (autosave + `-restore`)
- Save/load as JSON (same format as [bdraw](https://github.com/jim-ww/bdraw)), export to PNG or SVG
- Configurable palette and keybindings

## Install

Requires Go 1.26+ and Gio's [system dependencies](https://gioui.org/doc/install/linux).

```sh
go install github.com/jim-ww/gdraw@latest
```

Or with Nix:

```sh
nix run github:jim-ww/gdraw
nix develop   # shell with Go and Gio's build dependencies
```

## Usage

```sh
gdraw                  # blank canvas
gdraw file.json        # open (or create) a project
gdraw -restore         # reopen unsaved drawings from a crashed or closed session
gdraw -c config.json   # use a specific config file
gdraw -version         # print the version
```

Press `?` for every keybinding. Mouse: wheel zooms, middle-drag or space+drag pans.

Save to a `.png` or `.svg` path (or use `ctrl+e`) to export.

## Configuration

gdraw reads `config.json` from the user config directory (`~/.config/gdraw` on Linux). All fields are optional:

```json
{
  "palette": ["#ffffff", "#ff0000", "#00ff00", "#0000ff"],
  "default_color": "#ffffff",
  "default_size": 3,
  "hide_grid": false,
  "keys": {
    "tool_brush": ["b"],
    "undo": ["ctrl+z", "u"]
  }
}
```

Action names for `keys` are listed in `keys.go`.

## Support the project

If gdraw is useful to you, consider a small donation.

**Monero (XMR)**

`83YGRqP8uHed6NeegZQeX9ccCxbzoRHHEEi7pTwk4aqdJZEVXXA6NWtetnsEM2v33zFBBt3Rp6DNhU9qhJEGPspU14yN8t7`

## Releasing

```sh
make check                    # vet, test, tidy and goreleaser config check
make snapshot                 # build release archives into dist/ locally
make release                  # push the latest v* tag
make release VERSION=v0.2.0   # tag @- as v0.2.0 if needed, then push it
```

Pushing a `v*` tag runs `.github/workflows/release.yml`, which builds Linux (amd64) and Windows (amd64, arm64) binaries with GoReleaser and publishes the GitHub release. Linux binaries are built on Ubuntu because Gio links the system GUI libraries; macOS isn't built.

## License

GPL-3.0. Free to use, study, share, and modify — provided you keep the same freedoms for others.
