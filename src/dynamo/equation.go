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
	"go/parser"
	"go/token"
	"reflect"
	"strconv"
	"strings"
)

// Dependency handling modes
const (
	DEP_NORMAL  = iota // normal dependencies
	DEP_ENFORCE        // enforce dependencies
	DEP_SKIP           // skip dependencies
)

//----------------------------------------------------------------------
// EQUATION -- An equation is a formula that describes the (new) value
// of a variable with given name as a computation between old variables
// (and probably constants). An equation is only computable in the
// context of a model state.
//----------------------------------------------------------------------

// Equation represents a formula; the result is assigned to a variable
type Equation struct {
	Target       *Name    // Name of (indexed) variable (left side of equation)
	Dependencies []*Name  // List of (indexed) dependencies from right side.
	References   []*Name  // List of references on the right side (non-dependent)
	Mode         string   // Mode of equation as given in the source
	Formula      ast.Expr // formula in Go AST
	stmt         string   // complete equation in DYNAMO notation
}

// NewEquation converts a statement into one or more equation instances
func NewEquation(stmt *Line) (eqns *EqnList, res *Result) {
	res = Success()
	eqns = NewEqnList()
	Dbg.Msgf("NewEquation(%s)\n", stmt.String())

	// check for spaces in equation
	if strings.Index(stmt.Stmt, " ") != -1 {
		res = Failure(ErrParseInvalidSpace)
		return
	}
	// Const statements can have multiple assignments in one line.
	if stmt.Mode == "C" && strings.Count(stmt.Stmt, "=") > 1 {
		// add new extracted equation
		addEqn := func(line string) (res *Result) {
			var list *EqnList
			if list, res = NewEquation(&Line{
				Stmt: line,
				Mode: "C",
			}); res.Ok {
				eqns.AddList(list)
			}
			return
		}
		// parse from end of statement
		line := stmt.Stmt
		for {
			pos := strings.LastIndex(line, "=")
			delim := strings.LastIndex(line[:pos], ",")
			if delim == -1 {
				if delim = strings.LastIndex(line[:pos], "/"); delim == -1 {
					res = addEqn(line)
					break
				}
			}
			Dbg.Msgf("Delim: %d\n", delim)
			if res = addEqn(line[delim+1:]); !res.Ok {
				break
			}
			line = line[:delim]
		}
		return
	}
	// expand multiplication shortcut
	line := strings.ReplaceAll(stmt.Stmt, ")(", ")*(")
	// assignment work-around (HACK!)
	line = strings.ReplaceAll(line, "=", "==")
	// use Go to parse expression
	expr, err := parser.ParseExpr(line)
	if err != nil {
		res = Failure(err)
		return
	}
	switch x := expr.(type) {
	case *ast.BinaryExpr:
		// prepare equation instance
		eqn := &Equation{
			stmt:         stmt.Stmt,
			Mode:         stmt.Mode,
			Dependencies: make([]*Name, 0),
			References:   make([]*Name, 0),
		}
		eqn.Formula = x.Y

		// Handle LEFT side of equation
		if eqn.Target, res = NewName(x.X); !res.Ok {
			return
		}
		switch stmt.Mode {
		case "N":
			if eqn.Target.Kind != NAME_KIND_CONST {
				res = Failure(ErrModelEqnBadTargetKind)
				return
			}
			eqn.Target.Kind = NAME_KIND_INIT
		case "A":
			if eqn.Target.Kind != NAME_KIND_LEVEL && eqn.Target.Kind != NAME_KIND_RATE {
				res = Failure(ErrModelEqnBadTargetKind)
				return
			}
			eqn.Target.Kind = NAME_KIND_AUX
			if eqn.Target.Stage != NAME_STAGE_NEW {
				res = Failure(ErrModelEqnBadTargetStage)
				return
			}
		case "S":
			if eqn.Target.Kind != NAME_KIND_LEVEL {
				res = Failure(ErrModelEqnBadTargetKind)
				return
			}
			eqn.Target.Kind = NAME_KIND_SUPPL
			if eqn.Target.Stage != NAME_STAGE_NEW {
				res = Failure(ErrModelEqnBadTargetStage)
				return
			}
		}

		// Handle RIGHT side of equation recursively
		var check func(ast.Expr, int) *Result
		check = func(f ast.Expr, mode int) (res *Result) {
			res = Success()
			switch x := f.(type) {
			case *ast.Ident, *ast.SelectorExpr:
				var name *Name
				if name, res = NewName(x); res.Ok {
					if stmt.Mode == "N" {
						name.Stage = NAME_STAGE_NONE
					}
					// add variable as dependency or reference
					if (mode == DEP_NORMAL && name.Stage != NAME_STAGE_OLD) || mode == DEP_ENFORCE {
						eqn.Dependencies = append(eqn.Dependencies, name)
					} else {
						eqn.References = append(eqn.References, name)
					}
				}
			case *ast.BinaryExpr:
				if res = check(x.X, mode); res.Ok {
					res = check(x.Y, mode)
				}
			case *ast.ParenExpr:
				res = check(x.X, mode)
			case *ast.BasicLit:
				// skipped intentionally
			case *ast.UnaryExpr:
				res = check(x.X, mode)
			case *ast.CallExpr:
				// get function name
				var name *Name
				if name, res = NewName(x.Fun); !res.Ok {
					break
				}
				// check for function availibility
				Dbg.Msgf("Calling '%s'\n", name.Name)
				var intern []ast.Expr
				if mode, intern, res = HasFunction(name.Name, x.Args); !res.Ok {
					break
				}
				// check function arguments
				for _, arg := range x.Args {
					if res = check(arg, mode); !res.Ok {
						break
					}
				}
				// add internal variable
				x.Args = append(x.Args, intern...)

			default:
				res = Failure(ErrParseSyntax+": %v\n", reflect.TypeOf(x))
			}
			return
		}

		res = check(x.Y, DEP_NORMAL)
		if res.Ok {
			eqns.Add(eqn)
		}
		return

	default:
		res = Failure(ErrParseSyntax+": %v\n", reflect.TypeOf(x))
	}
	return
}

// String returns a human-readable equation formula.
func (eqn *Equation) String() string {
	return "'" + eqn.Mode + ":" + eqn.stmt + "'"
}

// DependsOn returns true if a variable is referenced in the formula.
func (eqn *Equation) DependsOn(v *Name) bool {
	for _, d := range eqn.Dependencies {
		if d.Compare(v)&NAME_SAMEVAR != 0 {
			return true
		}
	}
	return false
}

// Eval an equation and get the resulting numerical value and a status
// result. The computation is performed on the state variables (level, rate)
// of a DYNAMO model.
func (eqn *Equation) Eval(mdl *Model) (res *Result) {
	var val Variable
	Dbg.Msgf("Evaluating: %s\n", eqn.String())
	if val, res = eval(eqn.Formula, mdl); res.Ok {
		res = mdl.Set(eqn.Target, val)
	}
	return
}

// recursively evaluate the equation for a given model state
func eval(expr ast.Expr, mdl *Model) (val Variable, res *Result) {
	res = Success()

	switch x := expr.(type) {
	case *ast.BinaryExpr:
		var left, right Variable
		if left, res = eval(x.X, mdl); !res.Ok {
			break
		}
		if right, res = eval(x.Y, mdl); !res.Ok {
			break
		}
		switch x.Op {
		case token.ADD:
			val = left + right
		case token.SUB:
			val = left - right
		case token.MUL:
			val = left * right
		case token.QUO:
			val = left / right
		default:
			res = Failure(ErrParseInvalidOp+": %d", x.Op)
		}
		return

	case *ast.ParenExpr:
		val, res = eval(x.X, mdl)

	case *ast.BasicLit:
		v, err := strconv.ParseFloat(x.Value, 64)
		if err != nil {
			res = Failure(err)
		}
		val = Variable(v)

	case *ast.Ident, *ast.SelectorExpr:
		var name *Name
		if name, res = NewName(x); !res.Ok {
			break
		}
		val, res = mdl.Get(name)

	case *ast.CallExpr:
		// get name of function
		var name *Name
		if name, res = NewName(x.Fun); !res.Ok {
			break
		}
		// convert arguments to strings
		args := make([]string, len(x.Args))
		for i, arg := range x.Args {
			switch x := arg.(type) {
			case *ast.Ident:
				args[i] = x.Name
			case *ast.SelectorExpr:
				var n *Name
				if n, res = NewName(x); !res.Ok {
					return
				}
				name := n.Name
				if idx := n.GetIndex(); len(idx) > 0 {
					name += idx
				}
				args[i] = name
			case *ast.BasicLit:
				args[i] = x.Value
			case *ast.BinaryExpr:
				if val, res = eval(x, mdl); !res.Ok {
					return
				}
				args[i] = val.String()
			case *ast.ParenExpr:
				if val, res = eval(x, mdl); !res.Ok {
					return
				}
				args[i] = val.String()
			case *ast.UnaryExpr:
				if val, res = eval(x.X, mdl); !res.Ok {
					break
				}
				switch x.Op {
				case token.SUB:
					val = -val
				default:
					res = Failure(ErrParseInvalidOp+": %d", x.Op)
				}
			default:
				res = Failure(ErrModelFunctionArg+": %s", reflect.TypeOf(x))
				return
			}
		}
		val, res = CallFunction(name.Name, args, mdl)

	case *ast.UnaryExpr:
		if val, res = eval(x.X, mdl); !res.Ok {
			break
		}
		switch x.Op {
		case token.SUB:
			val = -val
		default:
			res = Failure(ErrParseInvalidOp+": %d", x.Op)
		}

	default:
		res = Failure(ErrParseSyntax+": %v\n", reflect.TypeOf(x))
	}
	return
}
