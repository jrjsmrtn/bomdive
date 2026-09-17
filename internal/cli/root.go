// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
//
// SPDX-License-Identifier: Apache-2.0

// Package cli wires the commands.
package cli

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/jrjsmrtn/bomdive/internal/bom"
	"github.com/jrjsmrtn/bomdive/internal/render"
	"github.com/spf13/cobra"
)

// app holds per-invocation state.
//
// Flags are not package-level globals, for two reasons that are worth separating.
// The real one: Run takes its streams, so a test can drive the CLI in-process
// without capturing os.Stdout, and nothing mutable is shared between callers.
//
// The reason that does NOT apply, recorded so nobody re-derives it: cobra examples
// use package globals, and it is tempting to say a second Run would inherit the
// first invocation's flags. Mutation testing showed it would not — cobra
// re-registers every flag with its default on each Run, which resets them. Avoid
// the globals on design grounds, not on a bug that does not exist here.
type app struct {
	out  io.Writer
	err  io.Writer
	json bool
	long bool

	cpuProfile string
	memProfile string
	stopCPU    func()
}

// Run executes the CLI with explicit args and streams, and returns the exit code.
//
// Everything is a parameter so a caller can drive it without a subprocess and
// without capturing os.Stdout.
func Run(version string, args []string, stdout, stderr io.Writer) int {
	a := &app{out: stdout, err: stderr}

	root := &cobra.Command{
		Use:   "bomdive",
		Short: "Read a CycloneDX xBOM at the terminal",
		Long: "bomdive reads a CycloneDX xBOM — SBOM, OBOM, HBOM — and navigates it with the\n" +
			"muscle memory of ls(1) and tree(1).\n\n" +
			"It is not a BOM generator, query tool, scanner, signer or converter.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().BoolVar(&a.json, "json", false, "emit JSON instead of text")
	root.PersistentFlags().BoolVarP(&a.long, "long", "l", false, "long format: type, purl and category")
	root.PersistentFlags().StringVar(&a.cpuProfile, "cpuprofile", "", "write a CPU profile to this file")
	root.PersistentFlags().StringVar(&a.memProfile, "memprofile", "", "write a heap profile to this file")

	// Profiling wraps the command rather than living inside it, so every subcommand
	// gets it and none has to remember to. It exists because a benchmark told us the
	// walk was slow and a GUESS about why was wrong — see quality-configuration.md.
	root.PersistentPreRunE = func(*cobra.Command, []string) error { return a.startProfiling() }
	root.PersistentPostRunE = func(*cobra.Command, []string) error { return a.stopProfiling() }
	root.AddCommand(a.lsCmd(), a.treeCmd(), a.browseCmd())

	if err := root.Execute(); err != nil {
		// PostRun does not run when the command fails, so profiles would be lost on
		// exactly the runs worth profiling.
		_ = a.stopProfiling()
		fmt.Fprintln(stderr, "bomdive:", err)
		return 1
	}
	return 0
}

func (a *app) startProfiling() error {
	if a.cpuProfile == "" {
		return nil
	}
	f, err := os.Create(a.cpuProfile)
	if err != nil {
		return fmt.Errorf("cpuprofile: %w", err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		return fmt.Errorf("cpuprofile: %w", err)
	}
	a.stopCPU = func() {
		pprof.StopCPUProfile()
		f.Close()
	}
	return nil
}

func (a *app) stopProfiling() error {
	if a.stopCPU != nil {
		a.stopCPU()
		a.stopCPU = nil
	}
	if a.memProfile == "" {
		return nil
	}
	f, err := os.Create(a.memProfile)
	if err != nil {
		return fmt.Errorf("memprofile: %w", err)
	}
	defer f.Close()
	// GC first, so the heap profile describes what is RETAINED rather than what
	// happens to be uncollected. Without it the numbers flatter the program.
	//
	// ⚠ DELIBERATELY UNTESTED. Removing this line is not caught by any test, and
	// mutation testing confirmed that. Asserting it would mean parsing the profile
	// and comparing allocation totals, which is flaky and would be the first gate
	// disabled. It changes the QUALITY of the report, not whether one is produced.
	runtime.GC()
	if err := pprof.WriteHeapProfile(f); err != nil {
		return fmt.Errorf("memprofile: %w", err)
	}
	return nil
}

func (a *app) emit(r render.Result) error {
	if a.json {
		return render.JSON(a.out, r)
	}
	return render.Text(a.out, r, a.long)
}

func load(path string) (*bom.Graph, error) { return bom.Load(path) }
