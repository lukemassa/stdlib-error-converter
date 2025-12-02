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

func fixFile(fset *token.FileSet, filename string, tree *ast.File) {
	v := fileVisitor{}
	if !containsPkgErrors(fset, tree) {
		log.Debugf("%s: Does not contain %s", filename, pkgErrors)
		return
	}
	ast.Walk(&v, tree)
	if len(v.failedToFixReasons) != 0 {
		log.Infof("%s: Fixed %d, failed to fix %d", filename, v.fixed, len(v.failedToFixReasons))
		return
	}
	if v.needsErrors {
		astutil.AddImport(fset, tree, "errors")
	}
	if v.needsFmt {
		astutil.AddImport(fset, tree, "fmt")
	}
	log.Infof("%s: Fixed %d references to pkg/errors", filename, v.fixed)
	astutil.DeleteImport(fset, tree, pkgErrors)
}

func Process(filename string) ([]byte, error) {
	fs := token.NewFileSet()

	// Read the file
	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse the source file
	tree, err := parser.ParseFile(fs, filename, src, parser.AllErrors|parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	fixFile(fs, filename, tree)

	var buf bytes.Buffer
	format.Node(&buf, fs, tree)

	return buf.Bytes(), nil
}
