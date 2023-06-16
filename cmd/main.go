package main

import (
	"flag"
	"strings"

	"forgetclose/pkg/analyzer"
	"golang.org/x/tools/go/analysis/singlechecker"
)

var checkTypes = flag.String("type", "", `dot separated list of types to check, like "database/sql.Rows"`)

func main() {
	flag.Parse()
	if checkTypes == nil || *checkTypes == "" {
		println("no type specified")
		return
	}
	fields := strings.Split(*checkTypes, ".")
	types := []analyzer.CheckType{{
		PkgPath: fields[0],
		Name:    fields[1],
	}}
	singlechecker.Main(analyzer.NewAnalyzer(types))
}
