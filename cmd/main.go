package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/lance6716/forgetclose/pkg/analyzer"
	"golang.org/x/tools/go/analysis/singlechecker"
)

var checkTypes = flag.String("type", "", `dot separated list of types to check, like "database/sql.Rows"`)

func main() {
	flag.Parse()
	if checkTypes == nil || *checkTypes == "" {
		fmt.Println("no type specified")
		return
	}
	dotIdx := strings.LastIndex(*checkTypes, ".")
	types := []analyzer.CheckType{{
		PkgPath:  (*checkTypes)[:dotIdx],
		TypeName: (*checkTypes)[dotIdx+1:],
	}}
	singlechecker.Main(analyzer.NewAnalyzer(types))
}
