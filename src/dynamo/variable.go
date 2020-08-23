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
	"reflect"
	"strings"
	"unicode"
)

//======================================================================
// A VARIABLE represents a named feature (or a constant) in a system
// model. The name is string beginning with a letter and an optional
// index. In 'strict' mode the name is limited to max. six characters.
//======================================================================

//----------------------------------------------------------------------
// NAME -- A variable name consists of two parts: a simple name part and
// an (optional) index, spearated by a dot ("."). The name must be
// uppercase and start with a letter; the length of a name is limited to
// MAX_NAME_LENGTH. The index part is limited to values as defined in
// INDEX_LIST. The 'stage' attribute classifies the temporality of the
// index (whether it refers to 'now' or 'past' states).
//
// Example names: COFFEE, SHPMTS.JK, INV.K
//----------------------------------------------------------------------

// Name-related constants
const (
	MAX_NAME_LENGTH = 6 // max. length of name in 'strict' mode

	// Kind of variable
	NAME_KIND_CONST = 0
	NAME_KIND_INIT  = 1
	NAME_KIND_LEVEL = 2
	NAME_KIND_RATE  = 3
	NAME_KIND_AUX   = 4
	NAME_KIND_SUPPL = 5

	// Stage of variable
	NAME_STAGE_NONE = 0 // only constants can have this stage
	NAME_STAGE_OLD  = 1
	NAME_STAGE_NEW  = 2

	// Results for Name.Compare:
	NAME_MISMATCH  = 0 // names don't match
	NAME_SAMEVAR   = 1 // variables have same name
	NAME_SAMEKIND  = 2 // variable are of same kind
	NAME_SAMESTAGE = 4 // variables have same stage
	NAME_MATCH     = 7 // names match fully
)

var (
	autoId = 0 // last automatic variable identifier
)

// NewAutoVar generates a new automatic variable name
func NewAutoVar() string {
	autoId++
	return fmt.Sprintf("_%d", autoId)
}

// Class is a classification for variables
type Class struct {
	Kind  int // NAME_KIND_?
	Stage int // NAME_STAGE_?
}

// Name of a state variable
type Name struct {
	Class
	Name string // Name of the variable
}

// NewName returns a name instance for a given identifier.
func NewName(v ast.Expr) (name *Name, res *Result) {
	res = Success()

	switch x := v.(type) {
	case *ast.Ident:
		name = new(Name)
		name.Kind = NAME_KIND_CONST
		name.Stage = NAME_STAGE_NONE
		name.Name = x.Name
		if strict {
			if len(name.Name) > MAX_NAME_LENGTH {
				res = Failure(ErrParseNameLength+": %d", len(name.Name))
			} else {
				start := []rune(name.Name)[0]
				if !unicode.IsLetter(start) && start != '_' {
					res = Failure(ErrParseInvalidName+": %s", name.Name)
				}
			}
		}
		return
	case *ast.SelectorExpr:
		if name, res = NewName(x.X); !res.Ok {
			return
		}
		res = name.setIndex(x.Sel.Name)
	default:
		res = Failure(ErrParseInvalidName+": %s", reflect.TypeOf(v))
	}
	return
}

// NewNameFromString returns a name instance for a given identifier.
func NewNameFromString(n string) (name *Name, res *Result) {
	res = Success()
	parts := strings.Split(n, ".")
	name = new(Name)
	name.Kind = NAME_KIND_CONST
	name.Stage = NAME_STAGE_NONE
	name.Name = parts[0]
	if strict && len(name.Name) > MAX_NAME_LENGTH {
		res = Failure(ErrParseNameLength+": %d", len(name.Name))
	}
	if len(parts) > 1 {
		res = name.setIndex(parts[1])
	}
	return
}

// SetIndex sets name flags for a given index string
func (n *Name) setIndex(idx string) (res *Result) {
	res = Success()
	switch idx {
	case "J":
		n.Kind = NAME_KIND_LEVEL
		n.Stage = NAME_STAGE_OLD
	case "JK":
		n.Kind = NAME_KIND_RATE
		n.Stage = NAME_STAGE_OLD
	case "K":
		n.Kind = NAME_KIND_LEVEL
		n.Stage = NAME_STAGE_NEW
	case "KL":
		n.Kind = NAME_KIND_RATE
		n.Stage = NAME_STAGE_NEW
	default:
		res = Failure(ErrParseInvalidIndex+": %s", idx)
	}
	return
}

// GetIndex returns the variable index
func (n *Name) GetIndex() string {
	if n.Stage == NAME_STAGE_OLD {
		if n.Kind == NAME_KIND_LEVEL {
			return "J"
		}
		if n.Kind == NAME_KIND_RATE {
			return "JK"
		}
	} else if n.Stage == NAME_STAGE_NEW {
		if n.Kind == NAME_KIND_LEVEL {
			return "K"
		}
		if n.Kind == NAME_KIND_RATE {
			return "KL"
		}
	}
	return ""
}

// String returns a name in human-readable format
func (n *Name) String() (name string) {
	name = n.Name
	switch n.Kind {
	case NAME_KIND_CONST:
		name += "/C"
	case NAME_KIND_INIT:
		name += "/I"
	case NAME_KIND_LEVEL:
		name += "/L"
	case NAME_KIND_RATE:
		name += "/R"
	}
	return
}

// Compare checks if two names (partially) match.
func (n *Name) Compare(m *Name) int {
	match := NAME_MISMATCH
	if n.Name == m.Name {
		match |= NAME_SAMEVAR
	}
	if n.Kind == m.Kind {
		match |= NAME_SAMEKIND
	}
	if n.Stage == m.Stage {
		match |= NAME_SAMESTAGE
	}
	return match
}

//----------------------------------------------------------------------
// VARIABLE -- represents variables in equations.
//----------------------------------------------------------------------

// Variable has a floating point value
type Variable float64

// String returns the human-readable representation of a variable
func (v Variable) String() string {
	return fmt.Sprintf("%f", v)
}

//----------------------------------------------------------------------
// TSVar -- Time-series variable
//----------------------------------------------------------------------

// TSVar is a named variable with a list of values (time series)
type TSVar struct {
	Name     string    // variable name
	Min, Max float64   // plot range
	Values   []float64 // time-series of values
}

// Add a TSVar value
func (ts *TSVar) Add(y float64) {
	if len(ts.Values) == 0 {
		ts.Min = y
		ts.Max = y
	} else if y < ts.Min {
		ts.Min = y
	} else if y > ts.Max {
		ts.Max = y
	}
	ts.Values = append(ts.Values, y)
}
