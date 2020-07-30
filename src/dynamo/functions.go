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
	"strconv"
)

// Function represents a callable entity in the Dynamo framework. The call
// takes a list of (stringed) arguments and returns a numerical result (as
// well as an status result). The number of arguments is usually fixed per
// function type.
type Function struct {
	MinArgs, MaxArgs int
	Fcn              func(args []string, mdl *Model) (Variable, *Result)
}

var (
	// fcnList is a collection of available functions
	fcnList = make(map[string]*Function)
)

// Initialize list of defined functions
func init() {
	fcnList["TABLE"] = &Function{
		MinArgs: 5,
		MaxArgs: 5,
		Fcn:     fcnTable,
	}
}

// HasFunction checks if a named function is available for given number
// of arguments.
func HasFunction(name string, args int) *Result {
	if f, ok := fcnList[name]; ok {
		if f.MinArgs <= args && args <= f.MaxArgs {
			return Success()
		}
		return Failure(ErrParseInvalidNumArgs)
	}
	return Failure(ErrParseUnknownFunction+": '%s'", name)
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

func fcnTable(args []string, mdl *Model) (val Variable, res *Result) {
	Dbg.Msgf("Function TABLE called with %v\n", args)
	tbl, ok := mdl.Tables[args[0]]
	if !ok {
		res = Failure(ErrModelNoSuchTable+": %s", args[0])
		return
	}
	x, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		res = Failure(err)
		return
	}
	min, err := strconv.ParseFloat(args[2], 64)
	if err != nil {
		res = Failure(err)
		return
	}
	max, err := strconv.ParseFloat(args[3], 64)
	if err != nil {
		res = Failure(err)
		return
	}
	step, err := strconv.ParseFloat(args[4], 64)
	if err != nil {
		res = Failure(err)
		return
	}
	pos := (x - min) / (max - min)
	idx := int(pos / step)
	frac := pos/step - float64(idx)
	Dbg.Msgf("TABLE: x=%f, pos=%f, idx=%d, frac=%f\n", x, pos, idx, frac)
	if idx < 0 {
		val = Variable(tbl.Data[0])
	} else if idx > len(tbl.Data)-2 {
		val = Variable(tbl.Data[len(tbl.Data)-1])
	} else {
		val = Variable(tbl.Data[idx] + (tbl.Data[idx+1]-tbl.Data[idx])*frac)
	}
	res = Success()
	return
}
