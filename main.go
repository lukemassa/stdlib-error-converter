package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"os"
)

type BarToBazVisitor struct {
	debug bool
}

func (v *BarToBazVisitor) Visit(n ast.Node) ast.Visitor {
	if call, ok := n.(*ast.CallExpr); ok {
		if fun, ok := call.Fun.(*ast.Ident); ok && fun.Name == "bar" {
			if v.debug {
				fmt.Printf("Replacing function call: %s -> baz\n", fun.Name)
			}
			fun.Name = "baz"
		}
	}
	return v
}

func processFile(filename string, debug bool) error {
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

	// Modify the AST using the visitor
	ast.Walk(&BarToBazVisitor{debug: debug}, tree)

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
	filename := "foo.go"
	debug := flag.Bool("debug", false, "enable debug output")
	flag.Parse()

	if err := processFile(filename, *debug); err != nil {
		log.Fatalf("error processing file: %v", err)
	}
}
