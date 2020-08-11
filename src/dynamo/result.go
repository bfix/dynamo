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
	"strings"
)

// DYNAMO error messages
const (
	ErrModelDependencyLoop    = "Equations have cyclic dependencies"
	ErrModelEqnBadTargetStage = "Wrong stage for equation target"
	ErrModelEqnBadMode        = "Wrong mode for equation"
	ErrModelEqnOverwrite      = "Equation overwrite"
	ErrModelEqnBadTargetKind  = "Wrong kind for equation target"
	ErrModelUnknownEqn        = "No defining equation for variable found"
	ErrModelUnknownFunction   = "Unknown function call"
	ErrModelFunctionArg       = "Invalid function argument"
	ErrModelNoVariable        = "No variable found"
	ErrModelVariabeExists     = "Variable already known"
	ErrModelNoSuchTable       = "No such table"
	ErrModelWrongTableSize    = "Tabe size mismatch"
	ErrModelNoTime            = "No TIME defined"
	ErrModelMaxRetry          = "Retry limit reached"
	ErrModelMissingDef        = "Missing definition of value"
	ErrModelNoData            = "No data available"

	ErrParseLineLength      = "Line too long"
	ErrParseInvalidSpace    = "Space in equation"
	ErrParseInvalidMode     = "Line does not start with a valid mode"
	ErrParseInvalidName     = "Invalid variable name"
	ErrParseInvalidIndex    = "Invalid variable index"
	ErrParseNameLength      = "Variable name too long"
	ErrParseSyntax          = "Syntax error"
	ErrParseInvalidOp       = "Unknown operand"
	ErrParseTableTooSmall   = "Not enough table elements"
	ErrParseUnknownFunction = "Unknown function"
	ErrParseInvalidNumArgs  = "Invalid number of arguments"
	ErrParseMacroDepth      = "Invalid nesting for macro function"
	ErrParseNotANumber      = "Not a number"

	ErrPlotRange = "Range failure"
)

// Result represents the response of a method call in the Dynamo framework.
// It allows to track failures with more information than 'error' alone
// provides.
type Result struct {
	Ok   bool        // call returned without problems
	Err  error       // error (if !Ok)
	Line int         // line number in input stream (0 if not parsing)
	Ctx  interface{} // Optional failure context
}

// Success is used if the call finishes without problems
func Success() *Result {
	return &Result{
		Ok:   true,
		Err:  nil,
		Line: 0,
		Ctx:  nil,
	}
}

// Failure returns a result for a failed operation. The parameter can be
// of type 'string' or 'error'.
func Failure(err interface{}, args ...interface{}) *Result {
	var e error = nil
	switch x := err.(type) {
	case error:
		e = x
	case string:
		if len(args) > 0 {
			e = fmt.Errorf(x, args...)
		} else {
			e = fmt.Errorf(x)
		}
	}
	return &Result{
		Ok:   false,
		Err:  e,
		Line: 0,
		Ctx:  nil,
	}
}

// SetLine should be used by methods that are involved with parsing
// DYNAMO source code to report the problematic line in the input stream.
func (r *Result) SetLine(line int) *Result {
	r.Line = line
	return r
}

// IsA returns true if the given (failure) result is of given error
func (r *Result) IsA(err string) bool {
	if r.Err == nil {
		return false
	}
	return strings.HasPrefix(r.Err.Error(), err)
}
