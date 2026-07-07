package main

import (
	"github.com/spachava753/dyson/internal/lint"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(lint.Analyzer)
}
