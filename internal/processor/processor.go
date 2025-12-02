package processor

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strings"

	log "github.com/lukemassa/clilog"
	"golang.org/x/tools/go/ast/astutil"
)

type result struct {
	content            []byte
	failedToFixReasons []string
}

func containsPkgErrors(fset *token.FileSet, tree *ast.File) bool {
	for _, paragraph := range astutil.Imports(fset, tree) {
		for _, importSpec := range paragraph {
			if strings.Trim(importSpec.Path.Value, "\"") == pkgErrors {
				return true
			}
		}
	}
	return false
}

func fixFile(fset *token.FileSet, filename string, tree *ast.File) []string {
	v := fileVisitor{}
	if !containsPkgErrors(fset, tree) {
		log.Debugf("%s: Does not contain %s", filename, pkgErrors)
		return v.failedToFixReasons
	}
	ast.Walk(&v, tree)
	if len(v.failedToFixReasons) != 0 {
		log.Infof("%s: Fixed %d, failed to fix %d", filename, v.fixed, len(v.failedToFixReasons))
		return v.failedToFixReasons
	}
	if v.needsErrors {
		astutil.AddImport(fset, tree, "errors")
	}
	if v.needsFmt {
		astutil.AddImport(fset, tree, "fmt")
	}
	log.Infof("%s: Fixed %d references to pkg/errors", filename, v.fixed)
	astutil.DeleteImport(fset, tree, pkgErrors)
	return v.failedToFixReasons
}

func Process(filename string) ([]byte, error) {

	// external caller don't care about failedToFixReasons, that's just for our testing
	// we already log the actual reasons as we go
	result, err := process(filename)
	return result.content, err
}

func process(filename string) (result, error) {
	fs := token.NewFileSet()

	// Read the file
	src, err := os.ReadFile(filename)
	if err != nil {
		return result{}, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse the source file
	tree, err := parser.ParseFile(fs, filename, src, parser.AllErrors|parser.ParseComments)
	if err != nil {
		return result{}, fmt.Errorf("failed to parse file: %w", err)
	}

	failedToFixReasons := fixFile(fs, filename, tree)

	var buf bytes.Buffer
	format.Node(&buf, fs, tree)

	return result{
		content:            buf.Bytes(),
		failedToFixReasons: failedToFixReasons,
	}, nil
}
