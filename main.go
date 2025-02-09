package main

import (
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"os"

	"golang.org/x/tools/go/ast/astutil"
)

const pkgErrors = "github.com/pkg/errors"

var debug = flag.Bool("debug", false, "enable debug output")

type fileVisitor struct {
	err         error // More than one maybe?
	needsErrors bool
	needsFmt    bool
}

func (v *fileVisitor) Visit(n ast.Node) ast.Visitor {
	//fmt.Println(n)
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return v
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return v
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || pkg.Name != "errors" {
		return v
	}
	switch selector.Sel.Name {
	case "As", "Is", "New":
		if *debug {
			fmt.Printf("errors.%s is the same in pkg/errors and stdlib, skipping\n", selector.Sel.Name)
		}
		v.needsErrors = true
	case "Errorf":
		selector.X = ast.NewIdent("fmt")
		v.needsFmt = true
	default:
		v.err = errors.Join(v.err, fmt.Errorf("unable to translate for %s", selector.Sel.Name))
	}
	return v
}

func (v *fileVisitor) fixWrap(call ast.Node) {

}

func fixFile(fset *token.FileSet, tree *ast.File) error {
	v := fileVisitor{}
	ast.Walk(&v, tree)
	if v.err != nil {
		return v.err
	}
	if v.needsErrors {
		astutil.AddImport(fset, tree, "errors")
	}
	if v.needsFmt {
		astutil.AddImport(fset, tree, "fmt")
	}
	astutil.DeleteImport(fset, tree, pkgErrors)
	return nil
}

func processFile(filename string) error {
	fs := token.NewFileSet()

	// Read the file
	src, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse the source file
	tree, err := parser.ParseFile(fs, filename, src, parser.AllErrors)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	err = fixFile(fs, tree)
	if err != nil {
		return err
	}

	// Create a temporary file
	tempFilename := filename + ".tmp"
	f, err := os.Create(tempFilename)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer f.Close()

	if err := printer.Fprint(f, fs, tree); err != nil {
		return fmt.Errorf("failed to write modified code: %w", err)
	}

	// Replace the original file with the new one
	if err := os.Rename(tempFilename, filename); err != nil {
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}

	return nil
}

func main() {
	filename := "tests/as.go"
	flag.Parse()

	if err := processFile(filename); err != nil {
		log.Fatalf("error processing file: %v", err)
	}
}
