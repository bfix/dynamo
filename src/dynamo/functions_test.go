package dynamo

import (
	"fmt"
	"math"
	"testing"
)

func TestFcnTable(t *testing.T) {

	mdl := NewModel("", "")
	pnts := []float64{0, 2.8, 5.5, 8, 9.5, 10}

	tbl := "TEST="
	for i, v := range pnts {
		if i > 0 {
			tbl += "/"
		}
		tbl += fmt.Sprintf("%f", v)
	}
	stmt := &Line{
		Mode: "T",
		Stmt: tbl,
	}
	res := mdl.AddStatement(stmt)
	if !res.Ok {
		t.Fatal(res.Err)
	}
	for x := 0; x <= 5; x++ {
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

func TestFcnTabpl(t *testing.T) {
	pnts := []float64{0, 2.8, 5.5, 8, 9.5, 10}
	for x := 0; x <= 5; x++ {
		y := newton(float64(x)/5, 0.2, pnts)
		if math.Abs(y-pnts[x]) > 1e-9 {
			t.Fatalf("Value mismatch: %f != %f", y, pnts[x])
		}
	}
}
