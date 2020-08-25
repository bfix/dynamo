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

var (
	strict = false // apply strict DYNAMO language rules
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
// MODEL as defined in a DYNAMO source
//
// This implementation does not make a difference between constants,
// rates, levels, auxiliaries - all variables go into a single "state".
// The only drawback is that names must be unique across types.
// The model keeps two states (LAST, CURRENT).
//----------------------------------------------------------------------

// Model represents a DYNAMO model that can be executed
type Model struct {
	Title   string              // title of the model as defined by mode "*"
	RunID   string              // identifier for model run
	Eqns    *EqnList            // list of equations
	Tables  map[string]*Table   // list of tables
	Last    State               // previous state (J)
	Current State               // current state (K)
	Print   *Printer            // printer instance
	Plot    *Plotter            // plotter instance
	Verbose bool                // verbose messaging
	Stack   map[string]*EqnList // stacked run models
	Edit    bool                // editing model?
}

// NewModel returns a new (empty) model instance.
func NewModel(printer, plotter string) *Model {
	mdl := &Model{
		Eqns:    NewEqnList(),
		Tables:  make(map[string]*Table),
		Last:    make(State),
		Current: make(State),
		Verbose: false,
		Stack:   make(map[string]*EqnList),
		Edit:    false,
	}
	mdl.Print = NewPrinter(printer, mdl)
	mdl.Plot = NewPlotter(plotter, mdl)
	return mdl
}

// Set strict mode (globally)
func (mdl *Model) SetStrict(flag bool) {
	strict = flag
}

// Output is called after a model is run to generate prints and plots.
func (mdl *Model) Output() (res *Result) {
	if res = mdl.Print.Generate(); !res.Ok {
		return
	}
	res = mdl.Plot.Generate()
	return
}

// Quit is called when done with a model.
func (mdl *Model) Quit() (res *Result) {
	// close all outputs
	if res = Dbg.Close(); !res.Ok {
		return
	}
	if res = mdl.Print.Close(); !res.Ok {
		return
	}
	res = mdl.Plot.Close()
	return
}

// Dump logs the current model state in human-readable form into
// the log stream.
func (mdl *Model) Dump() {

	mdl.Eqns.Dump(mdl.Verbose)
	Msg("-----------------------------------")
	Msgf(" Number of TABLE def's: %4d\n", len(mdl.Tables))
	for tname, tbl := range mdl.Tables {
		Msgf("   %s: %v\n", tname, tbl.Data)
	}
	Msg("-----------------------------------")
}

// AddStatement inserts a new source statement to the model.
// The statement must be formatted according to the DYNAMO language rules.
// The statement describes either equations or runtime instructions, that
// govern the evolution of the system state.
func (mdl *Model) AddStatement(stmt *Line) (res *Result) {
	res = Success()

	// skip empty and detect invalid statements
	if stmt == nil {
		return
	}
	line := stmt.Stmt
	if len(line) == 0 {
		return
	}
	prepLine := func() *Result {
		if strings.Index(line, " ") != -1 {
			if strict {
				return Failure(ErrParseInvalidSpace)
			} else {
				line = strings.Replace(line, " ", "", -1)
			}
		}
		return Success()
	}
	Dbg.Msgf("AddStmt: [%s] %s\n", stmt.Mode, stmt.Stmt)

	// handle statement based on its mode
	switch stmt.Mode {
	case "*":
		//--------------------------------------------------------------
		// title of model
		mdl.Title = stmt.Stmt

	case "NOTE":
		//--------------------------------------------------------------
		// skip over comments

	case "L", "R", "C", "N", "A", "S":
		//--------------------------------------------------------------
		// Level and rate equations
		var eqns *EqnList
		if eqns, res = NewEquation(stmt); !res.Ok {
			break
		}
		for _, eqn := range eqns.List() {
			// check if equation has correct temporality and kind
			// (don't check dependencies at this stage)
			if res = eqns.validateEqn(mdl, eqn, nil); !res.Ok {
				break
			}
			// check if equation is already defined.
			if mdl.Eqns.Contains(eqn) {
				if !mdl.Edit {
					res = Failure(ErrModelEqnOverwrite)
				}
				Dbg.Msgf("ReplaceEquation: %s\n", eqn.String())
				mdl.Eqns.Replace(eqn)
			} else {
				// unsorted append to list of equations
				Dbg.Msgf("AddEquation: %s\n", eqn.String())
				mdl.Eqns.Add(eqn)
			}
		}

	case "T":
		//--------------------------------------------------------------
		// Table definitions
		if res = prepLine(); !res.Ok {
			break
		}
		var tbl *Table
		tab := strings.Split(line, "=")
		vals := strings.Replace(tab[1], "/", ",", -1)
		if tbl, res = NewTable(strings.Split(vals, ",")); !res.Ok {
			break
		}
		mdl.Tables[tab[0]] = tbl

	case "SPEC":
		//--------------------------------------------------------------
		// Runtime/simulation parameters
		// This is an optional statement; the same effect can be achieved
		// by defining "C" equations for the parameters.
		if res = prepLine(); !res.Ok {
			break
		}
		// model simulation specification
		if mdl.Verbose {
			Msg("   Runtime specification:")
		}
		for _, def := range strings.Split(strings.Replace(line, "/", ",", -1), ",") {
			var eqns *EqnList
			stmt := &Line{
				Stmt: def,
				Mode: "C",
			}
			if eqns, res = NewEquation(stmt); !res.Ok {
				break
			}
			mdl.Eqns.AddList(eqns)

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
			if mdl.Verbose {
				Msgf("        %s = %f\n", x[0], val)
			}
		}

	case "PRINT":
		//--------------------------------------------------------------
		// Print-related parameters
		if res = prepLine(); !res.Ok {
			break
		}
		// set print specification
		res = mdl.Print.Prepare(line)

	case "PLOT":
		//--------------------------------------------------------------
		// Plot-related parameters
		if res = prepLine(); !res.Ok {
			break
		}
		// set plot specification
		res = mdl.Plot.Prepare(line)

	case "RUN":
		//--------------------------------------------------------------
		// Run model
		mdl.Edit = false
		mdl.RunID = stmt.Stmt
		Msgf("   Running system model '%s'...", mdl.RunID)
		if res = mdl.Run(); res.Ok {
			res = mdl.Output()
			// Stack model equations for later use
			mdl.Stack[mdl.RunID] = mdl.Eqns.Clone()
		}
		Msg("      Done.")

	case "EDIT":
		//--------------------------------------------------------------
		// Edit stacked model:
		// get named model equations
		eqns, ok := mdl.Stack[stmt.Stmt]
		if !ok {
			res = Failure(ErrModelNotAvailable+": %s", stmt.Stmt)
			break
		}
		Msgf("   Editing system model '%s':", mdl.RunID)
		mdl.Eqns = eqns
		mdl.Edit = true
		// reset output
		mdl.Print.Reset()
		mdl.Plot.Reset()
		// reset system vars
		mdl.Current["TIME"] = 0
		// reset states
		mdl.Last = make(State)
		mdl.Current = make(State)

	default:
		Dbg.Msgf("Unknown mode '%s'\n", stmt.Mode)
		res = Failure(ErrParseInvalidMode+": %s", stmt.Mode)
	}
	return
}

//----------------------------------------------------------------------
// Getter/Setter methods for DYNAMO variables (levels, rates, constants)
//----------------------------------------------------------------------

// Get returns the value of the named variable. The variable can either be
// a constant, a system parameter (like DT or a system/printer/plotter setting)
// or a level value (current, previous).
func (mdl *Model) Get(name *Name) (val Variable, res *Result) {
	res = Success()
	defer func() {
		Dbg.Msgf("<   %s = %f (%d)\n", name, val, name.Stage)
	}()

	var ok bool
	switch name.Stage {
	case NAME_STAGE_NONE, NAME_STAGE_NEW:
		if val, ok = mdl.Current[name.Name]; ok {
			return
		}
	case NAME_STAGE_OLD:
		if val, ok = mdl.Last[name.Name]; ok {
			return
		}
	}
	res = Failure(ErrModelNoVariable+": %s", name.String())
	return
}

// Set the value of the named variable. The variable can either be a constant,
// a system parameter (like DT or a system/printer/plotter setting) or a level
// value (current, previous).
func (mdl *Model) Set(name *Name, val Variable) (res *Result) {
	res = Success()
	mdl.Current[name.Name] = val
	Dbg.Msgf(">   %s = %f (%d)\n", name, val, name.Stage)
	return
}

// IsSystem returns true for pre-defined system variables.
func (mdl *Model) IsSystem(name string) bool {
	// check for pre-defined variable names
	if strings.Index("TIME,DT,LENGTH,PLTPER,PRTPER,", name+",") != -1 {
		return true
	}
	// skip table names
	for tbl, _ := range mdl.Tables {
		if name == tbl {
			return true
		}
	}
	return false
}

//----------------------------------------------------------------------
// DYNAMO model runtime
//----------------------------------------------------------------------

// Run a DYNAMO model.
func (mdl *Model) Run() (res *Result) {
	res = Success()

	// sort equations "topologically" after parsing
	if mdl.Eqns, res = mdl.Eqns.Sort(mdl); !res.Ok {
		return
	}
	// perform equation validation
	if res = mdl.Eqns.Validate(mdl); !res.Ok {
		return
	}
	if mdl.Verbose {
		mdl.Dump()
	}

	// compute all equations with specified mode
	compute := func(modes string, eqns *EqnList) (res *Result) {
		res = Success()
		for _, eqn := range eqns.List() {
			if strings.Contains(modes, eqn.Mode) {
				if res = eqn.Eval(mdl); !res.Ok {
					Dbg.Msg(eqn.String())
					break
				}
			}
		}
		return
	}

	// compute split in equation list between "init" and "run"
	split := 0
	for i, eqn := range mdl.Eqns.List() {
		if strings.Contains("CN", eqn.Mode) {
			split = i + 1
		}
	}
	if mdl.Verbose {
		Msgf("      INFO: Splitting equations: INIT=[1..%d], RUN=[%d..%d]\n", split, split+1, mdl.Eqns.Len())
	}
	initEqns, runEqns := mdl.Eqns.Split(split)

	//------------------------------------------------------------------
	// Initialize state:
	//------------------------------------------------------------------
	Msg("      Initializing state...")

	// initialize from equations
	if res = compute("CNRA", initEqns); !res.Ok {
		return
	}
	// set predefined (system) variables if not defined
	setDef := func(name string, val Variable) {
		if _, ok := mdl.Current[name]; !ok {
			Msgf("         INFO: Setting '%s' to %f\n", name, val)
			mdl.Current[name] = val
		}
	}
	setDef("TIME", 0)
	setDef("DT", 0.1)
	setDef("LENGTH", 10)
	setDef("PRTPER", 0)
	setDef("PLTPER", 0)

	// keep a list of a variables (level,rate)
	varList := make([]string, 0)

	// Check if all levels have level equations
	Msg("      Checking state...")
	check := make(map[string]bool)
	used := make(map[string]bool)
	ok := true
	for level, _ := range mdl.Current {
		if level[0] == '_' {
			continue
		}
		check[level] = false
		varList = append(varList, level)
	}
	for _, eqn := range mdl.Eqns.List() {
		for _, dep := range eqn.Dependencies {
			used[dep.Name] = true
		}
		for _, ref := range eqn.References {
			used[ref.Name] = true
		}
		if strings.Index("CRA", eqn.Mode) != -1 {
			check[eqn.Target.Name] = true
			continue
		}
		level := eqn.Target.Name
		if _, ok := check[level]; ok {
			check[level] = true
		} else {
			if eqn.Mode != "S" {
				Msgf("         %s not initialized\n", level)
			}
			ok = false
		}
	}
	for level, val := range check {
		if mdl.IsSystem(level) {
			continue
		}
		if !val {
			Msgf("         %s has no equation\n", level)
			ok = false
		} else if _, inuse := used[level]; !inuse {
			Msgf("         %s not used\n", level)
			ok = false
		}
	}
	if ok {
		Msg("         No problems detected.")
	}
	// get targets of rate equations
	for _, eqn := range mdl.Eqns.List() {
		if eqn.Mode != "R" {
			continue
		}
		varList = append(varList, eqn.Target.Name)
	}

	// Start printer and plotter
	if res = mdl.Print.Start(); !res.Ok {
		return
	}
	if res = mdl.Plot.Start(); !res.Ok {
		return
	}

	// Running the model
	Msg("      Iterating epochs...")
	dt := mdl.Current["DT"]
	length := mdl.Current["LENGTH"]
	time, ok := mdl.Current["TIME"]
	if !ok {
		time = 0.0
		mdl.Current["TIME"] = time
	}

	for epoch, t := 1, time; t <= length; epoch, t = epoch+1, t+dt {
		// compute auxiliaries and rates
		if res = compute("AR", runEqns); !res.Ok {
			break
		}
		// propagate state
		mdl.Last = mdl.Current
		mdl.Current = make(State)
		for level, val := range mdl.Last {
			mdl.Current[level] = val
		}
		// compute new levels and supplements
		if res = compute("LS", runEqns); !res.Ok {
			break
		}
		// emit current values for plot and print
		if res = mdl.Print.Add(epoch); !res.Ok {
			break
		}
		if res = mdl.Plot.Add(epoch); !res.Ok {
			break
		}
		// propagate in time
		mdl.Current["TIME"] = mdl.Current["TIME"] + mdl.Current["DT"]
	}
	return
}
