package dyson_test

import (
	"testing"
	"testing/fstest"

	"github.com/nalgeon/be"
	"github.com/spachava753/dyson"
)

func TestJSONModuleLoadsConfiguredFile(t *testing.T) {
	fsys := dyson.FromIOFS(fstest.MapFS{
		"document.json": {Data: []byte(`{"name":"dyson","values":[1,true,null]}`)},
	})
	sphere := dyson.NewSphere(nil, dyson.NewStdlib(dyson.StdlibConfig{FS: fsys}))
	t.Cleanup(func() {
		be.Err(t, sphere.Close(), nil)
	})

	be.Err(t, sphere.Eval(t.Context(), `
load("json.star", "json")
file = open("document.json")
document = getattr(json, "load")(file)
file.close()
if document != {"name": "dyson", "values": [1, True, None]}:
    fail("unexpected decoded document")
if json.dumps(document) != '{"name":"dyson","values":[1,true,null]}':
    fail("unexpected encoded document")
`), nil)
}
