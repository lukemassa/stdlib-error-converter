package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/lukemassa/clilog"

	"github.com/lukemassa/unpkg-errors/internal/processor"
)

func gatherFiles(paths []string, recursive bool) ([]string, error) {
	var files []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("stating %s: %v", path, err)
		}
		if !info.IsDir() {
			if filepath.Ext(path) == ".go" {
				files = append(files, path)
			}
			continue
		}
		if recursive {
			filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				if filepath.Ext(p) == ".go" {
					files = append(files, p)
				}
				return nil
			})
			continue
		}

		// only process top-level .go files
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %v", path, err)
		}
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".go" {
				files = append(files, filepath.Join(path, e.Name()))
			}
		}
	}
	return files, nil
}

func main() {
	// ---- Flags ----
	write := flag.Bool("w", false, "Write result to source files instead of stdout")
	recursive := flag.Bool("r", false, "Recurse into directories")
	verbose := flag.Bool("v", false, "Print file names as they are processed")
	debug := flag.Bool("d", false, "Print decision-level debug info to stderr")

	// allow long versions
	flag.BoolVar(write, "write", *write, "Same as -w")
	flag.BoolVar(recursive, "recursive", *recursive, "Same as -r")
	flag.BoolVar(verbose, "verbose", *verbose, "Same as -v")
	flag.BoolVar(debug, "debug", *debug, "Same as -d")

	flag.Parse()
	paths := flag.Args()

	if *debug {
		log.SetLogLevel(log.LevelDebug)
	}

	if len(paths) == 0 {
		paths = []string{"."} // default: current directory
	}
	files, err := gatherFiles(paths, *recursive)
	if err != nil {
		log.Fatal(err)
	}

	if len(files) == 0 {
		os.Exit(0)
	}

	// ---- Process files ----
	for _, f := range files {
		if *verbose {
			log.Infof("Processing %s", f)
		}

		modified, err := processor.Process(f)
		if err != nil {
			log.Fatalf("Processing %s: %v", f, err)
		}

		if *write {
			err = os.WriteFile(f, modified, 0o644)
			if err != nil {
				log.Fatalf("Writing %s: %v", f, err)
				os.Exit(2)
			}
		} else {
			_, err := os.Stdout.Write(modified)
			if err != nil {
				log.Fatalf("Writing to stdout failed: %v", err)
			}
		}
	}
}
