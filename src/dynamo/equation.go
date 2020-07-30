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

//----------------------------------------------------------------------
// EQUATION -- An equation is a formula that describes the (new) value
// of a variable with given name as a computation between old variables
// (and probably constants). An equation is only computable in the
// context of a model state.
//----------------------------------------------------------------------

// Equation represents a formula; the result is assigned to a variable
type Equation struct {
	Target       *Name    // Name of (indexed) variable (left side of equation)
	Dependencies []*Name  // List of (indexed) variables from right side.
	Mode         string   // Mode of equation as given in the source
	Formula      ast.Expr // formula in Go AST
	stmt         string   // complete equation in DYNAMO notation
}

// NewEquation converts a statement into an equation instance
func NewEquation(stmt, mode string) (eqn *Equation, res *Result) {
	res = Success()

	// check for spaces in equation
	if strings.Index(stmt, " ") != -1 {
		res = Failure(ErrParseInvalidSpace)
		return
	}
	// prepare equation instance
	eqn = &Equation{
		stmt:         stmt,
		Mode:         mode,
		Dependencies: make([]*Name, 0),
	}
	// expand multiplication shortcut
	stmt = strings.ReplaceAll(stmt, ")(", ")*(")
	// assignment work-around (HACK!)
	stmt = strings.ReplaceAll(stmt, "=", "==")
	// use Go to parse expression
	expr, err := parser.ParseExpr(stmt)
	if err != nil {
		res = Failure(err)
		return
	}
	switch x := expr.(type) {
	case *ast.BinaryExpr:
		// Handle LEFT side of equation
		if eqn.Target, res = NewName(x.X); !res.Ok {
			return
		}
		if mode == "N" {
			eqn.Target.Kind = NAME_KIND_LEVEL
			eqn.Target.Stage = NAME_STAGE_NONE
		}

		// Handle RIGHT side of equation
		eqn.Dependencies, res = checkFormula(x.Y, mode)
		eqn.Formula = x.Y

	default:
		res = Failure(ErrParseSyntax+": %v\n", reflect.TypeOf(x))
	}
	return
}

// String returns a human-readable equation formula.
func (eqn *Equation) String() string {
	return eqn.Mode + " " + eqn.stmt
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

// Check a formula
func checkFormula(f ast.Expr, mode string) (deps []*Name, res *Result) {
	res = Success()
	deps = make([]*Name, 0)

	var check func(ast.Expr) *Result
	check = func(f ast.Expr) (res *Result) {
		res = Success()
		switch x := f.(type) {
		case *ast.Ident, *ast.SelectorExpr:
			var name *Name
			if name, res = NewName(x); res.Ok {
				if mode == "N" {
					name.Stage = NAME_STAGE_NONE
				}
				deps = append(deps, name)
			}
		case *ast.BinaryExpr:
			if res = check(x.X); res.Ok {
				res = check(x.Y)
			}
		case *ast.ParenExpr:
			res = check(x.X)
		case *ast.BasicLit:
			// skipped intentionally
		case *ast.UnaryExpr:
			res = check(x.X)
		case *ast.CallExpr:
			var name *Name
			if name, res = NewName(x.Fun); !res.Ok {
				break
			}
			if res = HasFunction(name.Name, len(x.Args)); !res.Ok {
				break
			}
			for _, arg := range x.Args {
				if res = check(arg); !res.Ok {
					break
				}
			}
		default:
			res = Failure(ErrParseSyntax+": %v\n", reflect.TypeOf(x))
		}
		return
	}
	res = check(f)
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
			case *ast.BasicLit:
				args[i] = x.Value
			case *ast.BinaryExpr:
				if val, res = eval(x, mdl); !res.Ok {
					break
				}
				args[i] = val.String()
			default:
				res = Failure(ErrModelFunctionArg+": %s", reflect.TypeOf(x))
				break
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
