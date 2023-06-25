package analyzer

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

const (
	closeMethod = "Close"
)

func getTargetTypes(ssaPkg *ssa.Package, checkTypes []CheckType) []types.Type {
	targets := []types.Type{}

	for _, t := range checkTypes {
		pkg := ssaPkg.Prog.ImportedPackage(t.PkgPath)
		if pkg == nil {
			continue
		}

		pkgType := pkg.Type(t.TypeName)
		if pkgType == nil {
			continue
		}

		obj := pkgType.Object()
		targets = append(targets, obj.Type())
	}
	return targets
}

func isDeferClosure(c *ssa.MakeClosure) bool {
	for _, ref := range *c.Referrers() {
		if _, ok := ref.(*ssa.Defer); ok {
			return true
		}
	}
	return false
}

func receiverOfClose(call ssa.CallCommon) ssa.Value {
	if call.IsInvoke() {
		if call.Method == nil {
			return nil
		}
		if call.Method.Name() != closeMethod {
			return nil
		}
		return call.Value
	}

	if call.Value == nil {
		return nil
	}
	if call.Value.Name() != closeMethod {
		return nil
	}
	return call.Args[0]
}

func resolveInTypes(t types.Type, targetTypes []types.Type) bool {
	// dereference pointer, because we want to check both T.Close and (*T).Close
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	for _, targetType := range targetTypes {
		if types.Identical(t, targetType) {
			return true
		}
	}

	return false
}

func isNil(v ssa.Value) bool {
	c, ok := v.(*ssa.Const)
	if !ok {
		return false
	}
	return c.IsNil()
}

func deref(v ssa.Value) ssa.Value {
	v2, ok := v.(*ssa.UnOp)
	if !ok {
		return v
	}
	return deref(v2.X)
}
