package dynamo

import (
	"fmt"
	"testing"
)

func TestFcnTableList(t *testing.T) {

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
	t.Logf("X;Y;TABLE")
	for x := -20; x <= 70; x++ {
		xx := float64(x) / 50
		xs := fmt.Sprintf("%f", xx)
		val, res := CallFunction("TABLE", []string{"TEST", xs, "0", "1", "0.2"}, mdl)
		if !res.Ok {
			t.Fatal(res.Err)
		}
		t.Logf("%f;%f\n", xx, val)
	}
	t.Logf("X;Y;TABHL")
	for x := -20; x <= 70; x++ {
		xx := float64(x) / 50
		xs := fmt.Sprintf("%f", xx)
		val, res := CallFunction("TABHL", []string{"TEST", xs, "0", "1", "0.2"}, mdl)
		if !res.Ok {
			t.Fatal(res.Err)
		}
		t.Logf("%f;%f\n", xx, val)
	}
	t.Logf("X;Y;TABXT")
	for x := -20; x <= 70; x++ {
		xx := float64(x) / 50
		xs := fmt.Sprintf("%f", xx)
		val, res := CallFunction("TABXT", []string{"TEST", xs, "0", "1", "0.2"}, mdl)
		if !res.Ok {
			t.Fatal(res.Err)
		}
		t.Logf("%f;%f\n", xx, val)
	}
	t.Logf("X;Y;TABPL")
	for x := -20; x <= 70; x++ {
		xx := float64(x) / 50
		xs := fmt.Sprintf("%f", xx)
		val, res := CallFunction("TABPL", []string{"TEST", xs, "0", "1", "0.2"}, mdl)
		if !res.Ok {
			t.Fatal(res.Err)
		}
		t.Logf("%f;%f\n", xx, val)
	}
}

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
		xx := float64(x) / 5
		xs := fmt.Sprintf("%f", xx)
		val, res := CallFunction("TABLE", []string{"TEST", xs, "0", "1", "0.2"}, mdl)
		if !res.Ok {
			t.Fatal(res.Err)
		}
		if compare(float64(val), pnts[x]) != 0 {
			t.Fatalf("Value mismatch: %f != %f", val, pnts[x])
		}
	}
}

func TestFcnTabpl(t *testing.T) {
	pnts := []string{"0", "2.8", "5.5", "8", "9.5", "10"}
	tbl, res := NewTable(pnts)
	if !res.Ok {
		t.Fatal(res.Err)
	}
	for x := 0; x <= 5; x++ {
		y := tbl.Newton(Variable(x) / 5.)
		if y.Compare(Variable(tbl.Data[x])) != 0 {
			t.Fatalf("Value mismatch: %f != %f", y, tbl.Data[x])
		}
	}
}
