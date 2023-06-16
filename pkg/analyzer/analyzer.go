package analyzer

import (
	"fmt"
	"go/token"
	"go/types"
	"log"
	"sync"

	"golang.org/x/exp/slices"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

type CheckType struct {
	PkgPath string
	Name    string
}

type closeTracker struct {
	checkTypes              []CheckType
	mustClose               map[*ssa.BasicBlock][]*checkItem
	doneCheckItemFromParam  map[*ssa.Function]struct{}
	doneCheckItemFromInstrs map[*ssa.Function]struct{}

	runLock sync.Mutex
}

func NewAnalyzer(types []CheckType) *analysis.Analyzer {
	t := &closeTracker{
		checkTypes:              types,
		mustClose:               make(map[*ssa.BasicBlock][]*checkItem),
		doneCheckItemFromParam:  make(map[*ssa.Function]struct{}),
		doneCheckItemFromInstrs: make(map[*ssa.Function]struct{}),
	}
	return &analysis.Analyzer{
		Name: "forgetclose",
		Doc:  "Did you forget to call Close?",
		Run:  t.run,
		Requires: []*analysis.Analyzer{
			buildssa.Analyzer,
		},
	}
}

func (t *closeTracker) run(pass *analysis.Pass) (interface{}, error) {
	t.runLock.Lock()
	defer t.runLock.Unlock()

	pssa, ok := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	if !ok {
		return nil, fmt.Errorf("no SSA found, got type %T", pass.ResultOf[buildssa.Analyzer])
	}

	targetTypes := getTargetTypes(pssa.Pkg, t.checkTypes)

	if len(targetTypes) == 0 {
		return nil, nil
	}

	for _, f := range pssa.SrcFuncs {
		t.checkFunc(pass, f, targetTypes)
	}
	return nil, nil
}

type checkItem struct {
	// v is the value to be checked
	v        ssa.Value
	tp       string
	pos      token.Pos
	refs     []ssa.Instruction
	closed   *bool
	reported *bool
	// ctorErr is the error of `v, err := NewXXX()`. If ctorErr is not nil, we don't
	// need to close v.
	ctorErr ssa.Value
}

func (i *checkItem) markClosed() {
	*i.closed = true
}

func (i *checkItem) report(pass *analysis.Pass) {
	pass.Reportf(i.pos, "%s not closed!", i.tp)
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

func (t *closeTracker) checkFunc(
	pass *analysis.Pass,
	f *ssa.Function,
	targetTypes []types.Type,
	presetItems ...*checkItem,
) {
	defer func() {
		t.doneCheckItemFromInstrs[f] = struct{}{}
	}()

	for i, b := range f.DomPreorder() {
		log.Printf("block: %p %s", b, b)
		if len(presetItems) > 0 {
			if _, ok := t.doneCheckItemFromParam[f]; !ok && i == 0 {
				t.mustClose[b] = append(t.mustClose[b], presetItems...)
				t.doneCheckItemFromParam[f] = struct{}{}
			}
		}

		if _, ok := t.doneCheckItemFromInstrs[f]; !ok {
			for _, instr := range b.Instrs {
				val, ok := instr.(ssa.Value)
				if !ok {
					log.Printf("instr: %T %s", instr, instr)
				} else {
					log.Printf("instr(%s): %T %s", val.Name(), instr, instr)
				}
				item := getCheckItemFromCall(instr, targetTypes)
				if item != nil {
					t.mustClose[b] = append(t.mustClose[b], item)
				}
			}
		}

	mustCloseLoop:
		for _, item := range t.mustClose[b] {
			if *item.reported {
				continue mustCloseLoop
			}

			refsInBlock := item.popInstrsOfBlock(b)
			for i, ref := range refsInBlock {
				val, ok := ref.(ssa.Value)
				if !ok {
					log.Printf("ref: %T %s", ref, ref)
				} else {
					log.Printf("ref(%s): %T %s", val.Name(), ref, ref)
				}
				switch v := ref.(type) {
				case *ssa.Defer:
					if item.v == receiverOfClose(v.Call) {
						item.markClosed()
						continue mustCloseLoop
					}
					// used as a parameter of defer
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
						tp:       item.tp,
						pos:      item.pos,
						closed:   item.closed,
						reported: item.reported,
					}
					presetCheckItem.refs = *presetCheckItem.v.Referrers()
					t.checkFunc(pass, f, targetTypes, presetCheckItem)
					if *presetCheckItem.closed || *presetCheckItem.reported {
						continue mustCloseLoop
					}
				case *ssa.Call:
					if item.v == receiverOfClose(v.Call) {
						item.markClosed()
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
						tp:       item.tp,
						pos:      item.pos,
						closed:   item.closed,
						reported: item.reported,
					}
					presetCheckItem.refs = *presetCheckItem.v.Referrers()
					t.checkFunc(pass, f, targetTypes, presetCheckItem)
					item.markClosed()
					continue mustCloseLoop
				case *ssa.Store:
					// check if it's closed by a defer closure
					storedTo := v.Addr
					if _, ok := storedTo.(*ssa.FieldAddr); ok {
						// blindly assume it's closed after assign it to a field of struct
						item.markClosed()
						continue mustCloseLoop
					}
					for _, ref := range *storedTo.Referrers() {
						makeClosure, ok := ref.(*ssa.MakeClosure)
						if !ok {
							continue
						}
						if !isDeferClosure(makeClosure) {
							continue
						}
						presetCheckItem := &checkItem{
							pos:      item.pos,
							closed:   item.closed,
							reported: item.reported,
						}

						fn := makeClosure.Fn.(*ssa.Function)
						idx := slices.Index(makeClosure.Bindings, storedTo)
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
							presetCheckItem.tp = unOp.Type().String()
							break
						}

						presetCheckItem.refs = *presetCheckItem.v.Referrers()
						t.checkFunc(pass, fn, targetTypes, presetCheckItem)
						item.markClosed()
						continue mustCloseLoop
					}
				case *ssa.MakeClosure:
					if isDeferClosure(v) {
						item.markClosed()
						continue mustCloseLoop
					}
				case *ssa.Return:
					// caller will find the target by getCheckItemFromCall
					item.markClosed()
					continue mustCloseLoop
				}
			}
			if !t.trySpreadToSuccBlocks(b, item) {
				item.report(pass)
			}
		}
		t.mustClose[b] = nil
	}
}

// trySpreadCtorErrBranch handles the if `v, err := NewXXX(); err != nil` branch
func (t *closeTracker) trySpreadCtorErrBranch(b *ssa.BasicBlock, item *checkItem) bool {
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
		t.mustClose[b.Succs[1]] = append(t.mustClose[b.Succs[1]], item)
		return true
	case token.EQL:
		if cond.X != item.ctorErr && cond.Y != item.ctorErr {
			return false
		}
		t.mustClose[b.Succs[0]] = append(t.mustClose[b.Succs[0]], item)
		return true
	}
	return false
}

func (t *closeTracker) trySpreadToSuccBlocks(b *ssa.BasicBlock, item *checkItem) bool {
	if len(b.Succs) == 0 {
		return false
	}
	if len(item.refs) == 0 {
		return false
	}

	if t.trySpreadCtorErrBranch(b, item) {
		return true
	}
	for _, succBlock := range b.Succs {
		t.mustClose[succBlock] = append(t.mustClose[succBlock], item)
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
		targetTp   string
	)
	for i := 0; i < results.Len(); i++ {
		tp := results.At(i).Type()
		if types.Identical(tp, errTp) {
			ctorErrIdx = i
		}

		if resolveInTypes(tp, targetTypes) {
			targetTp = tp.String()
			targetIdx = i
		}
	}

	if targetIdx == -1 {
		return nil
	}

	for _, ref := range *call.Referrers() {
		switch instr := ref.(type) {
		case *ssa.Extract:
			switch instr.Index {
			case targetIdx:
				target = instr
			case ctorErrIdx:
				ctorErr = instr
			}
		case *ssa.Call:
			target = call
		}
	}

	ret := &checkItem{
		v:        target,
		tp:       targetTp,
		pos:      call.Pos(),
		closed:   new(bool),
		reported: new(bool),
	}
	if ret.v != nil && ret.v.Referrers() != nil {
		ret.refs = *ret.v.Referrers()
	}
	if ctorErrIdx != -1 {
		ret.ctorErr = ctorErr
	}
	return ret
}
