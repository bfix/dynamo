package dynamo

//----------------------------------------------------------------------
// This file is part of Dynamo.
// Copyright (C) 2011-2020 Bernd Fix
//
// Dynamo is free software: you can redistribute it and/or modify it
// under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License,
// or (at your option) any later version.
//
// Dynamo is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
// Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
//
// SPDX-License-Identifier: AGPL3.0-or-later
//----------------------------------------------------------------------

import (
	"bytes"
	"testing"
)

// testData is a data structure for a test case
type testData struct {
	name   string   // name of test case
	src    []string // DYNAMO source code
	lineno int      // lines processed
	err    string   // parse error
	run    bool     // run model?
	status error    // runtime error
}

// testSet is a collection of all test cases
var testSet = []*testData{
	//------------------------------------------------------------------
	// valid source code
	&testData{
		name: "mdl: ok",
		src: []string{
			"R CHNGE.KL=CONST*(ROOM-COFFEE.K)",
			"L COFFEE.K=COFFEE.J+(DT)(CHNG.JK)",
			"C DIFF=ROOM-COFFEE",
			"C CONST=0.2",
			"C ROOM=20",
			"C COFFEE=90",
		},
		lineno: 6,
		err:    "",
	},
	//------------------------------------------------------------------
	// Dependency loop detection
	&testData{
		name: "mdl: dependency loop",
		src: []string{
			"L INV.K=INV.J+DT*CHNG.JK+TEST.K",
			"L TEST.K=CONST*INV.K",
			"R CHNG.KL=0",
			"C CONST=1",
		},
		lineno: 4,
		err:    ErrModelDependencyLoop,
	},
	//------------------------------------------------------------------
	// Missing mode
	&testData{
		name: "eqn: missing mode",
		src: []string{
			"INV.K=INV.J+DT*CHNG.JK",
			"R CHNG.KL=0",
		},
		lineno: 2,
		err:    ErrParseInvalidMode,
	},
	//------------------------------------------------------------------
	// Bad mode
	&testData{
		name: "eqn: bad mode",
		src: []string{
			"Y INV.K=INV.J+DT*CHNG.JK",
			"R CHNG.KL=0",
		},
		lineno: 2,
		err:    ErrParseInvalidMode,
	},
	//------------------------------------------------------------------
	// Syntax error in equation
	&testData{
		name: "eqn: syntax",
		src: []string{
			"L INV.K=INV.J+DT**CHNG.JK",
			"R CHNG.KL=0",
		},
		lineno: 2,
		err:    ErrParseSyntax,
	},
	//------------------------------------------------------------------
	// Invalid target state (not NEW)
	&testData{
		name: "eqn: bad target stage",
		src: []string{
			"L INV.J=INV.K+DT*CHNG.JK",
		},
		lineno: 1,
		err:    ErrModelEqnBadTargetStage,
	},
	//------------------------------------------------------------------
	// Duplicate equation
	&testData{
		name: "eqn: overwrite",
		src: []string{
			"L INV.K=INV.J+DT*CHNG.JK",
			"L INV.K=CONST*INV.J",
		},
		lineno: 2,
		err:    ErrModelEqnOverwrite,
	},
	//------------------------------------------------------------------
	// Line too long
	&testData{
		name: "parse: line too long",
		src: []string{
			"L INV.K=INV.J+DT*CHNG.JK-COFFEE.J+(DT)(CHNG.JK)+CONST*(ROOM-COFFEE.K)+OFFSET",
		},
		lineno: 1,
		err:    ErrParseLineLength,
	},
	//------------------------------------------------------------------
	// Invalid variable name (too long)
	&testData{
		name: "eqn: var name too long",
		src: []string{
			"L INVENTARLISTE.K=INVENTARLISTE.J+DT*CHNG.JK",
		},
		lineno: 1,
		err:    ErrParseNameLength,
	},
	//------------------------------------------------------------------
	// Invalid variable index
	&testData{
		name: "eqn: bad var index",
		src: []string{
			"L INV.L=INV.J+DT*CHNG.JK",
		},
		lineno: 1,
		err:    ErrParseInvalidIndex,
	},
	//------------------------------------------------------------------
	// Wrong target kind for equation
	&testData{
		name: "eqn: bad var kind",
		src: []string{
			"R INV.K=INV.J+DT*CHNG.JK",
		},
		lineno: 1,
		err:    ErrModelEqnBadTargetKind,
	},
}

func TestModel(t *testing.T) {
	failed := 0
	for _, td := range testSet {
		mdl := NewModel("", "")
		buf := new(bytes.Buffer)
		for _, line := range td.src {
			buf.WriteString(line + "\n")
		}
		res := mdl.Parse(bytes.NewReader(buf.Bytes()))
		if !res.Ok && !res.IsA(td.err) {
			t.Logf("[%s] Error mismtach: %s != %s\n", td.name, res.Err.Error(), td.err)
			failed++
		}
		if res.Line != td.lineno {
			t.Logf("[%s] Line mismtach: %d != %d\n", td.name, res.Line, td.lineno)
			failed++
		}
		if res.Ok && td.run {
			if res = mdl.Run(); !res.IsA(td.err) {
				t.Logf("[%s] Status mismtach: %s != %s\n", td.name, res.Err.Error(), td.err)
				failed++
			}
		}
	}
	if failed > 0 {
		t.Fatalf("%d test cases failed", failed)
	}
}
