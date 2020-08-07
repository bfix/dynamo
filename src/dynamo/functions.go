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
	"fmt"
	"go/ast"
	"log"
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
)

// Function represents a callable entity in the Dynamo framework. The call
// takes a list of (stringed) arguments and returns a numerical result (as
// well as an status result). The number of arguments is usually fixed per
// function type.
// Pseudo functions like DELAY? are also supported; pseudo functions are
// not evaluated, but act as "templates" to expand into a set of normal
// equations, e.g.
//     R   @1.KL=DELAY1(@2.JK,@3)
// is expanded into three equations:
//     L   $1.K=$1.J+DT*(@2.JK-@1.JK)
//     N   $1=@2*@3
//     R   @1.KL=$1.K/@3
// $1 is a generated (automatic) variable; @<n> is the n.th name as it appears
// in the original expression. Pseudo functions can be defined recursively.
//
type Function struct {
	NumArgs int
	Fcn     func(args []string, mdl *Model) (Variable, *Result)
	Pseudo  []*Line
}

var (
	// fcnList is a collection of available functions
	fcnList map[string]*Function
	tvar    *regexp.Regexp
)

// Initialize list of defined functions
func init() {
	// initialize list of functions.
	fcnList = map[string]*Function{
		//--------------------------------------------------------------
		// Mathematical functions
		//--------------------------------------------------------------
		"SQRT": &Function{
			NumArgs: 1,
			Fcn: func(args []string, mdl *Model) (val Variable, res *Result) {
				var x float64
				if x, res = resolve(args[0], mdl); res.Ok {
					val = Variable(math.Sqrt(x))
				}
				return
			},
			Pseudo: nil,
		},
		"SIN": &Function{
			NumArgs: 1,
			Fcn: func(args []string, mdl *Model) (val Variable, res *Result) {
				var x float64
				if x, res = resolve(args[0], mdl); res.Ok {
					val = Variable(math.Sin(x))
				}
				return
			},
			Pseudo: nil,
		},
		"COS": &Function{
			NumArgs: 1,
			Fcn: func(args []string, mdl *Model) (val Variable, res *Result) {
				var x float64
				if x, res = resolve(args[0], mdl); res.Ok {
					val = Variable(math.Cos(x))
				}
				return
			},
			Pseudo: nil,
		},
		"EXP": &Function{
			NumArgs: 1,
			Fcn: func(args []string, mdl *Model) (val Variable, res *Result) {
				var x float64
				if x, res = resolve(args[0], mdl); res.Ok {
					val = Variable(math.Exp(x))
				}
				return
			},
			Pseudo: nil,
		},
		"LOG": &Function{
			NumArgs: 1,
			Fcn: func(args []string, mdl *Model) (val Variable, res *Result) {
				var x float64
				if x, res = resolve(args[0], mdl); res.Ok {
					val = Variable(math.Log(x))
				}
				return
			},
			Pseudo: nil,
		},
		"MAX": &Function{
			NumArgs: 2,
			Fcn: func(args []string, mdl *Model) (val Variable, res *Result) {
				var a, b float64
				if a, res = resolve(args[0], mdl); res.Ok {
					if b, res = resolve(args[1], mdl); res.Ok {
						if compare(a, b) < 0 {
							val = Variable(b)
						} else {
							val = Variable(a)
						}
					}
				}
				return
			},
			Pseudo: nil,
		},
		"MIN": &Function{
			NumArgs: 2,
			Fcn: func(args []string, mdl *Model) (val Variable, res *Result) {
				var a, b float64
				if a, res = resolve(args[0], mdl); res.Ok {
					if b, res = resolve(args[1], mdl); res.Ok {
						if compare(a, b) < 0 {
							val = Variable(a)
						} else {
							val = Variable(b)
						}
					}
				}
				return
			},
			Pseudo: nil,
		},
		"CLIP": &Function{
			NumArgs: 4,
			Fcn: func(args []string, mdl *Model) (val Variable, res *Result) {
				var a, b, x, y float64
				if a, res = resolve(args[0], mdl); res.Ok {
					if b, res = resolve(args[1], mdl); res.Ok {
						if x, res = resolve(args[2], mdl); res.Ok {
							if y, res = resolve(args[3], mdl); res.Ok {
								if compare(x, y) < 0 {
									val = Variable(b)
								} else {
									val = Variable(a)
								}
							}
						}
					}
				}
				return
			},
			Pseudo: nil,
		},
		"SWITCH": &Function{
			NumArgs: 3,
			Fcn: func(args []string, mdl *Model) (val Variable, res *Result) {
				var a, b, x float64
				if a, res = resolve(args[0], mdl); res.Ok {
					if b, res = resolve(args[1], mdl); res.Ok {
						if x, res = resolve(args[2], mdl); res.Ok {
							if compare(x, 0) == 0 {
								val = Variable(a)
							} else {
								val = Variable(b)
							}
						}
					}
				}
				return
			},
			Pseudo: nil,
		},
		//--------------------------------------------------------------
		// Generating functions
		//--------------------------------------------------------------
		"STEP": &Function{
			NumArgs: 2,
			Fcn: func(args []string, mdl *Model) (val Variable, res *Result) {
				var a, b float64
				if a, res = resolve(args[0], mdl); res.Ok {
					if b, res = resolve(args[1], mdl); res.Ok {
						if time, ok := mdl.Current["TIME"]; ok {
							if compare(float64(time), b) >= 0 {
								val = Variable(a)
							}
						} else {
							res = Failure(ErrModelNoTime)
						}
					}
				}
				return
			},
			Pseudo: nil,
		},
		"RAMP": &Function{
			NumArgs: 2,
			Fcn: func(args []string, mdl *Model) (val Variable, res *Result) {
				var a, b float64
				if a, res = resolve(args[0], mdl); res.Ok {
					if b, res = resolve(args[1], mdl); res.Ok {
						if time, ok := mdl.Current["TIME"]; ok {
							t := float64(time)
							if compare(t, b) >= 0 {
								val = Variable(a * (t - b))
							} else {
								val = 0
							}
						} else {
							res = Failure(ErrModelNoTime)
						}
					}
				}
				return
			},
			Pseudo: nil,
		},
		"PULSE": &Function{
			NumArgs: 3,
			Fcn: func(args []string, mdl *Model) (val Variable, res *Result) {
				var a, b, c float64
				if a, res = resolve(args[0], mdl); res.Ok {
					if b, res = resolve(args[1], mdl); res.Ok {
						if c, res = resolve(args[2], mdl); res.Ok {
							if time, ok := mdl.Current["TIME"]; ok {
								t := float64(time)
								x := (t - b) / c
								if compare(x, math.Floor(x)) == 0 {
									val = Variable(a)
								}
							}
						}
					}
				}
				return
			},
			Pseudo: nil,
		},
		"NOISE": &Function{
			NumArgs: 0,
			Fcn: func(args []string, mdl *Model) (val Variable, res *Result) {
				val = Variable(rand.Float64() - 0.5)
				res = Success()
				return
			},
			Pseudo: nil,
		},
		//--------------------------------------------------------------
		// TABLE functions
		//--------------------------------------------------------------
		"TABLE": &Function{
			NumArgs: 5,
			Fcn: func(args []string, mdl *Model) (val Variable, res *Result) {
				return table(args, mdl, 0)
			},
			Pseudo: nil,
		},
		"TABXT": &Function{
			NumArgs: 5,
			Fcn: func(args []string, mdl *Model) (val Variable, res *Result) {
				return table(args, mdl, 1)
			},
			Pseudo: nil,
		},
		"TABPL": &Function{
			NumArgs: 5,
			Fcn: func(args []string, mdl *Model) (val Variable, res *Result) {
				return table(args, mdl, 2)
			},
			Pseudo: nil,
		},
		//--------------------------------------------------------------
		// DELAY functions
		//--------------------------------------------------------------
		"DELAY1": &Function{
			NumArgs: 2,
			Fcn:     nil,
			Pseudo: []*Line{
				{"L", "$1.K=$1.J+DT*(@2.JK-@1.JK)", ""},
				{"N", "$1=@2*@3", ""},
				{"R", "@1.KL=$1.K/@3", ""},
			},
		},
		"DELAY3": &Function{
			NumArgs: 2,
			Fcn:     nil,
			Pseudo: []*Line{
				{"L", "$1.K=$1.J+DT*(@2.JK-$4.JK)", ""},
				{"N", "$1=@2*@3/3", ""},
				{"R", "$4.KL=$1.K*3/@3", ""},
				{"L", "$2.K=$2.J+DT*($4.JK-$5.JK)", ""},
				{"N", "$2=$4*@3/3", ""},
				{"R", "$5.KL=$2.K*3/@3", ""},
				{"L", "$3.K=$3.J+DT*($5.JK-@1.JK)", ""},
				{"N", "$3=$5*@3/3", ""},
				{"R", "@1.KL=$3.K*3/@3", ""},
			},
		},
		"DELAYP": &Function{
			NumArgs: 2,
			Fcn:     nil,
			Pseudo: []*Line{
				{"L", "$1.K=$1.J+DT*(@2.JK-$4.JK)", ""},
				{"N", "$1=@2*@3/3", ""},
				{"R", "$4.KL=$1.K*3/@3", ""},
				{"L", "$2.K=$2.J+DT*($4.JK-$5.JK)", ""},
				{"N", "$2=$4*@3/3", ""},
				{"R", "$5.KL=$2.K*3/@3", ""},
				{"L", "$3.K=$3.J+DT*($5.JK-@1.JK)", ""},
				{"N", "$3=$5*@3/3", ""},
				{"R", "@1.KL=$3.K*3/@3", ""},
				{"L", "@4.K=$1+$2+$3", ""},
			},
		},
		//--------------------------------------------------------------
		// SMOOTH functions
		//--------------------------------------------------------------
		"SMOOTH": &Function{
			NumArgs: 2,
			Fcn:     nil,
			Pseudo: []*Line{
				{"L", "@1.K=@1.J+(DT/@3)*(@2.J-@1.J)", ""},
				{"N", "@1=@2", ""},
			},
		},
	}
	// compile regular expression for temp. variables
	var err error
	if tvar, err = regexp.Compile(`\$[0-9]+`); err != nil {
		log.Fatal(err)
	}
}

// HasFunction checks if a named function is available for given number
// of arguments.
func HasFunction(target *Name, name string, args []ast.Expr, depth int) (Pseudo []*Equation, res *Result) {
	if f, ok := fcnList[name]; ok {
		if len(args) != f.NumArgs {
			return nil, Failure(ErrParseInvalidNumArgs)
		}
		if f.Pseudo != nil {
			if depth != 0 {
				// Pseudo functions must resolve at depth 0
				res = Failure(ErrParsePseudoDepth+": %s", name)
				return
			}
			// expand pseudo function
			temps := make(map[string]string)
			temps["@1"] = target.Name
			for i, expr := range args {
				var n *Name
				if n, res = NewName(expr); !res.Ok {
					return
				}
				temps[fmt.Sprintf("@%d", i+2)] = n.Name
			}
			var eqns []*Equation
			for _, eqn := range f.Pseudo {
				line := eqn.Stmt
				matches := tvar.FindAllString(line, -1)
				for _, m := range matches {
					if _, ok := temps[m]; !ok {
						temps[m] = NewAutoVar()
					}
				}
				for vn, repl := range temps {
					line = strings.Replace(line, vn, repl, -1)
				}
				eqns, res = NewEquation(&Line{
					Mode: eqn.Mode,
					Stmt: line,
				})
				Pseudo = append(Pseudo, eqns...)
			}
		}
		res = Success()
		return
	}
	res = Failure(ErrParseUnknownFunction+": '%s'", name)
	return
}

// CallFunction executes a function call with given arguments
func CallFunction(name string, args []string, mdl *Model) (val Variable, res *Result) {
	res = Success()
	val = 0.0

	// lookup built-in function
	f, ok := fcnList[name]
	if !ok {
		res = Failure(ErrModelUnknownFunction+": %s\n", name)
		return
	}
	val, res = f.Fcn(args, mdl)
	return
}

//======================================================================
// Implementation of Dynamo functions
//======================================================================

// resolve returns a value from a number string or variable name
func resolve(x string, mdl *Model) (val float64, res *Result) {
	res = Success()
	val, err := strconv.ParseFloat(x, 64)
	if err != nil {
		var (
			name *Name
			v    Variable
		)
		if name, res = NewNameFromString(x); res.Ok {
			v, res = mdl.Get(name)
			val = float64(v)
		}
	}
	return
}

// compare a variable to a value
func compare(v float64, x float64) int {
	if math.Abs(v-x) < 1e-9 {
		return 0
	}
	if v > x {
		return 1
	}
	return -1
}

//----------------------------------------------------------------------
// TABLEs
//----------------------------------------------------------------------

// generic table handling
func table(args []string, mdl *Model, mode int) (val Variable, res *Result) {
	Dbg.Msgf("Function TABLE(%d) called with %v\n", mode, args)

	// lookup table from name
	tbl, ok := mdl.Tables[args[0]]
	if !ok {
		res = Failure(ErrModelNoSuchTable+": %s", args[0])
		return
	}
	// get table parameters
	var x, min, max, step float64
	if x, res = resolve(args[1], mdl); !res.Ok {
		return
	}
	if min, res = resolve(args[2], mdl); !res.Ok {
		return
	}
	if max, res = resolve(args[3], mdl); !res.Ok {
		return
	}
	if step, res = resolve(args[4], mdl); !res.Ok {
		return
	}
	// check if parameters match table data
	n := len(tbl.Data)
	if n != int((max-min)/step)+1 {
		res = Failure(ErrModelWrongTableSize)
	}
	// get inter-/extrapolation parameters
	pos := (x - min) / (max - min)
	idx := int(pos / step)
	frac := pos/step - float64(idx)
	Dbg.Msgf("TABLE: x=%f, pos=%f, idx=%d, frac=%f\n", x, pos, idx, frac)

	if mode == 2 {
		// polynominal interpolation
		val = Variable(newton(pos, tbl.A_j))
	} else {
		// linear inter-/extrapolation
		if idx < 0 {
			if mode == 0 {
				val = Variable(tbl.Data[0])
			} else {
				val = Variable((tbl.Data[1]-tbl.Data[0])/step*pos + tbl.Data[0])
			}
		} else if idx > len(tbl.Data)-2 {
			last := len(tbl.Data) - 1
			if mode == 0 {
				val = Variable(tbl.Data[last])
			} else {
				val = Variable((tbl.Data[last]-tbl.Data[last-1])/step*(pos-1) + tbl.Data[last])
			}
		} else {
			val = Variable(tbl.Data[idx] + (tbl.Data[idx+1]-tbl.Data[idx])*frac)
		}
	}
	res = Success()
	return
}

// Newton polynominal interpolation that relies on 'divided differences'.
// 'x' is normalized [0,1]; points are equidistant with given step size.
func newton(x float64, a_j []float64) float64 {
	num := len(a_j)
	step := 1.0 / float64(num-1)
	n_j := func(x float64, j int) float64 {
		y := 1.0
		for i := 0; i < j; i++ {
			y *= (x - float64(i)*step)
		}
		return y
	}
	// polynominal interpolation
	y := 0.0
	for j := 0; j < num; j++ {
		y += a_j[j] * n_j(x, j)
	}
	return y
}
