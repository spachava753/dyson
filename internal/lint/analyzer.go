package lint

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const doc = "checks that Starlark builtin callbacks return codec-supported values"

// Analyzer reports Starlark builtin callbacks that return concrete Starlark
// values Dyson's durable codec registry cannot serialize.
var Analyzer = &analysis.Analyzer{
	Name: "dysonlint",
	Doc:  doc,
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	unsupportedHelpers := collectUnsupportedHelpers(pass)
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || !isBuiltinCallback(pass, fn) {
				return true
			}
			checkBuiltin(pass, fn, unsupportedHelpers)
			return false
		})
	}
	return nil, nil
}

func collectUnsupportedHelpers(pass *analysis.Pass) map[*types.Func]ast.Expr {
	helpers := map[*types.Func]ast.Expr{}
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || isBuiltinCallback(pass, fn) || fn.Body == nil {
				return true
			}
			obj, ok := pass.TypesInfo.ObjectOf(fn.Name).(*types.Func)
			if !ok {
				return true
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				ret, ok := n.(*ast.ReturnStmt)
				if !ok || len(ret.Results) == 0 {
					return true
				}
				if isUnsupportedValueExpr(pass, ret.Results[0]) {
					helpers[obj] = ret.Results[0]
					return false
				}
				return true
			})
			return false
		})
	}
	return helpers
}

func isBuiltinCallback(pass *analysis.Pass, fn *ast.FuncDecl) bool {
	obj := pass.TypesInfo.ObjectOf(fn.Name)
	if obj == nil {
		return false
	}
	sig, ok := obj.Type().(*types.Signature)
	if !ok || sig.Params().Len() != 4 || sig.Results().Len() != 2 {
		return false
	}
	return hasName(sig.Results().At(0).Type(), "Value") && hasName(sig.Results().At(1).Type(), "error")
}

func hasName(typ types.Type, name string) bool {
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}
	if slice, ok := typ.(*types.Slice); ok {
		typ = slice.Elem()
	}
	if named, ok := typ.(*types.Named); ok {
		return named.Obj().Name() == name
	}
	return false
}

func checkBuiltin(pass *analysis.Pass, fn *ast.FuncDecl, unsupportedHelpers map[*types.Func]ast.Expr) {
	customLists := map[*types.Var]ast.Expr{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			rememberCustomList(pass, customLists, unsupportedHelpers, stmt)
		case *ast.ReturnStmt:
			checkReturn(pass, customLists, unsupportedHelpers, stmt)
		}
		return true
	})
}

func rememberCustomList(pass *analysis.Pass, customLists map[*types.Var]ast.Expr, unsupportedHelpers map[*types.Func]ast.Expr, stmt *ast.AssignStmt) {
	for i, rhs := range stmt.Rhs {
		call, ok := rhs.(*ast.CallExpr)
		if !ok || !isMakeStarlarkValueSlice(pass, call) {
			continue
		}
		if i >= len(stmt.Lhs) {
			continue
		}
		id, ok := stmt.Lhs[i].(*ast.Ident)
		if !ok {
			continue
		}
		obj, ok := pass.TypesInfo.ObjectOf(id).(*types.Var)
		if ok {
			customLists[obj] = nil
		}
	}

	for i, rhs := range stmt.Rhs {
		if i >= len(stmt.Lhs) || !isUnsupportedOrHelperValueExpr(pass, unsupportedHelpers, rhs) {
			continue
		}
		if idx, ok := stmt.Lhs[i].(*ast.IndexExpr); ok {
			if obj := identVar(pass, idx.X); obj != nil {
				if _, ok := customLists[obj]; ok {
					customLists[obj] = rhs
				}
			}
		}
	}

	for _, rhs := range stmt.Rhs {
		if !isUnsupportedOrHelperValueExpr(pass, unsupportedHelpers, rhs) {
			continue
		}
		for obj := range customLists {
			if objectUsed(pass, stmt.Lhs, obj) {
				customLists[obj] = rhs
			}
		}
	}
}

func checkReturn(pass *analysis.Pass, customLists map[*types.Var]ast.Expr, unsupportedHelpers map[*types.Func]ast.Expr, stmt *ast.ReturnStmt) {
	if len(stmt.Results) == 0 {
		return
	}
	value := stmt.Results[0]
	if isNil(value) {
		return
	}
	if isUnsupportedOrHelperValueExpr(pass, unsupportedHelpers, value) {
		pass.Reportf(value.Pos(), "starlark builtin returns %s, which is not registered with Dyson's codec registry", exprName(pass, value))
		return
	}
	if call, ok := value.(*ast.CallExpr); ok && isStarlarkNewList(pass, call) && len(call.Args) == 1 {
		if obj := identVar(pass, call.Args[0]); obj != nil {
			if cause, ok := customLists[obj]; ok && cause != nil {
				pass.Reportf(value.Pos(), "starlark builtin returns list containing %s, which is not registered with Dyson's codec registry", exprName(pass, cause))
			}
		}
	}
}

func isUnsupportedOrHelperValueExpr(pass *analysis.Pass, unsupportedHelpers map[*types.Func]ast.Expr, expr ast.Expr) bool {
	if isUnsupportedValueExpr(pass, expr) {
		return true
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	fun, ok := funcObject(pass, call.Fun)
	return ok && unsupportedHelpers[fun] != nil
}

func funcObject(pass *analysis.Pass, expr ast.Expr) (*types.Func, bool) {
	switch e := expr.(type) {
	case *ast.Ident:
		fn, ok := pass.TypesInfo.ObjectOf(e).(*types.Func)
		return fn, ok
	case *ast.SelectorExpr:
		fn, ok := pass.TypesInfo.Uses[e.Sel].(*types.Func)
		return fn, ok
	default:
		return nil, false
	}
}

func isUnsupportedValueExpr(pass *analysis.Pass, expr ast.Expr) bool {
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op.String() == "&" {
		return isUnsupportedConcreteValue(pass, pass.TypesInfo.TypeOf(expr))
	}
	if call, ok := expr.(*ast.CallExpr); ok {
		if isStarlarkStructFromStringDict(pass, call) {
			return true
		}
		return isUnsupportedConcreteValue(pass, pass.TypesInfo.TypeOf(call))
	}
	return isUnsupportedConcreteValue(pass, pass.TypesInfo.TypeOf(expr))
}

func isUnsupportedConcreteValue(pass *analysis.Pass, typ types.Type) bool {
	if typ == nil || isCodecSupportedType(typ) {
		return false
	}
	valueType := lookupStarlarkValue(pass)
	if valueType == nil || !types.Implements(typ, valueType) {
		return false
	}
	return !isInterface(typ)
}

func lookupStarlarkValue(pass *analysis.Pass) *types.Interface {
	for _, pkg := range pass.Pkg.Imports() {
		if pkg.Path() == "go.starlark.net/starlark" {
			obj := pkg.Scope().Lookup("Value")
			if named, ok := obj.Type().(*types.Named); ok {
				if iface, ok := named.Underlying().(*types.Interface); ok {
					return iface
				}
			}
		}
	}
	return nil
}

func isCodecSupportedType(typ types.Type) bool {
	name := types.TypeString(typ, nil)
	if strings.HasPrefix(name, "*") {
		name = strings.TrimPrefix(name, "*")
	}
	switch name {
	case "go.starlark.net/starlark.NoneType",
		"go.starlark.net/starlark.Bool",
		"go.starlark.net/starlark.Int",
		"go.starlark.net/starlark.Float",
		"go.starlark.net/starlark.String",
		"go.starlark.net/starlark.Bytes",
		"go.starlark.net/starlark.Tuple",
		"go.starlark.net/starlark.List",
		"go.starlark.net/starlark.Dict",
		"go.starlark.net/starlark.Set":
		return true
	}
	return false
}

func isInterface(typ types.Type) bool {
	_, ok := typ.Underlying().(*types.Interface)
	return ok
}

func isMakeStarlarkValueSlice(pass *analysis.Pass, call *ast.CallExpr) bool {
	fun, ok := call.Fun.(*ast.Ident)
	if !ok || fun.Name != "make" || len(call.Args) == 0 {
		return false
	}
	if types.TypeString(pass.TypesInfo.TypeOf(call), nil) == "[]go.starlark.net/starlark.Value" {
		return true
	}
	return strings.Contains(types.TypeString(pass.TypesInfo.TypeOf(call), nil), "[]") && strings.Contains(types.TypeString(pass.TypesInfo.TypeOf(call), nil), "starlark.Value")
}

func isStarlarkNewList(pass *analysis.Pass, call *ast.CallExpr) bool {
	return qualifiedFunc(pass, call, "go.starlark.net/starlark", "NewList")
}

func isStarlarkStructFromStringDict(pass *analysis.Pass, call *ast.CallExpr) bool {
	return qualifiedFunc(pass, call, "go.starlark.net/starlarkstruct", "FromStringDict")
}

func qualifiedFunc(pass *analysis.Pass, call *ast.CallExpr, pkgPath, name string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	obj := pass.TypesInfo.Uses[sel.Sel]
	if obj == nil || obj.Pkg() == nil {
		return false
	}
	return obj.Pkg().Path() == pkgPath
}

func identVar(pass *analysis.Pass, expr ast.Expr) *types.Var {
	id, ok := expr.(*ast.Ident)
	if !ok {
		return nil
	}
	obj, _ := pass.TypesInfo.ObjectOf(id).(*types.Var)
	return obj
}

func objectUsed(pass *analysis.Pass, exprs []ast.Expr, target *types.Var) bool {
	for _, expr := range exprs {
		used := false
		ast.Inspect(expr, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			if pass.TypesInfo.ObjectOf(id) == target || pass.TypesInfo.Uses[id] == target {
				used = true
				return false
			}
			return true
		})
		if used {
			return true
		}
	}
	return false
}

func isNil(expr ast.Expr) bool {
	id, ok := expr.(*ast.Ident)
	return ok && id.Name == "nil"
}

func exprName(pass *analysis.Pass, expr ast.Expr) string {
	if typ := pass.TypesInfo.TypeOf(expr); typ != nil {
		return types.TypeString(typ, nil)
	}
	return "unsupported starlark.Value"
}
