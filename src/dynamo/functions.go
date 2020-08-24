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
	"go/ast"
	"math"
	"math/rand"
	"strconv"
)

// Function represents a callable entity in the Dynamo framework.
// A function takes a list of arguments and returns a numerical result (as well
// as an status result). The list of arguments is build from the list of
// explicit arguments (as given in the equation statement) and instance
// arguments (as requested by the function). Instance arguments are stateful;
// they refer to automatic variables.
type Function struct {
	NumArgs int // number of expected (explicit) arguments
	NumVars int // number of requested internal variables

	Check func(args []ast.Expr) *Result                       // argument check function
	Eval  func(args []string, mdl *Model) (Variable, *Result) // evalutae function
}

var (
	// fcnList is a collection of available functions
	fcnList map[string]*Function
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
			NumVars: 0,
			Check:   nil,
			Eval: func(args []string, mdl *Model) (val Variable, res *Result) {
				if val, res = resolve(args[0], mdl); res.Ok {
					val = val.Sqrt()
				}
				return
			},
		},
		"SIN": &Function{
			NumArgs: 1,
			NumVars: 0,
			Check:   nil,
			Eval: func(args []string, mdl *Model) (val Variable, res *Result) {
				if val, res = resolve(args[0], mdl); res.Ok {
					val = val.Sin()
				}
				return
			},
		},
		"COS": &Function{
			NumArgs: 1,
			NumVars: 0,
			Check:   nil,
			Eval: func(args []string, mdl *Model) (val Variable, res *Result) {
				if val, res = resolve(args[0], mdl); res.Ok {
					val = val.Cos()
				}
				return
			},
		},
		"EXP": &Function{
			NumArgs: 1,
			NumVars: 0,
			Check:   nil,
			Eval: func(args []string, mdl *Model) (val Variable, res *Result) {
				if val, res = resolve(args[0], mdl); res.Ok {
					val = val.Exp()
				}
				return
			},
		},
		"LOG": &Function{
			NumArgs: 1,
			NumVars: 0,
			Check:   nil,
			Eval: func(args []string, mdl *Model) (val Variable, res *Result) {
				if val, res = resolve(args[0], mdl); res.Ok {
					val = val.Log()
				}
				return
			},
		},
		"MAX": &Function{
			NumArgs: 2,
			NumVars: 0,
			Check:   nil,
			Eval: func(args []string, mdl *Model) (val Variable, res *Result) {
				var a, b Variable
				if a, res = resolve(args[0], mdl); res.Ok {
					if b, res = resolve(args[1], mdl); res.Ok {
						if a.Compare(b) < 0 {
							val = b
						} else {
							val = a
						}
					}
				}
				return
			},
		},
		"MIN": &Function{
			NumArgs: 2,
			NumVars: 0,
			Check:   nil,
			Eval: func(args []string, mdl *Model) (val Variable, res *Result) {
				var a, b Variable
				if a, res = resolve(args[0], mdl); res.Ok {
					if b, res = resolve(args[1], mdl); res.Ok {
						if a.Compare(b) < 0 {
							val = a
						} else {
							val = b
						}
					}
				}
				return
			},
		},
		"CLIP": &Function{
			NumArgs: 4,
			NumVars: 0,
			Check:   nil,
			Eval: func(args []string, mdl *Model) (val Variable, res *Result) {
				var a, b, x, y Variable
				if a, res = resolve(args[0], mdl); res.Ok {
					if b, res = resolve(args[1], mdl); res.Ok {
						if x, res = resolve(args[2], mdl); res.Ok {
							if y, res = resolve(args[3], mdl); res.Ok {
								if x.Compare(y) < 0 {
									val = b
								} else {
									val = a
								}
							}
						}
					}
				}
				return
			},
		},
		"SWITCH": &Function{
			NumArgs: 3,
			NumVars: 0,
			Check:   nil,
			Eval: func(args []string, mdl *Model) (val Variable, res *Result) {
				var a, b, x Variable
				if a, res = resolve(args[0], mdl); res.Ok {
					if b, res = resolve(args[1], mdl); res.Ok {
						if x, res = resolve(args[2], mdl); res.Ok {
							if x.Compare(0) == 0 {
								val = a
							} else {
								val = b
							}
						}
					}
				}
				return
			},
		},
		//--------------------------------------------------------------
		// Generating functions
		//--------------------------------------------------------------
		"STEP": &Function{
			NumArgs: 2,
			NumVars: 0,
			Check:   nil,
			Eval: func(args []string, mdl *Model) (val Variable, res *Result) {
				var a, b Variable
				if a, res = resolve(args[0], mdl); res.Ok {
					if b, res = resolve(args[1], mdl); res.Ok {
						if time, ok := mdl.Current["TIME"]; ok {
							if time.Compare(b) >= 0 {
								val = a
							}
						} else {
							res = Failure(ErrModelNoTime)
						}
					}
				}
				return
			},
		},
		"RAMP": &Function{
			NumArgs: 2,
			NumVars: 0,
			Check:   nil,
			Eval: func(args []string, mdl *Model) (val Variable, res *Result) {
				if val, res = CallFunction("STEP", args, mdl); res.Ok {
					lvl := args[len(args)-1]
					dt := mdl.Current["DT"]
					v, _ := mdl.Last[lvl]
					val = v + dt*val
					mdl.Current[lvl] = val
				}
				return
			},
		},
		"PULSE": &Function{
			NumArgs: 3,
			NumVars: 0,
			Check:   nil,
			Eval: func(args []string, mdl *Model) (val Variable, res *Result) {
				var a, b, c Variable
				if a, res = resolve(args[0], mdl); res.Ok {
					if b, res = resolve(args[1], mdl); res.Ok {
						if c, res = resolve(args[2], mdl); res.Ok {
							if time, ok := mdl.Current["TIME"]; ok {
								x := (time - b) / c
								if x.Compare(x.Floor()) == 0 {
									val = a
								}
							}
						}
					}
				}
				return
			},
		},
		"NOISE": &Function{
			NumArgs: 0,
			NumVars: 0,
			Check:   nil,
			Eval: func(args []string, mdl *Model) (val Variable, res *Result) {
				val = Variable(rand.Float64() - 0.5)
				res = Success()
				return
			},
		},
		//--------------------------------------------------------------
		// TABLE functions
		//--------------------------------------------------------------
		"TABLE": &Function{
			NumArgs: 5,
			NumVars: 0,
			Check:   nil,
			Eval: func(args []string, mdl *Model) (val Variable, res *Result) {
				return table(args, mdl, 0)
			},
		},
		"TABHL": &Function{
			NumArgs: 5,
			NumVars: 1,
			Check:   nil,
			Eval: func(args []string, mdl *Model) (val Variable, res *Result) {
				return table(args, mdl, 0)
			},
		},
		"TABXT": &Function{
			NumArgs: 5,
			NumVars: 0,
			Check:   nil,
			Eval: func(args []string, mdl *Model) (val Variable, res *Result) {
				return table(args, mdl, 1)
			},
		},
		"TABPL": &Function{
			NumArgs: 5,
			NumVars: 0,
			Check:   nil,
			Eval: func(args []string, mdl *Model) (val Variable, res *Result) {
				return table(args, mdl, 2)
			},
		},
		//--------------------------------------------------------------
		// DELAY functions
		//--------------------------------------------------------------
		"DELAY1": &Function{
			NumArgs: 2,
			NumVars: 2,
			Check: func(args []ast.Expr) *Result {
				// the first variable must be of kind RATE from OLD state
				n, res := NewName(args[0])
				if !res.Ok {
					return res
				}
				if n.Kind != NAME_KIND_RATE {
					return Failure(ErrModelFunction+": DELAY1 --  %s not a rate", n.String())
				}
				if n.Stage != NAME_STAGE_OLD {
					return Failure(ErrModelFunction+": DELAY1 --  %s%s not old", n.Name, n.GetIndex())
				}
				return Success()
			},
			Eval: func(args []string, mdl *Model) (val Variable, res *Result) {
				var (
					name   *Name
					a, b   Variable
					v1, v2 Variable
					dt     Variable
				)
				if b, res = resolve(args[1], mdl); !res.Ok {
					return
				}
				if dt, res = resolve("DT", mdl); !res.Ok {
					return
				}
				// get value of first argument
				if a, res = resolve(args[0], mdl); !res.Ok {
					// if it is missing, we are initializing (no previos state):
					// get the current value of the variable
					if name, res = NewNameFromString(args[0]); !res.Ok {
						return
					}
					name.Stage = NAME_STAGE_NEW
					if a, res = mdl.Get(name); !res.Ok {
						return
					}
					// perform initialization
					mdl.Current[args[2]] = a * b
					mdl.Current[args[3]] = a
					return
				}
				// get old internal state
				if v1, res = resolve(args[2], mdl); !res.Ok {
					return
				}
				if v2, res = resolve(args[3], mdl); !res.Ok {
					return
				}
				// compute new internal state
				v1 += dt * (a - v2)
				v2 = v1 / b
				mdl.Current[args[2]] = v1
				mdl.Current[args[3]] = v2
				// return function result
				return v2, Success()
			},
		},
		"DELAY3": &Function{
			NumArgs: 2,
			NumVars: 0,
			Check:   nil,
			Eval:    nil,
		},
		"DELAYP": &Function{
			NumArgs: 2,
			NumVars: 0,
			Check:   nil,
			Eval:    nil,
		},
		//--------------------------------------------------------------
		// SMOOTH functions
		//--------------------------------------------------------------
		"SMOOTH": &Function{
			NumArgs: 2,
			NumVars: 0,
			Check:   nil,
			Eval:    nil,
		},
	}
}

// HasFunction checks if a named function is available for given number of
// arguments. It returns the list of automatic variable assigned to the
// function call instance.
func HasFunction(target *Name, name string, args []ast.Expr) ([]ast.Expr, *Result) {
	// check if we have a function of given name in our list
	if f, ok := fcnList[name]; ok {
		// check number of explicit arguments
		if len(args) != f.NumArgs {
			return nil, Failure(ErrParseInvalidNumArgs)
		}
		// if we have a list of internal variables, create them now
		intern := make([]ast.Expr, f.NumVars)
		for i := range intern {
			intern[i] = &ast.Ident{
				Name: NewAutoVar(),
			}
		}
		// use optional check function to validate arguments
		res := Success()
		if f.Check != nil {
			res = f.Check(args)
		}
		return intern, res
	}
	return nil, Failure(ErrParseUnknownFunction+": '%s'", name)
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
	val, res = f.Eval(args, mdl)
	return
}

//----------------------------------------------------------------------
// TABLE to model functions of form "Y = TABLE(X)" (TABHL, TABXT, TABPL)
//----------------------------------------------------------------------

// Table is a list of values
type Table struct {
	Data []float64
	A_j  []float64
}

// NewTable creates a new Table from a given list of (stringed) values.
func NewTable(list []string) (tbl *Table, res *Result) {
	res = Success()

	// check argument
	num := len(list)
	if num < 2 {
		res = Failure(ErrParseTableTooSmall)
		return
	}
	tbl = new(Table)
	tbl.Data = make([]float64, num)
	for i, v := range list {
		val, err := strconv.ParseFloat(v, 64)
		if err != nil {
			res = Failure(err)
			break
		}
		tbl.Data[i] = val
	}

	// precompute coefficients for Newton polynominal interpolation
	step := 1. / float64(num-1)
	var a_mj func(int, int) float64
	a_mj = func(m, j int) (y float64) {
		if m == j {
			y = tbl.Data[m]
		} else {
			y = (a_mj(m+1, j) - a_mj(m, j-1)) / (float64(j-m) * step)
		}
		return
	}
	tbl.A_j = make([]float64, num)
	for j := 0; j < num; j++ {
		tbl.A_j[j] = a_mj(0, j)
	}
	return
}

//======================================================================
// Implementation of Dynamo functions
//======================================================================

// resolve returns a value from a number string or variable name
func resolve(x string, mdl *Model) (Variable, *Result) {
	if val, err := strconv.ParseFloat(x, 64); err == nil {
		return Variable(val), Success()
	}
	name, res := NewNameFromString(x)
	if res.Ok {
		return mdl.Get(name)
	}
	return 0, res
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
	var x, min, max, step Variable
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
	n := Variable(len(tbl.Data) - 1)
	if (max - min).Compare(n*step) != 0 {
		res = Failure(ErrModelWrongTableSize)
		return
	}
	// get inter-/extrapolation parameters
	pos := float64(n * (x - min) / (max - min))
	idx := int(pos)
	frac := pos - float64(idx)
	Dbg.Msgf("TABLE: x=%f, pos=%f, idx=%d, frac=%f\n", x, pos, idx, frac)

	// check for "HL" argument
	if len(args) == 6 {
		hl, ok := mdl.Current[args[5]]
		if !ok {
			hl = 1.0
			mdl.Current[args[5]] = hl
		}
		// range check
		if hl.Compare(0) != 0 {
			outside := (compare(pos, 0) < 0) || (compare(pos, float64(n)) > 0)
			if outside && hl.Compare(0) > 0 {
				Msgf("WARN: Leaving table range '%s'...\n", args[0])
				mdl.Current[args[5]] = -1.0
			} else if !outside && hl.Compare(0) < 0 {
				Msgf("WARN: Entering table range '%s'...\n", args[0])
				mdl.Current[args[5]] = 1.0
			}
		}
	}

	// process table depending on mode
	if mode == 2 {
		// polynominal interpolation
		val = Variable(newton(pos, tbl.A_j))
	} else {
		// linear inter-/extrapolation
		if idx < 0 {
			if mode == 0 {
				val = Variable(tbl.Data[0])
			} else {
				val = Variable((tbl.Data[1]-tbl.Data[0])/float64(step)*pos + tbl.Data[0])
			}
		} else if idx > len(tbl.Data)-2 {
			last := len(tbl.Data) - 1
			if mode == 0 {
				val = Variable(tbl.Data[last])
			} else {
				val = Variable((tbl.Data[last]-tbl.Data[last-1])/float64(step)*(pos-1) + tbl.Data[last])
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
