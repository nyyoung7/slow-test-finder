// slowtest reads the JSON event stream produced by `go test -json` and
// prints the slowest tests, sorted by elapsed time.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
)

// testEvent mirrors the subset of the go test -json schema we care about.
// The full schema also has Output and FailedBuild fields; we ignore them.
type testEvent struct {
	Action  string
	Package string
	Test    string
	Elapsed float64
}

type testResult struct {
	Package string
	Test    string
	Action  string
	Elapsed float64
}

// packageResult is the package-level pass/fail/skip event go test -json
// emits once per package. Its Elapsed is the total wall-clock time for that
// package's test run, which is what "slowest package" actually means -
// summing individual test elapsed times would overcount time spent in
// parallel subtests.
type packageResult struct {
	Package string
	Action  string
	Elapsed float64
}

func main() {
	n := flag.Int("n", 10, "number of slowest tests to print (0 for all)")
	jsonOut := flag.Bool("json", false, "print results as a JSON array instead of text")
	byPkg := flag.Bool("bypkg", false, "group and sum elapsed time by package instead of listing individual tests")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: slowtest [-n count] [-json] [-bypkg] [file]\n\n")
		fmt.Fprintf(os.Stderr, "reads go test -json output from file, or from stdin if no file is given\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	r, closeFn, err := openInput(flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "slowtest:", err)
		os.Exit(1)
	}
	defer closeFn()

	results, packages, err := parse(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, "slowtest:", err)
		os.Exit(1)
	}

	if *byPkg {
		printPackages(packages, *n, *jsonOut)
		return
	}

	if len(results) == 0 {
		fmt.Fprintln(os.Stderr, "slowtest: no test results found in input")
		os.Exit(1)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Elapsed > results[j].Elapsed })

	if *n > 0 && *n < len(results) {
		results = results[:*n]
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(results); err != nil {
			fmt.Fprintln(os.Stderr, "slowtest:", err)
			os.Exit(1)
		}
		return
	}

	for _, res := range results {
		fmt.Printf("%8.3fs  %-4s  %s  %s\n", res.Elapsed, res.Action, res.Package, res.Test)
	}
}

func printPackages(packages []packageResult, n int, jsonOut bool) {
	if len(packages) == 0 {
		fmt.Fprintln(os.Stderr, "slowtest: no package results found in input")
		os.Exit(1)
	}

	sort.Slice(packages, func(i, j int) bool { return packages[i].Elapsed > packages[j].Elapsed })

	if n > 0 && n < len(packages) {
		packages = packages[:n]
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(packages); err != nil {
			fmt.Fprintln(os.Stderr, "slowtest:", err)
			os.Exit(1)
		}
		return
	}

	for _, pkg := range packages {
		fmt.Printf("%8.3fs  %-4s  %s\n", pkg.Elapsed, pkg.Action, pkg.Package)
	}
}

// openInput picks stdin when no path is given (or the path is "-"), and a
// regular file otherwise. That's the one thing this tool needs to get right.
func openInput(args []string) (io.Reader, func() error, error) {
	if len(args) == 0 || args[0] == "-" {
		return os.Stdin, func() error { return nil }, nil
	}
	f, err := os.Open(args[0])
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}

// parse reads newline-delimited JSON test events and splits them into
// per-test results and per-package results. Build output and "run" events
// are dropped.
func parse(r io.Reader) ([]testResult, []packageResult, error) {
	var results []testResult
	var packages []packageResult
	dec := json.NewDecoder(bufio.NewReader(r))
	for {
		var ev testEvent
		err := dec.Decode(&ev)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("decoding test event: %w", err)
		}
		if ev.Test == "" {
			switch ev.Action {
			case "pass", "fail", "skip":
				packages = append(packages, packageResult{
					Package: ev.Package,
					Action:  ev.Action,
					Elapsed: ev.Elapsed,
				})
			}
			continue
		}
		switch ev.Action {
		case "pass", "fail", "skip":
			results = append(results, testResult{
				Package: ev.Package,
				Test:    ev.Test,
				Action:  ev.Action,
				Elapsed: ev.Elapsed,
			})
		}
	}
	return results, packages, nil
}
