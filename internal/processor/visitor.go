package processor

import (
	"fmt"
	"go/ast"

	log "github.com/lukemassa/clilog"
)

const pkgErrors = "github.com/pkg/errors"

type fileVisitor struct {
	needsErrors        bool
	needsFmt           bool
	fixed              int
	failedToFixReasons []string
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
		ok := v.fixWrap(call)
		if !ok {
			return v
		}

		log.Debug("Replacing errors.Wrap with fmt.Errorf")
		v.needsFmt = true
		v.fixed++

	default:
		v.failedToFix("Cannot translate for errors.%s", selector.Sel.Name)
	}
	return v
}

func (v *fileVisitor) failedToFix(format string, args ...any) {
	v.failedToFixReasons = append(v.failedToFixReasons, fmt.Sprintf(format, args...))
}

func (v *fileVisitor) fixWrap(call *ast.CallExpr) bool {
	selector := call.Fun.(*ast.SelectorExpr)
	if len(call.Args) < 2 {
		v.failedToFix("Found call to errors.Wrap() with fewer than 2 args, existing code is not valid")
		return false
	}
	errToWrap, ok := call.Args[0].(*ast.Ident)
	if !ok {
		v.failedToFix("Cannot fix if first call to wrap is not an identifier")
		return false
	}

	fmtExpr, additionalArgs := v.getWrapArgs(call.Args[1:])
	if fmtExpr == nil {
		return false
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

	return true
}

// getWrapArgs takes all the args after the first to Wrap, i.e. after the error
// and returns what can be called by fmt.Errorf(), except the error
func (v *fileVisitor) getWrapArgs(additionalArgs []ast.Expr) (*ast.BasicLit, []ast.Expr) {

	msgLit, ok := additionalArgs[0].(*ast.BasicLit)
	if ok && isStringLiteral(msgLit) {
		return msgLit, additionalArgs[1:]
	}
	msgLit, remainingArgs := getWrapFmtPrintf(additionalArgs)
	if msgLit == nil {
		v.failedToFix("Cannot fix if second call to wrap is not a literal or a call to fmt.Sprintf()")
	}
	return msgLit, remainingArgs

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

func isStringLiteral(lit *ast.BasicLit) bool {
	litValue := lit.Value
	return litValue[0] == '"' && litValue[len(litValue)-1] == '"'
}
