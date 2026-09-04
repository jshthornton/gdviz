package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

const usageText = `gdviz %s — visual regression testing for Godot projects

Usage:
  gdviz test     [--project DIR] [filters]   capture every shot, diff vs baselines, report (CI exit code)
  gdviz capture  [--project DIR] [filters]   capture current screenshots only
  gdviz compare  [--project DIR]             diff existing current screenshots vs baselines (no Godot)
  gdviz approve  [--project DIR] [--all|KEY...]  promote current screenshots to baselines
  gdviz review   [--project DIR] [--port N]  open the review UI (diffs, overlays, recordings, approvals)
  gdviz shots    [--project DIR]             list configured shots
  gdviz run      [--project DIR] SCENE [-- USER_ARGS...]
                                             launch one scene adhoc (windowed,
                                             unfocused; no diffing)
  gdviz version

Filters:  --only NAME   run jobs whose name/scene contains NAME (repeatable)
          --skip NAME   skip jobs whose name/scene contains NAME (repeatable)

Common flags:
  --project DIR    Godot project root containing gdviz.toml (default: .)
  --godot PATH     Godot binary (else $GDVIZ_GODOT, config, or PATH)
  --set KEY=VAL    extra project-setting override passed to Godot via override.cfg
  --no-record      disable recordings for this run
  --record         force recordings on for this run

Docs: https://github.com/your-name/gdviz
`

func usage() int {
	fmt.Fprintf(os.Stderr, usageText, version)
	return 2
}

func main() {
	os.Exit(run(os.Args))
}

func run(args []string) int {
	if len(args) < 2 {
		return usage()
	}
	var err error
	switch args[1] {
	case "test":
		err = cmdTest(args[2:])
	case "capture":
		err = cmdCapture(args[2:])
	case "compare":
		err = cmdCompare(args[2:])
	case "approve":
		err = cmdApprove(args[2:])
	case "review":
		err = cmdReview(args[2:])
	case "shots":
		err = cmdShots(args[2:])
	case "run":
		err = cmdRun(args[2:])
	case "version", "--version", "-v":
		fmt.Println("gdviz", version)
	case "help", "--help", "-h":
		fmt.Printf(usageText, version)
	default:
		return usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gdviz:", err)
		return 1
	}
	return 0
}
