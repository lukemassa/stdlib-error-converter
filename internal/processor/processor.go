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
		ok, err := fixWrap(call)
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

func fixWrap(call *ast.CallExpr) (bool, error) {
	selector := call.Fun.(*ast.SelectorExpr)
	if len(call.Args) < 2 {
		return false, errors.New("wrap call must have at least two args")
	}
	errToWrap, ok := call.Args[0].(*ast.Ident)
	if !ok {
		log.Warn("Cannot fix if first call to wrap is not an identifier")
		return false, nil
	}

	fmtExpr, additionalArgs := getWrapArgs(call.Args[1:])
	if fmtExpr == nil {
		return false, nil
	}
	fmtString := fmtExpr.Value

	// Update the string to include wrapped error
	fmtExpr.Value = fmtString[:len(fmtString)-1] + `: %w"`

	// fmt.Errorf(fmt, args..., errToWrap)
	newArgs := []ast.Expr{fmtExpr}

	newArgs = append(newArgs, additionalArgs...)

	newArgs = append(newArgs, errToWrap)

	selector.X = ast.NewIdent("fmt")
	selector.Sel = ast.NewIdent("Errorf")
	call.Args = newArgs

	return true, nil
}

// getWrapArgs takes all the args after the first to Wrap, i.e. after the error
// and returns what can be called by fmt.Errorf(), except the error
func getWrapArgs(additionalArgs []ast.Expr) (*ast.BasicLit, []ast.Expr) {

	msgLit, ok := additionalArgs[0].(*ast.BasicLit)
	if ok && isStringLiteral(msgLit) {
		return msgLit, additionalArgs[1:]
	}
	msgLit, remainingArgs := getWrapFmtPrintf(additionalArgs)
	if msgLit == nil {
		log.Warn("Cannot fix if second call to wrap is not a literal or a call to fmt.Sprintf()")
	}
	return msgLit, remainingArgs

}
func isStringLiteral(lit *ast.BasicLit) bool {
	litValue := lit.Value
	return litValue[0] == '"' && litValue[len(litValue)-1] == '"'
}

// getWrapFmtPrintf is like getWrapArgs, but for the particular case where the second arg
// is fmt.Sprintf()
func getWrapFmtPrintf(additionalArgs []ast.Expr) (*ast.BasicLit, []ast.Expr) {

	if len(additionalArgs) != 1 {
		return nil, nil
	}

	funcCall, ok := additionalArgs[0].(*ast.CallExpr)
	if !ok {
		return nil, nil
	}
	if len(funcCall.Args) < 1 {
		return nil, nil
	}
	fmtSprintfCall, ok := funcCall.Fun.(*ast.SelectorExpr)
	if !ok || fmtSprintfCall.Sel.Name != "Sprintf" {
		return nil, nil
	}
	fmtIdent, ok := fmtSprintfCall.X.(*ast.Ident)
	if !ok || fmtIdent.Name != "fmt" {
		return nil, nil
	}

	msgLit, ok := funcCall.Args[0].(*ast.BasicLit)
	if !ok || !isStringLiteral(msgLit) {
		return nil, nil
	}

	// OK by the time we got here, we confirmed that additional args look exactly like:
	// [fmt.Sprintf("some string", ...otherargs)]
	// Then we can "fold them in" to the higher level function

	return msgLit, funcCall.Args[1:]
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

func fixFile(fset *token.FileSet, filename string, tree *ast.File) error {
	v := fileVisitor{}
	if !containsPkgErrors(fset, tree) {
		log.Debugf("%s: Does not contain %s", filename, pkgErrors)
		return nil
	}
	ast.Walk(&v, tree)
	if v.err != nil {
		return v.err
	}
	if v.failedToFix != 0 {
		log.Infof("%s: Fixed %d, failed to fix %d", filename, v.fixed, v.failedToFix)
		return nil
	}
	if v.needsErrors {
		astutil.AddImport(fset, tree, "errors")
	}
	if v.needsFmt {
		astutil.AddImport(fset, tree, "fmt")
	}
	log.Infof("%s: Fixed %d references to pkg/errors", filename, v.fixed)
	astutil.DeleteImport(fset, tree, pkgErrors)
	return nil
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

	err = fixFile(fs, filename, tree)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	format.Node(&buf, fs, tree)

	return buf.Bytes(), nil
}
