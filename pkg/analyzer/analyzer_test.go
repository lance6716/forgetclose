package analyzer

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()

	types := []CheckType{
		{"database/sql", "Rows"},
	}
	checker := NewAnalyzer(types)
	//analysistest.Run(t, testdata, checker, "wip")
	analysistest.Run(t, testdata, checker, "testcase")
}
