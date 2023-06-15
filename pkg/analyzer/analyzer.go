package analyzer

import (
	"fmt"
	"go/token"
	"go/types"

	"golang.org/x/exp/slices"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

// TODO: not elegant, move to argument of NewAnalyzer
var (
	PackageAndTypes = [][2]string{
		{"database/sql", "Rows"},
	}
)

func NewAnalyzer() *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "forgetclose",
		Doc:  "Did you forget to call Close?",
		Run:  run,
		Requires: []*analysis.Analyzer{
			buildssa.Analyzer,
		},
	}
}

func run(pass *analysis.Pass) (interface{}, error) {
	pssa, ok := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	if !ok {
		return nil, fmt.Errorf("no SSA found, got type %T", pass.ResultOf[buildssa.Analyzer])
	}

	targetTypes := getTargetTypes(pssa.Pkg)

	if len(targetTypes) == 0 {
		return nil, nil
	}

	for _, f := range pssa.SrcFuncs {
		checkFunc(pass, f, targetTypes)
	}
	return nil, nil
}

type checkItem struct {
	// v is the value to be checked
	v        ssa.Value
	pos      token.Pos
	refs     []ssa.Instruction
	reported *bool
	// ctorErr is the error of `v, err := NewXXX()`. If ctorErr is not nil, we don't
	// need to close v.
	ctorErr ssa.Value
}

func (i *checkItem) report(pass *analysis.Pass) {
	pass.Reportf(i.pos, "%s not closed!", i.v.Type())
	*i.reported = true
}

func (i *checkItem) popInstrsOfBlock(b *ssa.BasicBlock) []ssa.Instruction {
	remain := make([]ssa.Instruction, 0, len(i.refs)/2)
	ret := make([]ssa.Instruction, 0, len(i.refs)/2)
	for _, ref := range i.refs {
		if ref.Block() == b {
			ret = append(ret, ref)
		} else {
			remain = append(remain, ref)
		}
	}

	i.refs = remain
	return ret
}

// TODO: move to member of analyzer
var (
	mustClose               = map[*ssa.BasicBlock][]*checkItem{}
	doneCheckItemFromParam  = map[*ssa.Function]struct{}{}
	doneCheckItemFromInstrs = map[*ssa.Function]struct{}{}
)

func checkFunc(
	pass *analysis.Pass,
	f *ssa.Function,
	targetTypes []types.Type,
	presetItems ...*checkItem,
) {
	defer func() {
		doneCheckItemFromInstrs[f] = struct{}{}
	}()

	for i, b := range f.DomPreorder() {
		if len(presetItems) > 0 {
			if _, ok := doneCheckItemFromParam[f]; !ok && i == 0 {
				mustClose[b] = append(mustClose[b], presetItems...)
				doneCheckItemFromParam[f] = struct{}{}
			}
		}

		if _, ok := doneCheckItemFromInstrs[f]; !ok {
			for _, instr := range b.Instrs {
				item := getCheckItemFromCall(instr, targetTypes)
				if item != nil {
					mustClose[b] = append(mustClose[b], item)
				}
			}
		}

	mustCloseLoop:
		for _, item := range mustClose[b] {
			if *item.reported {
				continue mustCloseLoop
			}

			refsInBlock := item.popInstrsOfBlock(b)
			for i, ref := range refsInBlock {
				switch v := ref.(type) {
				case *ssa.Defer:
					if item.v == receiverOfClose(v.Call) {
						continue mustCloseLoop
					}
				case *ssa.Call:
					if item.v == receiverOfClose(v.Call) {
						continue mustCloseLoop
					}

					if i != len(refsInBlock)-1 {
						continue
					}
					// for the last call, maybe the value is closed inside the function
					f := v.Call.StaticCallee()
					argIdx := slices.Index(v.Call.Args, item.v)
					if argIdx >= len(f.Params) {
						// functions whose source code cannot be accessed will have zero params
						if len(f.Params) == 0 {
							continue
						}
					}
					presetCheckItem := &checkItem{
						v:        f.Params[argIdx],
						pos:      item.pos,
						reported: item.reported,
					}
					presetCheckItem.refs = *presetCheckItem.v.Referrers()
					checkFunc(pass, f, targetTypes, presetCheckItem)
					continue mustCloseLoop
				case *ssa.Store:
					// check if it's closed by a defer closure
					maybeClosureCapture := v.Addr
					for _, ref := range *maybeClosureCapture.Referrers() {
						makeClosure, ok := ref.(*ssa.MakeClosure)
						if !ok {
							continue
						}
						if !isDeferClosure(makeClosure) {
							continue
						}
						presetCheckItem := &checkItem{
							pos:      item.pos,
							reported: item.reported,
						}

						fn := makeClosure.Fn.(*ssa.Function)
						idx := slices.Index(makeClosure.Bindings, maybeClosureCapture)
						captureVar := fn.FreeVars[idx]

						for _, ref := range *captureVar.Referrers() {
							unOp, ok := ref.(*ssa.UnOp)
							if !ok {
								continue
							}
							if unOp.Op != token.MUL {
								continue
							}
							presetCheckItem.v = unOp
							break
						}

						presetCheckItem.refs = *presetCheckItem.v.Referrers()
						checkFunc(pass, fn, targetTypes, presetCheckItem)
						continue mustCloseLoop
					}
				case *ssa.MakeClosure:
					if isDeferClosure(v) {
						continue mustCloseLoop
					}
				}
			}
			if !trySpreadToSuccBlocks(b, item) {
				item.report(pass)
			}
		}
		mustClose[b] = nil
	}
}

// trySpreadCtorErrBranch handles the if `v, err := NewXXX(); err != nil` branch
func trySpreadCtorErrBranch(b *ssa.BasicBlock, item *checkItem) bool {
	if item.ctorErr == nil {
		return false
	}

	lastInstr := b.Instrs[len(b.Instrs)-1]
	ifInstr, ok := lastInstr.(*ssa.If)
	if !ok {
		return false
	}
	cond, ok := ifInstr.Cond.(*ssa.BinOp)
	if !ok {
		return false
	}
	switch cond.Op {
	case token.NEQ:
		if cond.X != item.ctorErr && cond.Y != item.ctorErr {
			return false
		}
		mustClose[b.Succs[1]] = append(mustClose[b.Succs[1]], item)
		return true
	case token.EQL:
		if cond.X != item.ctorErr && cond.Y != item.ctorErr {
			return false
		}
		mustClose[b.Succs[0]] = append(mustClose[b.Succs[0]], item)
		return true
	}
	return false
}

func trySpreadToSuccBlocks(b *ssa.BasicBlock, item *checkItem) bool {
	if len(b.Succs) == 0 {
		return false
	}

	if trySpreadCtorErrBranch(b, item) {
		return true
	}
	for _, succBlock := range b.Succs {
		mustClose[succBlock] = append(mustClose[succBlock], item)
	}
	return true
}

var errTp = types.Universe.Lookup("error").Type()

func getCheckItemFromCall(instr ssa.Instruction, targetTypes []types.Type) *checkItem {
	call, ok := instr.(*ssa.Call)
	if !ok {
		return nil
	}

	results := call.Call.Signature().Results()
	if results.Len() == 0 {
		return nil
	}

	// iterate the types of the results to find the index, them use index to get the
	// SSA value

	var (
		targetIdx  = -1
		ctorErrIdx = -1
		target     ssa.Value
		ctorErr    ssa.Value
	)
	for i := 0; i < results.Len(); i++ {
		tp := results.At(i).Type()
		if types.Identical(tp, errTp) {
			ctorErrIdx = i
		}

		if resolveInTypes(tp, targetTypes) {
			targetIdx = i
		}
	}

	if targetIdx == -1 {
		return nil
	}

	for _, ref := range *call.Referrers() {
		switch i := ref.(type) {
		case *ssa.Extract:
			switch i.Index {
			case targetIdx:
				target = i
			case ctorErrIdx:
				ctorErr = i
			}
		default:
			// TODO: check it
		}
	}

	ret := &checkItem{
		v:        target,
		pos:      call.Pos(),
		reported: new(bool),
	}
	if ret.v.Referrers() != nil {
		ret.refs = *ret.v.Referrers()
	}
	if ctorErrIdx != -1 {
		ret.ctorErr = ctorErr
	}
	return ret
}
