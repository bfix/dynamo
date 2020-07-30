package dynamo

import (
	"fmt"
	"math"
	"testing"
)

func TestFcnTable(t *testing.T) {

	mdl := NewModel()
	pnts := []float64{0, 2.8, 5.5, 8, 9.5, 10}

	tbl := "TEST="
	for i, v := range pnts {
		if i > 0 {
			tbl += "/"
		}
		tbl += fmt.Sprintf("%f", v)
	}
	res := mdl.AddStatement(tbl, "T")
	if !res.Ok {
		t.Fatal(res.Err)
	}
	for x := 0; x <= 5; x += 1 {
		xs := fmt.Sprintf("%f", float64(x)/5)
		val, res := CallFunction("TABLE", []string{"TEST", xs, "0", "1", "0.2"}, mdl)
		if !res.Ok {
			t.Fatal(res.Err)
		}
		if math.Abs(float64(val)-pnts[x]) > 1e-9 {
			t.Fatalf("Value mismatch: %f != %f", val, pnts[x])
		}
	}
}
