# gdviz example — minimal Godot project (shapes + colours)

Run it from this directory (godot on PATH, or `GDVIZ_GODOT=/path/to/godot`):

```bash
gdviz test                       # 1st run: 4 shots, 2 will fail — by design
gdviz review                     # explore the UI: slider, diff heatmap, recording
gdviz approve shapes spinner     # adopt the passing shots
```

Four shots over two scenes of basic primitives:

| shot | what it shows |
|---|---|
| `shapes` | a clean render — **passes** |
| `spinner` | a rotating rig, recorded — **passes**, and the UI gets a ▶ recording scrubber |
| `drift` | the same scene with `--drift=0.18`: every material brightened — **fails** on the changed-area budget (global drift changes most pixels a little) |
| `mismatch` | the same scene with `--broken`: the red box hidden, the ball moved — **fails** the per-pixel threshold (structural change) |

`drift` and `mismatch` ship with the *clean* baselines on purpose, so the
example always has interesting failures to look at. Delete their entries
from `gdviz.toml` — or `gdviz approve drift mismatch` — to go all-green.

Baselines were captured on one machine; other GPUs may differ slightly in
dither/AA. If everything fails on your machine, that's the tool working —
run `gdviz approve --all` once to adopt your machine's render.

Requires a GPU/display (Godot renders for real); godot 4.3+ on PATH.
