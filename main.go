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
	err         error
	needsErrors bool
	needsFmt    bool
}

func (v *fileVisitor) Visit(n ast.Node) ast.Visitor {
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
		if *debug {
			fmt.Println("errors.Errorf can be replaced with fmt.Errorf")
		}
		selector.X = ast.NewIdent("fmt")
		v.needsFmt = true
	case "Wrap", "Wrapf":
		err := v.fixWrap(call)
		if err != nil {
			v.err = errors.Join(v.err, fmt.Errorf("could not convert Wrap: %v", err))
		} else if *debug {
			fmt.Println("Replacing errors.Wrap with fmt.Errorf")
		}
		v.needsFmt = true
	default:
		v.err = errors.Join(v.err, fmt.Errorf("unable to translate for %s", selector.Sel.Name))
	}
	return v
}

func (v *fileVisitor) fixWrap(call *ast.CallExpr) error {
	selector := call.Fun.(*ast.SelectorExpr)
	if len(call.Args) < 2 {
		return errors.New("wrap call must have at least two args")
	}
	errToWrap, ok := call.Args[0].(*ast.Ident)
	if !ok {
		return fmt.Errorf("first arg to Wrap is not identifier")
	}
	msgLit, ok := call.Args[1].(*ast.BasicLit)
	if !ok {
		return fmt.Errorf("second arg to Wrap is not a literal")
	}
	msg := msgLit.Value
	if msg[0] != '"' || msg[len(msg)-1] != '"' {
		return fmt.Errorf("second arg to Wrap is not a string literal")
	}
	// Update the string to include wrapped error
	msgLit.Value = msg[:len(msg)-1] + `: %w"`

	selector.X = ast.NewIdent("fmt")
	selector.Sel = ast.NewIdent("Errorf")
	newArgs := []ast.Expr{
		msgLit,
	}
	for i := 2; i < len(call.Args); i++ {
		newArgs = append(newArgs, call.Args[i])
	}
	newArgs = append(newArgs, errToWrap)
	call.Args = newArgs

	return nil
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
