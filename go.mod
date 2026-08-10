module github.com/spachava753/dyson

go 1.26.4

require (
	github.com/nalgeon/be v0.3.0
	github.com/spf13/afero v1.15.0
	go.starlark.net v0.0.0-20260613233743-8ba36ccb83fb
	golang.org/x/net v0.56.0
	golang.org/x/sys v0.46.0
)

require golang.org/x/text v0.38.0 // indirect

replace go.starlark.net => github.com/spachava753/starlarkx v0.0.0-20260810013711-243e6013254e
