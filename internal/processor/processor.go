package processor

import (
	"bytes"
	"errors"
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

const pkgErrors = "github.com/pkg/errors"

type fileVisitor struct {
	err         error
	needsErrors bool
	needsFmt    bool
	fixed       int
	failedToFix int
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
		log.Debugf("errors.%s is the same in pkg/errors and stdlib, skipping\n", selector.Sel.Name)
		v.fixed++
		v.needsErrors = true
	case "Errorf":
		log.Debug("errors.Errorf can be replaced with fmt.Errorf")
		selector.X = ast.NewIdent("fmt")
		v.needsFmt = true
		v.fixed++
	case "Wrap", "Wrapf":
		ok, err := v.fixWrap(call)
		if err != nil {
			v.err = errors.Join(v.err, err)
			return v
		}
		if !ok {
			v.failedToFix++
			return v
		}

		log.Debug("Replacing errors.Wrap with fmt.Errorf")
		v.needsFmt = true
		v.fixed++

	default:
		v.err = errors.Join(v.err, fmt.Errorf("unable to translate for %s", selector.Sel.Name))
	}
	return v
}

func (v *fileVisitor) fixWrap(call *ast.CallExpr) (bool, error) {
	selector := call.Fun.(*ast.SelectorExpr)
	if len(call.Args) < 2 {
		return false, errors.New("wrap call must have at least two args")
	}
	errToWrap, ok := call.Args[0].(*ast.Ident)
	if !ok {
		log.Warn("Cannot fix if first call to wrap is not an identifier")
		return false, nil
	}
	msgLit, ok := call.Args[1].(*ast.BasicLit)
	if !ok {
		log.Warn("Cannot fix if second call to wrap is not a literal")
		return false, nil
	}
	msg := msgLit.Value
	if msg[0] != '"' || msg[len(msg)-1] != '"' {
		log.Warn("Cannot fix if second call to wrap is not a string literal")
		return false, nil
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

	return true, nil
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

func fixFile(fset *token.FileSet, tree *ast.File) error {
	v := fileVisitor{}
	if !containsPkgErrors(fset, tree) {
		log.Debugf("Does not contain %s", pkgErrors)
		return nil
	}
	ast.Walk(&v, tree)
	if v.err != nil {
		return v.err
	}
	if v.failedToFix != 0 {
		log.Infof("Fixed %d, failed to fix %d", v.fixed, v.failedToFix)
		return nil
	}
	if v.needsErrors {
		astutil.AddImport(fset, tree, "errors")
	}
	if v.needsFmt {
		astutil.AddImport(fset, tree, "fmt")
	}
	log.Infof("Fixed %d references to pkg/errors", v.fixed)
	astutil.DeleteImport(fset, tree, pkgErrors)
	return nil
}

func Process(filename string, debug bool) ([]byte, error) {
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

	err = fixFile(fs, tree)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	format.Node(&buf, fs, tree)

	return buf.Bytes(), nil
}
