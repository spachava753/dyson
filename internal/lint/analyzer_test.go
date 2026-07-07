package lint_test

import (
	"testing"

	"github.com/spachava753/dyson/internal/lint"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	// analysistest.TestData finds this package's testdata directory, then Run
	// analyzes only the requested package pattern, testdata/src/a. The
	// go.starlark.net packages under testdata/src are dependency stubs used only
	// so package a can type-check its imports; they are not analyzer targets.
	// Expected diagnostics in package a are matched against // want comments.
	analysistest.Run(t, analysistest.TestData(), lint.Analyzer, "a")
}
