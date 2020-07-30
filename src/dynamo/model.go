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
	"strings"
)

const (
	strict = true
)

//======================================================================
// DYNAMO
//
// A DYNAMO program describes a dynamic model: a state (collection of
// variables with value) that is governed by a set of equations and
// start conditions / constants.
//======================================================================

//----------------------------------------------------------------------
// STATE -- A state represents the model state as a collection of named
// variables. The state transition is goverened by a set of equations
// and starting values / constants.
//----------------------------------------------------------------------

// State is a collection of named variables
type State map[string]Variable

//----------------------------------------------------------------------
// TABLE to model functions of form "Y = TABLE(X)"
//----------------------------------------------------------------------

// Table is a list of values
type Table struct {
	Data []float64
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
	return
}

//----------------------------------------------------------------------
// MODEL as defined in a DYNAMO source
//----------------------------------------------------------------------

// Model represents a DYNAMO model that can be executed
type Model struct {
	Title   string            // title of the model as defined by mode "*"
	Eqns    []*Equation       // list of equations
	Tables  map[string]*Table // list of tables
	Consts  State             // model constants and system settings
	Last    State             // previous state (J)
	Current State             // current state (K)
	Print   *Printer          // printer instance
	Plot    *Plotter          // plotter instance
}

// NewModel returns a new (empty) model instance.
func NewModel(printer, plotter string) *Model {
	mdl := &Model{
		Eqns:    make([]*Equation, 0),
		Tables:  make(map[string]*Table),
		Consts:  make(State),
		Last:    make(State),
		Current: make(State),
	}
	mdl.Print = NewPrinter(printer, mdl)
	mdl.Plot = NewPlotter(plotter, mdl)
	return mdl
}

// Quit is called when done with a model.
func (mdl *Model) Quit() {
	Dbg.Close()
	mdl.Print.Close()
	mdl.Plot.Close()
}

// Dump logs the current model state in human-readable form into
// the log stream.
func (mdl *Model) Dump() {

	// count equations by type
	cnt := make(map[string]int)
	incr := func(mode string) {
		v, ok := cnt[mode]
		if !ok {
			v = 0
		}
		cnt[mode] = v + 1
	}
	for _, e := range mdl.Eqns {
		incr(e.Mode)
	}
	Msgf(">     Number of equations: %4d\n", len(mdl.Eqns))
	Msgf(">         LEVEL equations: %4d\n", cnt["L"])
	Msgf(">          RATE equations: %4d\n", cnt["R"])
	Msgf(">           AUX equations: %4d\n", cnt["A"])
	Msgf(">         CONST equations: %4d\n", cnt["C"])
	Msgf(">          INIT equations: %4d\n", cnt["N"])
	Msgf(">   Number of TABLE def's: %4d\n", len(mdl.Tables))
}

// AddStatement inserts a new source statement to the model.
// The statement must be formatted according to the DYNAMO language rules.
// The statement describes either equations or runtime instructions, that
// govern the evolution of the system state.
func (mdl *Model) AddStatement(stmt, mode string) (res *Result) {
	res = Success()

	// skip empty statements
	if len(stmt) == 0 {
		return
	}
	// handle statement based on its mode
	switch mode {
	case "*":
		//--------------------------------------------------------------
		// title of model
		mdl.Title = stmt

	case "NOTE":
		//--------------------------------------------------------------
		// skip over comments

	case "L", "R", "C", "N", "A":
		//--------------------------------------------------------------
		// Level and rate equations
		var eqn *Equation
		if eqn, res = NewEquation(stmt, mode); !res.Ok {
			break
		}
		res = mdl.AddEquation(eqn)

	case "T":
		//--------------------------------------------------------------
		// Table definitions
		if strings.Index(stmt, " ") != -1 {
			if strict {
				res = Failure(ErrParseInvalidSpace)
				break
			} else {
				stmt = strings.Replace(stmt, " ", "", -1)
			}
		}
		var tbl *Table
		tab := strings.Split(stmt, "=")
		vals := strings.Replace(tab[1], "/", ",", -1)
		if tbl, res = NewTable(strings.Split(vals, ",")); !res.Ok {
			break
		}
		mdl.Tables[tab[0]] = tbl

	case "SPEC":
		//--------------------------------------------------------------
		// Runtime/simulation parameters
		if strings.Index(stmt, " ") != -1 {
			if strict {
				res = Failure(ErrParseInvalidSpace)
				break
			} else {
				stmt = strings.Replace(stmt, " ", "", -1)
			}
		}
		// model simulation specification
		for _, def := range strings.Split(strings.Replace(stmt, "/", ",", -1), ",") {
			x := strings.Split(def, "=")
			if len(x) != 2 {
				res = Failure(ErrParseSyntax)
				break
			}
			val, err := strconv.ParseFloat(x[1], 64)
			if err != nil {
				res = Failure(err)
				break
			}
			mdl.Consts[x[0]] = Variable(val)
		}

	case "PRINT":
		//--------------------------------------------------------------
		// Print-related parameters
		if strings.Index(stmt, " ") != -1 {
			if strict {
				res = Failure(ErrParseInvalidSpace)
				break
			} else {
				stmt = strings.Replace(stmt, " ", "", -1)
			}
		}
		// print specification
		for i, level := range strings.Split(strings.Replace(stmt, "/", ",", -1), ",") {
			mdl.Print.AddVariable(level, i+1)
		}

	case "PLOT":
		//--------------------------------------------------------------
		// Plot-related parameters
		if strings.Index(stmt, " ") != -1 {
			if strict {
				res = Failure(ErrParseInvalidSpace)
				break
			} else {
				stmt = strings.Replace(stmt, " ", "", -1)
			}
		}
		// plot settings
		if pos := strings.Index(stmt, "("); pos != -1 {
			stmt = stmt[:pos]
		}
		for _, def := range strings.Split(strings.Replace(stmt, "/", ",", -1), ",") {
			x := strings.Split(def, "=")
			if len(x) != 2 {
				res = Failure(ErrParseSyntax)
				break
			}
			mdl.Plot.AddVariable(x[0], []rune(x[1])[0], -1, -1)
		}
	default:
		Dbg.Msgf("Unknown mode '%s'\n", mode)
		res = Failure(ErrParseInvalidMode+": %s", mode)
	}
	return
}

// AddEquation inserts a new equation into the list at the best position:
// * Check if stage of target variable is "NEW" (failure if "OLD")
// * Rate equations are inserted at the end (appended)
// * Compute "posBefore" and "posAfter":
//    "posBefore": Position of first equation using this target
//    "posAfter": Position of last target being used in this equation
// * Report cycle error if posBefore < posAfter
// * Insert fails if another equation for the same target variable already exists.
func (mdl *Model) AddEquation(eqn *Equation) (res *Result) {
	res = Success()
	Dbg.Msgf("!!! Inserting '%s'\n", eqn.String())

	// check equation target stage
	if eqn.Target.Stage == NAME_STAGE_OLD {
		res = Failure(ErrModelEqnBadTargetStage)
		return
	}
	// check for matching equation mode and target kind
	if (strings.Index("LAN", eqn.Mode) != -1 && eqn.Target.Kind != NAME_KIND_LEVEL) ||
		(eqn.Mode == "R" && eqn.Target.Kind != NAME_KIND_RATE) ||
		(eqn.Mode == "C" && eqn.Target.Kind != NAME_KIND_CONST) {
		Dbg.Msgf("Mode='%s', Kind=%d\n", eqn.Mode, eqn.Target.Kind)
		res = Failure(ErrModelEqnBadTargetKind)
		return
	}
	// we can savely append rate equations as they are forward in time (and therefore computed last)
	if eqn.Mode == "R" {
		mdl.Eqns = append(mdl.Eqns, eqn)
		return
	}
	// find insertion point
	num := len(mdl.Eqns)
	posBefore, posAfter := num, -1
	for i, e := range mdl.Eqns {
		// we are done at the first rate equation
		if e.Mode == "R" {
			break
		}
		// check for same target
		if e.Target.Compare(eqn.Target) == NAME_MATCH {
			res = Failure(ErrModelEqnOverwrite)
			return
		}
		// update "posBefore" if eqn target is used (in same stage)
		if e.DependsOn(eqn.Target) && i < posBefore {
			posBefore = i
		}
		// update "posAfter" if eqn uses existing target (in same stage)
		if eqn.DependsOn(e.Target) {
			posAfter = i
		}
	}
	Dbg.Msgf("***%d:%d\n", posBefore, posAfter)
	if posBefore < posAfter {
		res = Failure(ErrModelDependencyLoop)
		return
	}
	newEqns := make([]*Equation, num+1)
	copy(newEqns, mdl.Eqns[:posAfter+1])
	newEqns[posAfter+1] = eqn
	copy(newEqns[posAfter+2:], mdl.Eqns[posAfter+1:])
	mdl.Eqns = newEqns

	for i, e := range mdl.Eqns {
		Dbg.Msgf("[%d] %s | %s=%v\n", i, e, e.Target, e.Dependencies)
	}
	return
}

// Get returns the value of the named variable. The variable can either be
// a constant, a system parameter (like DT or a system/printer/plotter setting)
// or a level value (current, previous).
func (mdl *Model) Get(name *Name) (val Variable, res *Result) {
	res = Success()
	defer func() {
		Dbg.Msgf("Get('%s',%d,%d) = %f\n", name.Name, name.Kind, name.Stage, val)
	}()

	var ok bool
	if name.Stage == NAME_STAGE_NONE {
		if val, ok = mdl.Consts[name.Name]; ok {
			return
		}
		if val, ok = mdl.Current[name.Name]; ok {
			return
		}
	}
	if name.Stage == NAME_STAGE_NEW {
		if val, ok = mdl.Current[name.Name]; ok {
			return
		}
	}
	if name.Stage == NAME_STAGE_OLD {
		if val, ok = mdl.Last[name.Name]; ok {
			return
		}
	}
	res = Failure(ErrModelNoVariable+": %s", name.Name)
	return
}

// Set the value of the named variable. The variable can either be a constant,
// a system parameter (like DT or a system/printer/plotter setting) or a level
// value (current, previous).
func (mdl *Model) Set(name *Name, val Variable) (res *Result) {
	res = Success()
	switch name.Kind {
	case NAME_KIND_CONST:
		mdl.Consts[name.Name] = val
	case NAME_KIND_LEVEL, NAME_KIND_RATE:
		mdl.Current[name.Name] = val
	}
	Msgf(">    %s = %f\n", name, val)
	return
}

//----------------------------------------------------------------------
// DYNAMO model runtime
//----------------------------------------------------------------------

// Run a DYNAMO model.
func (mdl *Model) Run() (res *Result) {
	res = Success()

	// compute all equations with specified mode
	compute := func(modes string) (res *Result) {
		res = Success()
		for _, eqn := range mdl.Eqns {
			if strings.Contains(modes, eqn.Mode) {
				if res = eqn.Eval(mdl); !res.Ok {
					Dbg.Msg(eqn.String())
					break
				}
			}
		}
		return
	}

	// Initialize constants
	Msg("   Initializing constants:")
	if res = compute("C"); !res.Ok {
		return
	}
	// Intialize levels
	Msg("   Initializing levels:")
	if res = compute("N"); !res.Ok {
		return
	}
	// keep a list of a variables (level,rate)
	varList := make([]string, 0)

	// Check if all levels have level equations
	Msg("   Checking levels:")
	check := make(map[string]bool)
	ok := true
	for level, _ := range mdl.Current {
		check[level] = false
		varList = append(varList, level)
	}
	for _, eqn := range mdl.Eqns {
		if eqn.Mode != "L" {
			continue
		}
		level := eqn.Target.Name
		if _, ok := check[level]; ok {
			check[level] = true
		} else {
			Msgf(">     %s not initialized\n", level)
			ok = false
		}
	}
	for level, val := range check {
		if !val {
			Msgf(">     %s has no equation\n", level)
			ok = false
		}
	}
	if ok {
		Msg("      No problems detected.")
	}
	// get targets of rate equations
	for _, eqn := range mdl.Eqns {
		if eqn.Mode != "R" {
			continue
		}
		varList = append(varList, eqn.Target.Name)
	}

	// Start printer and plotter
	mdl.Print.Start()
	mdl.Plot.Start()

	// Running the model
	Msg("   Iterating epochs:")
	dt := mdl.Consts["DT"]
	length := mdl.Consts["LENGTH"]
	time, ok := mdl.Current["TIME"]
	if !ok {
		time = 0.0
	}
	mdl.Current["TIME"] = time
	for epoch, t := 1, time; t <= length; epoch, t = epoch+1, t+dt {
		// compute auxiliaries
		Msgf("      Epoch %d:\n", epoch)
		Msg("         Evaluating AUX equations:")
		if res = compute("A"); !res.Ok {
			break
		}
		// compute rates
		Msg("         Evaluating RATE equations:")
		if res = compute("R"); !res.Ok {
			break
		}
		// emit current values for plot and print
		if res = mdl.Print.Add(); !res.Ok {
			break
		}
		if res = mdl.Plot.Add(); !res.Ok {
			break
		}
		// Propagate in time
		mdl.Last = mdl.Current
		mdl.Current = make(State)
		for level, val := range mdl.Last {
			mdl.Current[level] = val
		}
		mdl.Current["TIME"] = mdl.Current["TIME"] + mdl.Consts["DT"]
		// compute new levels
		Msg("         Evaluating LEVEL equations:")
		if res = compute("L"); !res.Ok {
			break
		}
		Dbg.Msgf("[%d] %v\n", epoch, mdl.Current)
	}
	return
}
