package analyzer

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()

	checker := NewAnalyzer()
	analysistest.Run(t, testdata, checker, "wip")
	//analysistest.Run(t, testdata, checker, "testcase")
}
