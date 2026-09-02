# gdviz

**Visual regression testing for Godot — with a review UI, approvals, and recordings.**

gdviz brings the webdev workflow (Chromatic, Percy, Playwright screenshots) to Godot:
render a scene deterministically, screenshot it, pixel-diff it against a committed
baseline, and review the diffs in a local UI where approving a change promotes it
to the new baseline. Scenarios can also record themselves so you can watch what
actually played out — recordings are never diffed and never committed.

One static binary. No pip, no node, no runtime dependencies — the review UI is
embedded (htmx 4).

```
gdviz test     →  capture every shot, diff vs baselines, non-zero exit on regressions
gdviz review   →  http://127.0.0.1:8420  — diffs, overlay slider, recordings, approvals
gdviz approve  →  promote a changed image to the new baseline
```

## Why

Godot has unit test frameworks (GUT, gdUnit4), but games are visual: layout
regressions, material drift, generation changes and authored-scene mistakes are
invisible to `assert_eq`. gdviz makes every screenshot a reviewable, committed
artifact — intended changes get approved and become the new reference,
unintended ones fail the run with a pixel heatmap of exactly what moved.

## Install

```bash
go install github.com/jshthornton/gdviz@latest
```

or build from a clone (mise users: `mise run build`, others: `go build -o bin/gdviz .`).

Requirements: a machine with a GPU/display that can run Godot graphically
(Forward+/Vulkan on Linux; a small X11 window is spawned, unfocused). Headless
servers without a GPU cannot render, so they cannot capture.

## Quickstart

Add a `gdviz.toml` to your Godot project root:

```toml
baseline_dir = "tests/visual/baselines"   # committed to git
output_dir = "tmp/gdviz"                  # gitignored: currents, diffs, recordings, report

[render]
width = 1280
height = 720

[[shot]]
name = "main-menu"
scene = "res://scenes/main_menu.tscn"
quit_after = 120                # generic mode: render 120 frames, last frame is the shot

[[shot]]
name = "gameplay"
scene = "res://tests/visual/gameplay.tscn"   # your scene: it screenshots itself and quits
paths = ["tmp/visual_tests/gameplay/*.png"]  # globs (relative to project) collected as shots
threshold = 0.05
```

```bash
gdviz test               # 1st run: everything is NEW — nothing has a baseline yet
gdviz review             # inspect the shots; click approve to adopt them
gdviz approve --all      # or adopt from the CLI
gdviz test               # now: PASS — and every future run diffs against these
```

Commit `baseline_dir/` (the reference images). Never commit `output_dir/`.

### Two ways a shot is captured

- **Generic mode** (`quit_after`, no `paths`): gdviz runs the scene under Godot's
  movie-writer (deterministic fixed-fps frames) and uses the final recorded frame
  as the screenshot. Zero project code — point it at any scene.
- **Cooperative mode** (`paths`): your capture scene runs its scenario, saves PNGs
  wherever it likes, and quits. gdviz deletes previously matched files, runs the
  scene, then collects every file the globs match as tracked shots. A `paths` glob
  that matches many files produces one shot per file (named by file stem) — leave
  `name` empty for that mode.

Both modes can **record** (see below).

## The review UI

```bash
gdviz review [--port 8420]
```

- Shot list with status chips; failing/new shots first-class.
- **Overlay slider** — drag to wipe baseline vs current; plus side-by-side, and
  single-image baseline/current views.
- **Diff heatmap** — red: baseline was darker, green: current is darker,
  yellow: anti-aliasing (ignored by the threshold).
- **▶ recording** — plays the scenario's recorded frames in a scrubber (play/pause,
  arrow keys, fps selector). Playbook only: recordings are never diffed.
- **approve → baseline** per shot, or approve all pending. Approvals update the
  live page (htmx 4 partial swaps) and rewrite the baseline PNG on disk.

## Commands

| command | what it does |
|---|---|
| `gdviz test` | capture every shot + diff vs baselines + write report; exit 1 on regression |
| `gdviz capture` | capture only (no diffing) |
| `gdviz compare` | re-diff existing currents vs baselines without launching Godot |
| `gdviz approve [--all\|KEY...]` | promote current captures to baselines |
| `gdviz review` | serve the review UI |
| `gdviz shots` | list configured jobs and baseline count |

Useful flags: `--project DIR`, `--godot PATH` (else `$GDVIZ_GODOT`, config, `PATH`),
`--only SUBSTR` / `--skip SUBSTR` (filter jobs), `--no-record` / `--record`,
`--set SECTION/KEY=VALUE` (extra Godot project-setting overrides),
`--fail-on-new` (treat unapproved new shots as failures — for CI),
`--parallel N`.

## Configuration reference

```toml
godot = ""                            # godot binary ($GDVIZ_GODOT / PATH win over config? no: --godot > env > config > PATH)
display_driver = "x11"                # linux only
baseline_dir = "tests/visual/baselines"
output_dir = "tmp/gdviz"

[render]
width = 1280                          # default window/capture size
height = 720
fps = 30                              # recording fps (movie-writer --fixed-fps)
max_frames = 240                      # recordings are decimated to at most this many frames
method = ""                           # optional --rendering-method passthrough

[defaults]
record = true                         # record scenarios by default
threshold = 0.1                       # pixelmatch threshold 0..1 (smaller = more sensitive)
timeout = 600                         # per-job seconds before godot is killed
parallel = 1                          # jobs run concurrently up to this many
args = []                             # extra args appended after "--" for every job
env = []                              # KEY=VALUE env for every job

[[shot]]
name = "my-shot"                      # optional; omit for multi mode (stem-named shots)
scene = "res://scenes/whatever.tscn"  # required
size = "1280x720"                     # or width/height ints; falls back to [render]
paths = ["tmp/shots/*.png"]           # required — globs collected after the run
args = ["--level=2"]                  # extra args for this job only
env = ["SPOOKY_SEED=1234"]
record = true
threshold = 0.05
quit_after = 240                      # generic mode: quit after N frames
serial = true                         # heavy job: run exclusively (nothing else alongside)
timeout = 900
```

## How the diff works

gdviz ports the [pixelmatch](https://github.com/mapbox/pixelmatch) algorithm
(gamma-corrected RGB → YIQ perceptual distance, per-pixel threshold, and
anti-aliased-pixel detection so edge softening does not count as a regression).
`threshold` follows pixelmatch semantics: 0.1 default, smaller = stricter.
A size change (different resolution) is its own `size` failure — you almost
always want to notice that explicitly.

## Recordings

When `record` is on, gdviz launches the scene with `--write-movie` +
`--fixed-fps`, so the whole scenario lands as PNG frames. Frames are decimated
to `max_frames` (evenly spaced, last frame kept) and served by the review UI's
scrubber. They live under `output_dir` (gitignored), never enter the diff, and
cost disk only. In generic mode the recording doubles as the screenshot source;
in cooperative mode your scene still saves its own PNGs exactly as before.

## Parallelism

`[defaults] parallel` (or `--parallel`) runs up to N capture jobs at once — each
job gets its own Godot process, window (cascade-offset so they don't stack
perfectly), overlay project, recording dir and log. A job with `serial = true`
is scheduled **exclusively**: nothing else runs while it does. Use it for heavy
full-game scenarios that want the whole GPU.

## Determinism tips

- Fix your seeds. Randomized scenes will (correctly) fail every run.
- gdviz launches Godot with a small unfocused window (`no_focus`, fixed size,
  dummy audio) via a project-settings overlay, so capture runs don't fight your
  desktop or settings autoloads.
- Movie mode decouples simulation from wall-clock (`--fixed-fps`), which makes
  frame-timed captures stable.
- For UI screenshots beware animated clocks/blinking cursors — mask them or
  raise the threshold for that shot.

## CI

`gdviz test` exits non-zero when any shot is `fail`, `size`, `error` or
`missing`; `--fail-on-new` also fails on unapproved `new` shots. The report
lands in `output_dir/report.json`, and `gdviz review` serves it for triage.
You need a GPU runner (or a self-hosted runner on a dev box) — Godot cannot
render on headless CPU-only machines.

## Status

v0.1 — Linux (X11 + Vulkan/Forward+) is the tested path; Windows/macOS need
their display-driver paths wired (code structure allows it). License: MIT;
the diff algorithm is modelled on pixelmatch (ISC).
