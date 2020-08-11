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
	"strconv"
	"strings"
)

const (
	strict = true // apply strict DYNAMO language rules
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
	Title   string            // title of the model as defined by mode "*"
	RunID   string            // identifier for model run
	Eqns    []*Equation       // list of equations
	Tables  map[string]*Table // list of tables
	Last    State             // previous state (J)
	Current State             // current state (K)
	Print   *Printer          // printer instance
	Plot    *Plotter          // plotter instance
	Verbose bool              // verbose messaging
}

// NewModel returns a new (empty) model instance.
func NewModel(printer, plotter string) *Model {
	mdl := &Model{
		Eqns:    make([]*Equation, 0),
		Tables:  make(map[string]*Table),
		Last:    make(State),
		Current: make(State),
		Verbose: false,
	}
	mdl.Print = NewPrinter(printer, mdl)
	mdl.Plot = NewPlotter(plotter, mdl)
	return mdl
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
	Msg("-----------------------------------")
	Msgf("   Number of equations: %4d\n", len(mdl.Eqns))
	Msgf("       LEVEL equations: %4d\n", cnt["L"])
	Msgf("        RATE equations: %4d\n", cnt["R"])
	Msgf("         AUX equations: %4d\n", cnt["A"])
	Msgf("       CONST equations: %4d\n", cnt["C"])
	Msgf("        INIT equations: %4d\n", cnt["N"])
	Msg("-----------------------------------")
	for i, e := range mdl.Eqns {
		Msgf("   %5d: %s\n", i+1, e.String())
	}
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

	case "L", "R", "C", "N", "A":
		//--------------------------------------------------------------
		// Level and rate equations
		var eqns []*Equation
		if eqns, res = NewEquation(stmt); !res.Ok {
			break
		}
	loop:
		for _, eqn := range eqns {
			// check if equation has correct temporality
			if eqn.Target.Stage != NAME_STAGE_NEW && strings.Contains("LRA", eqn.Mode) {
				res = Failure(ErrModelEqnBadTargetStage)
				break
			}
			// check for matching equation mode and target kind
			if (strings.Index("LA", eqn.Mode) != -1 && eqn.Target.Kind != NAME_KIND_LEVEL) ||
				(eqn.Mode == "N" && eqn.Target.Kind != NAME_KIND_INIT) ||
				(eqn.Mode == "R" && eqn.Target.Kind != NAME_KIND_RATE) ||
				(eqn.Mode == "C" && eqn.Target.Kind != NAME_KIND_CONST) {
				res = Failure(ErrModelEqnBadTargetKind)
				break
			}
			// check if equation is not defined yet.
			for _, e := range mdl.Eqns {
				if e.Target.Compare(eqn.Target) == NAME_MATCH {
					res = Failure(ErrModelEqnOverwrite)
					break loop
				}
			}
			// unsorted append to list of equations
			Dbg.Msgf("AddEquation: %s\n", eqn.String())
			mdl.Eqns = append(mdl.Eqns, eqn)
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
		if res = prepLine(); !res.Ok {
			break
		}
		// model simulation specification
		if mdl.Verbose {
			Msg("   Runtime specification:")
		}
		for _, def := range strings.Split(strings.Replace(line, "/", ",", -1), ",") {
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
			mdl.Current[x[0]] = Variable(val)
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
		mdl.RunID = stmt.Stmt

	default:
		Dbg.Msgf("Unknown mode '%s'\n", stmt.Mode)
		res = Failure(ErrParseInvalidMode+": %s", stmt.Mode)
	}
	return
}

//----------------------------------------------------------------------
// Sorting DYNAMO equations based on dependencies (topological sort)
//----------------------------------------------------------------------

// An eqnEntry represents an equation in the list 'mdl.Eqns' at given position
// 'pos'. It keeps a list of dependencies; a dependency referes to anther
// equation that is referenced in this equation (on the right side).
type eqnEntry struct {
	pos  int
	name string
	deps map[int]bool
}

// String returns a human-readable representation of an entry
func (e *eqnEntry) String() string {
	deps := ""
	for i := range e.deps {
		if len(deps) > 0 {
			deps += ","
		}
		deps += strconv.Itoa(i)
	}
	if len(deps) > 0 {
		return fmt.Sprintf("{%d:%s|%s}", e.pos, e.name, deps)
	}
	return fmt.Sprintf("{%d:%s}", e.pos, e.name)
}

// create new entry for equation position
func newEntry(i int, name string) *eqnEntry {
	return &eqnEntry{
		pos:  i,
		name: name,
		deps: make(map[int]bool),
	}
}

// SortEquations sorts an equation list "topologically" based on dependencies.
func (mdl *Model) SortEquations() (res *Result) {
	res = Success()

	// Kahn's algorithm (1962) is used for sorting.
	eqnSort := func(list, ref map[string]*eqnEntry) (out []int, res *Result) {
		res = Success()
		for _, entry := range list {
			eqn := mdl.Eqns[entry.pos]
		loop:
			for _, d := range eqn.Dependencies {
				// skip system variables
				if strings.Index("TIME,DT,", d.Name+",") != -1 {
					continue
				}
				// skip table names
				for tbl, _ := range mdl.Tables {
					if d.Name == tbl {
						continue loop
					}
				}
				// get defining equation for dependency
				x, ok := list[d.Name]
				if ok {
					if x.pos != entry.pos {
						entry.deps[x.pos] = true
					}
					continue
				}
				if ref != nil {
					_, ok = ref[d.Name]
				}
				if !ok {
					Dbg.Msgf("Failed in %s:\n", eqn.String())
					res = Failure(ErrModelUnknownEqn+": %s", d.Name)
					break
				}
			}
		}
		Dbg.Msgf(">> %v\n", list)
		var (
			L     []*eqnEntry // empty list that will contain the sorted elements
			S     []*eqnEntry // set of all nodes with no incoming edge
			graph []*eqnEntry // list of pending nodes in graph
		)
		for _, entry := range list {
			if len(entry.deps) == 0 {
				S = append(S, entry)
			} else {
				graph = append(graph, entry)
			}
		}
		for len(S) > 0 {
			n := S[0]
			S = S[1:]
			L = append(L, n)
			var newGraph []*eqnEntry
			for _, m := range graph {
				if _, ok := m.deps[n.pos]; ok {
					delete(m.deps, n.pos)
				}
				if len(m.deps) == 0 {
					S = append(S, m)
				} else {
					newGraph = append(newGraph, m)
				}
			}
			graph = newGraph
		}
		if len(graph) > 0 {
			Msg("Cyclic dependencies detected:")
			for _, e := range graph {
				eqn := mdl.Eqns[e.pos]
				Msg(">> " + eqn.String())
			}
			res = Failure(ErrModelDependencyLoop)
		} else {
			// build re-ordered equation list
			for _, entry := range L {
				out = append(out, entry.pos)
			}
		}
		return
	}

	// we build two separate equation lists: one for non-levels ("C", "N", "A"
	// and "R") and one for levels ("L").
	res = Success()
	Dbg.Msgf("SortEquations: Sorting %d equations...\n", len(mdl.Eqns))
	eqnInit := make(map[string]*eqnEntry)
	eqnRun := make(map[string]*eqnEntry)
	for i, eqn := range mdl.Eqns {
		name := eqn.Target.Name
		Dbg.Msgf("SortEquations << [%d] %s\n", i, eqn.String())
		if strings.Index("CNRA", eqn.Mode) != -1 {
			if _, ok := eqnInit[name]; ok {
				return Failure(ErrModelVariabeExists+": %s", name)
			}
			eqnInit[name] = newEntry(i, name)
		} else if strings.Index("L", eqn.Mode) != -1 {
			if _, ok := eqnRun[name]; ok {
				return Failure(ErrModelVariabeExists+": %s", name)
			}
			eqnRun[name] = newEntry(i, name)
		} else {
			return Failure(ErrModelEqnBadMode)
		}
	}
	// sort both lists
	var listInit, listRun []int
	Dbg.Msg("Sorting eqnInit...")
	if listInit, res = eqnSort(eqnInit, nil); res.Ok {
		Dbg.Msg("Sorting eqnRun...")
		if listRun, res = eqnSort(eqnRun, eqnInit); res.Ok {
			// build re-ordered equation list
			var eqns []*Equation
			for _, i := range listInit {
				eqns = append(eqns, mdl.Eqns[i])
			}
			for _, i := range listRun {
				eqns = append(eqns, mdl.Eqns[i])
			}
			mdl.Eqns = eqns
			Dbg.Msgf("SortEquations: Finishing %d equations...\n", len(mdl.Eqns))
			for i, eqn := range mdl.Eqns {
				Dbg.Msgf("SortEquations >> [%d] %s\n", i, eqn.String())
			}
		}
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
		Dbg.Msgf("Get('%s',%d,%d) = %f\n", name.Name, name.Kind, name.Stage, val)
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
	res = Failure(ErrModelNoVariable+": '%s'", name.Name)
	return
}

// Set the value of the named variable. The variable can either be a constant,
// a system parameter (like DT or a system/printer/plotter setting) or a level
// value (current, previous).
func (mdl *Model) Set(name *Name, val Variable) (res *Result) {
	res = Success()
	mdl.Current[name.Name] = val
	Dbg.Msgf(">    %s = %f\n", name, val)
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
	// Initialize state:
	Msg("   Initializing state...")
	mdl.Current["TIME"] = 0
	if res = compute("CNRA"); !res.Ok {
		return
	}
	// keep a list of a variables (level,rate)
	varList := make([]string, 0)

	// Check if all levels have level equations
	Msg("   Checking state...")
	check := make(map[string]bool)
	used := make(map[string]bool)
	ok := true
	for level, _ := range mdl.Current {
		check[level] = false
		varList = append(varList, level)
	}
	for _, eqn := range mdl.Eqns {
		for _, dep := range eqn.Dependencies {
			used[dep.Name] = true
		}
		if strings.Index("CRA", eqn.Mode) != -1 {
			check[eqn.Target.Name] = true
			continue
		}
		level := eqn.Target.Name
		if _, ok := check[level]; ok {
			check[level] = true
		} else {
			Msgf("      %s not initialized\n", level)
			ok = false
		}
	}
	for level, val := range check {
		if strings.Index("TIME;DT;LENGTH;PRTPER;PLTPER;", level+";") != -1 {
			continue
		}
		if !val {
			Msgf("      %s has no equation\n", level)
			ok = false
		} else if _, inuse := used[level]; !inuse {
			Msgf("      %s not used\n", level)
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
	if res = mdl.Print.Start(); !res.Ok {
		return
	}
	if res = mdl.Plot.Start(); !res.Ok {
		return
	}

	// Running the model
	Msg("   Iterating epochs...")
	dt := mdl.Current["DT"]
	length := mdl.Current["LENGTH"]
	time, ok := mdl.Current["TIME"]
	if !ok {
		time = 0.0
		mdl.Current["TIME"] = time
	}

	for epoch, t := 1, time; t <= length; epoch, t = epoch+1, t+dt {
		// compute auxiliaries
		if mdl.Verbose {
			Msgf("      Epoch %d:\n", epoch)
			Msg("         Evaluating AUX + RATE equations:")
		}
		if res = compute("AR"); !res.Ok {
			break
		}
		// emit current values for plot and print
		if res = mdl.Print.Add(epoch); !res.Ok {
			break
		}
		if res = mdl.Plot.Add(epoch); !res.Ok {
			break
		}
		// propagate state
		mdl.Last = mdl.Current
		mdl.Current = make(State)
		for level, val := range mdl.Last {
			mdl.Current[level] = val
		}
		// compute new levels
		if mdl.Verbose {
			Msg("         Evaluating LEVEL equations:")
		}
		if res = compute("L"); !res.Ok {
			break
		}
		Dbg.Msgf("[%d] %v\n", epoch, mdl.Current)
		// propagate in time
		mdl.Current["TIME"] = mdl.Current["TIME"] + mdl.Current["DT"]
	}
	return
}
