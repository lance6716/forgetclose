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

const isDebug = false

var debugMu sync.Mutex

type CheckType struct {
	PkgPath    string
	StructName string
}

type closeTracker struct {
	checkTypes              []CheckType
	checkBlock              map[*ssa.BasicBlock][]*checkItem
	doneCheckFuncFromParam  map[*ssa.Function]struct{}
	doneCheckFuncFromInstrs map[*ssa.Function]struct{}
}

func NewAnalyzer(types []CheckType) *analysis.Analyzer {
	run := func(pass *analysis.Pass) (interface{}, error) {
		if isDebug {
			debugMu.Lock()
			defer debugMu.Unlock()
		}
		t := &closeTracker{
			checkTypes:              types,
			checkBlock:              make(map[*ssa.BasicBlock][]*checkItem),
			doneCheckFuncFromParam:  make(map[*ssa.Function]struct{}),
			doneCheckFuncFromInstrs: make(map[*ssa.Function]struct{}),
		}
		return t.run(pass)
	}
	return &analysis.Analyzer{
		Name: "forgetclose",
		Doc:  "Did you forget to call Close?",
		Run:  run,
		Requires: []*analysis.Analyzer{
			buildssa.Analyzer,
		},
	}
}

func (t *closeTracker) run(pass *analysis.Pass) (interface{}, error) {
	debug(func() {
		log.Printf("run: %s", pass.Pkg.Path())
	})
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
	// v is the value to be checked, closeTracker will look the referee of v to find
	// Close() call. Note that the Close() may be called at *v.
	v        ssa.Value
	tp       string
	pos      token.Pos
	refs     []ssa.Instruction
	closed   *bool
	reported *bool
	// ifErr is the error of `v, err := NewXXX()`. If ifErr is not nil, we don't need
	// to close v.
	ifErr ssa.Value
}

func (i *checkItem) markClosed() {
	*i.closed = true
}

func (i *checkItem) report(pass *analysis.Pass) {
	// TODO: add leak position to message
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
		t.doneCheckFuncFromInstrs[f] = struct{}{}
	}()

	// use dominator tree order so no need to look back
	for i, b := range f.DomPreorder() {
		debug(func() {
			log.Printf("block: %p %s", b, b)
		})
		if len(presetItems) > 0 {
			if _, ok := t.doneCheckFuncFromParam[f]; !ok && i == 0 {
				t.checkBlock[b] = append(t.checkBlock[b], presetItems...)
				t.doneCheckFuncFromParam[f] = struct{}{}
			}
		}

		if _, ok := t.doneCheckFuncFromInstrs[f]; !ok {
			for _, instr := range b.Instrs {
				debug(func() {
					val, ok := instr.(ssa.Value)
					if !ok {
						log.Printf("instr: %T %s", instr, instr)
					} else {
						log.Printf("instr(%s): %T %s", val.Name(), instr, instr)
					}
				})
				item := getCheckItemFromCall(instr, targetTypes)
				if item != nil {
					t.checkBlock[b] = append(t.checkBlock[b], item)
				}
			}
		}

	checkBlockLoop:
		for _, item := range t.checkBlock[b] {
			if *item.reported {
				continue checkBlockLoop
			}

			refsInBlock := item.popInstrsOfBlock(b)

			for i, ref := range refsInBlock {
				isLast := i == len(refsInBlock)-1
				if t.checkRefIsClosed(pass, ref, isLast, item, targetTypes) {
					continue checkBlockLoop
				}
			}
			if !t.trySpreadToSuccBlocks(b, item) {
				item.report(pass)
			}
		}
		t.checkBlock[b] = nil
	}
}

func (t *closeTracker) checkRefIsClosed(
	pass *analysis.Pass,
	ref ssa.Instruction,
	isLastRef bool,
	item *checkItem,
	targetTypes []types.Type,
) (shouldSkip bool) {
	debug(func() {
		val, ok := ref.(ssa.Value)
		if !ok {
			log.Printf("ref: %T %s", ref, ref)
		} else {
			log.Printf("ref(%s): %T %s", val.Name(), ref, ref)
		}
	})

	switch v := ref.(type) {
	case *ssa.UnOp:
		// for simplicity, when we are inside closure, defer all the capture
		if v.Op != token.MUL {
			return false
		}

		refs := *v.Referrers()
		newItem := &checkItem{
			v:        v,
			tp:       item.tp,
			pos:      item.pos,
			closed:   item.closed,
			reported: item.reported,
			refs:     refs,
		}
		for i, ref := range refs {
			isLast := i == len(refs)-1

			if t.checkRefIsClosed(pass, ref, isLast, newItem, targetTypes) {
				return true
			}
		}
	case *ssa.Defer:
		if item.v == receiverOfClose(v.Call) {
			item.markClosed()
			return true
		}
		// used as a parameter of defer
		t.checkFuncCheckArg(pass, v.Call, item, targetTypes)
		if *item.closed || *item.reported {
			return true
		}
	case *ssa.Call:
		if item.v == receiverOfClose(v.Call) {
			item.markClosed()
			return true
		}
		if !isLastRef {
			return false
		}
		// for the last call, maybe the value is closed inside the function
		if t.checkFuncCheckArg(pass, v.Call, item, targetTypes) {
			return true
		}
	case *ssa.Store:
		// check if it's closed by a defer closure
		storedTo := v.Addr
		if _, ok := storedTo.(*ssa.FieldAddr); ok {
			// blindly assume it's closed after assign it to a field of struct
			item.markClosed()
			return true
		}
		for _, ref := range *storedTo.Referrers() {
			makeClosure, ok := ref.(*ssa.MakeClosure)
			if !ok {
				continue
			}
			if !isDeferClosure(makeClosure) {
				continue
			}

			fn := makeClosure.Fn.(*ssa.Function)
			idx := slices.Index(makeClosure.Bindings, storedTo)
			captureVar := fn.FreeVars[idx]

			presetCheckItem := &checkItem{
				v:        captureVar,
				tp:       item.tp,
				pos:      item.pos,
				closed:   item.closed,
				reported: item.reported,
			}

			presetCheckItem.refs = *presetCheckItem.v.Referrers()
			// generally Close is the last call, so if ref is inside defer we only need to
			// check this ref
			t.checkFunc(pass, fn, targetTypes, presetCheckItem)
			return true
		}
	case *ssa.MakeClosure:
		if isDeferClosure(v) {
			item.markClosed()
			return true
		}
	case *ssa.Return:
		// caller will find the target by getCheckItemFromCall
		item.markClosed()
		return true
	}
	return false
}

func (t *closeTracker) checkFuncCheckArg(
	pass *analysis.Pass,
	call ssa.CallCommon,
	item *checkItem,
	targetTypes []types.Type,
) (ok bool) {
	var f *ssa.Function
	dynCall, ok := call.Value.(*ssa.Call)
	if !ok {
		f = call.StaticCallee()
	} else {
		f = dynCall.Call.StaticCallee()
	}

	argIdx := slices.Index(call.Args, item.v)
	if argIdx == -1 {
		return false
	}
	if argIdx >= len(f.Params) {
		// TODO: functions whose source code cannot be accessed will have zero params
		if len(f.Params) == 0 {
			return false
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
	return true
}

func (t *closeTracker) trySpreadToSuccBlocks(
	b *ssa.BasicBlock,
	item *checkItem,
) (ok bool) {
	if len(b.Succs) == 0 {
		return false
	}
	if len(item.refs) == 0 {
		return false
	}

	if t.trySpreadIfErrBranch(b, item) {
		return true
	}
	if t.trySpreadNilCheckBranch(b, item) {
		return true
	}
	for _, succBlock := range b.Succs {
		t.checkBlock[succBlock] = append(t.checkBlock[succBlock], item)
	}
	return true
}

// trySpreadIfErrBranch handles the `if v, err := NewXXX(); err != nil` branch
func (t *closeTracker) trySpreadIfErrBranch(
	b *ssa.BasicBlock,
	item *checkItem,
) (ok bool) {
	if item.ifErr == nil {
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

	x := deref(cond.X)
	y := deref(cond.Y)

	switch cond.Op {
	case token.NEQ:
		if x != item.ifErr && y != item.ifErr {
			return false
		}
		t.checkBlock[b.Succs[1]] = append(t.checkBlock[b.Succs[1]], item)
		return true
	case token.EQL:
		if x != item.ifErr && y != item.ifErr {
			return false
		}
		t.checkBlock[b.Succs[0]] = append(t.checkBlock[b.Succs[0]], item)
		return true
	}
	return false
}

// trySpreadNilCheckBranch handles the `if v, err := NewXXX(); v != nil` branch
func (t *closeTracker) trySpreadNilCheckBranch(
	b *ssa.BasicBlock,
	item *checkItem,
) (ok bool) {
	lastInstr := b.Instrs[len(b.Instrs)-1]
	ifInstr, ok := lastInstr.(*ssa.If)
	if !ok {
		return false
	}
	cond, ok := ifInstr.Cond.(*ssa.BinOp)
	if !ok {
		return false
	}

	x := deref(cond.X)
	y := deref(cond.Y)

	switch cond.Op {
	case token.NEQ:
		if x != item.v && y != item.v {
			return false
		}
		if !isNil(cond.X) && !isNil(cond.Y) {
			return false
		}
		t.checkBlock[b.Succs[0]] = append(t.checkBlock[b.Succs[0]], item)
		return true
	case token.EQL:
		if x != item.v && y != item.v {
			return false
		}
		if !isNil(cond.X) && !isNil(cond.Y) {
			return false
		}
		t.checkBlock[b.Succs[1]] = append(t.checkBlock[b.Succs[1]], item)
		return true
	}
	return false
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
		targetIdx = -1
		ifErrIdx  = -1
		target    ssa.Value
		ifErr     ssa.Value
		targetTp  string
	)
	for i := 0; i < results.Len(); i++ {
		tp := results.At(i).Type()
		if types.Identical(tp, errTp) {
			ifErrIdx = i
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
			case ifErrIdx:
				ifErr = instr
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
	if ifErrIdx != -1 {
		// TODO: ugly check Store to named return. Can we unify the processing for v?
		ret.ifErr = ifErr
		refs := *ifErr.Referrers()
		if len(refs) == 1 {
			if store, ok := refs[0].(*ssa.Store); ok {
				ret.ifErr = store.Addr
			}
		}
	}
	return ret
}

func debug(f func()) {
	if !isDebug {
		return
	}
	f()
}
