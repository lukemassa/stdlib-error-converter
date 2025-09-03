package main

import (
	"os"

	"github.com/jessevdk/go-flags"
	log "github.com/lukemassa/clilog"

	"github.com/lukemassa/stdlib-error-converter/internal/processor"
)

func main() {
	var opts struct {
		Debug bool `long:"debug" description:"enable verbose debug logs"`
	}

	parser := flags.NewParser(&opts, flags.Default)
	args, err := parser.Parse()
	if err != nil {
		// If the user asked for help, go-flags already printed it.
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			os.Exit(0)
		}
		log.Fatal(err)
	}

	if opts.Debug {
		log.SetLogLevel(log.LevelDebug)
	}

	if len(args) == 0 {
		// Show help text and a concise usage error.
		parser.WriteHelp(os.Stderr)
		log.Fatal("missing file arguments (usage: stdlib-error-converter [--debug] <path> ...)")
	}

	for _, filename := range args {
		err := processor.ProcessFile(filename)
		if err != nil {
			log.Fatalf("processing %q: %v", filename, err)
		}
	}
}
