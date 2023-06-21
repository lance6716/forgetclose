package analyzer

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestSQLRows(t *testing.T) {
	testdata := analysistest.TestData()

	types := []CheckType{
		{"database/sql", "Rows"},
	}
	checker := NewAnalyzer(types)
	//analysistest.Run(t, testdata, checker, "wip")
	analysistest.Run(t, testdata, checker, "testcase")
}

func TestCustomInterface(t *testing.T) {
	testdata := analysistest.TestData()

	types := []CheckType{
		{"importee", "Closer"},
	}
	checker := NewAnalyzer(types)
	analysistest.Run(t, testdata, checker, "interfacetest", "importee")
}
