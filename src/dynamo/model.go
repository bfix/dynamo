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
// TABLE to model functions of form "Y = TABLE(X)" (or TABXT or TABPL)
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
	Msgf(">   Number of equations: %4d\n", len(mdl.Eqns))
	Msgf(">       LEVEL equations: %4d\n", cnt["L"])
	Msgf(">        RATE equations: %4d\n", cnt["R"])
	Msgf(">         AUX equations: %4d\n", cnt["A"])
	Msgf(">       CONST equations: %4d\n", cnt["C"])
	Msgf(">        INIT equations: %4d\n", cnt["N"])
	Msg("-----------------------------------")
	for i, e := range mdl.Eqns {
		Msgf("> %5d: %s\n", i+1, e.String())
	}
	Msg("-----------------------------------")
	Msgf("> Number of TABLE def's: %4d\n", len(mdl.Tables))
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
			if (strings.Index("LAN", eqn.Mode) != -1 && eqn.Target.Kind != NAME_KIND_LEVEL) ||
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
			mdl.Consts[x[0]] = Variable(val)
		}

	case "PRINT":
		//--------------------------------------------------------------
		// Print-related parameters
		if res = prepLine(); !res.Ok {
			break
		}
		// print specification
		for i, level := range strings.Split(strings.Replace(line, "/", ",", -1), ",") {
			mdl.Print.AddVariable(level, i+1)
		}

	case "PLOT":
		//--------------------------------------------------------------
		// Plot-related parameters
		if res = prepLine(); !res.Ok {
			break
		}
		// plot settings
		if pos := strings.Index(line, "("); pos != -1 {
			line = line[:pos]
		}
		for _, def := range strings.Split(strings.Replace(line, "/", ",", -1), ",") {
			x := strings.Split(def, "=")
			if len(x) != 2 {
				res = Failure(ErrParseSyntax)
				break
			}
			mdl.Plot.AddVariable(x[0], []rune(x[1])[0], -1, -1)
		}
	default:
		Dbg.Msgf("Unknown mode '%s'\n", stmt.Mode)
		res = Failure(ErrParseInvalidMode+": %s", stmt.Mode)
	}
	return
}

//----------------------------------------------------------------------
// Sorting DYNAMO equations based on dependencies (topological sort)
//----------------------------------------------------------------------

// An entry represents an equation in the list (at given position 'pos')
// and a list of dependencies. A dependency referes to anther equation
// that is referenced in this equation (on the right side).
type entry struct {
	pos  int
	deps map[int]bool
}

// String returns a human-readable representation of an entry
func (e *entry) String() string {
	deps := ""
	for i := range e.deps {
		if len(deps) > 0 {
			deps += ","
		}
		deps += strconv.Itoa(i)
	}
	if len(deps) > 0 {
		return fmt.Sprintf("%d/%s", e.pos, deps)
	}
	return strconv.Itoa(e.pos)
}

// SortEquations sorts an  equation list "topologically" based on dependencies.
// Kahn's algorithm (1962) is used for sorting.
func (mdl *Model) SortEquations() (res *Result) {
	res = Success()

	// pass 1: build set of entries from equations
	index := make(map[string]*entry)
	for i, eqn := range mdl.Eqns {
		index[eqn.Target.Name] = &entry{
			pos:  i,
			deps: make(map[int]bool),
		}
	}
	// pass 2: build dependencies
	for _, e := range index {
		eqn := mdl.Eqns[e.pos]
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
			x, ok := index[d.Name]
			if !ok {
				res = Failure(ErrModelUnknownEqn+": %s", d.Name)
				return
			}
			e.deps[x.pos] = true
		}
	}

	// L ← Empty list that will contain the sorted elements
	// S ← Set of all nodes with no incoming edge
	var L, S, G []*entry
	for _, e := range index {
		if len(e.deps) == 0 {
			S = append(S, e)
		} else {
			G = append(G, e)
		}
	}
	for len(S) > 0 {
		n := S[0]
		S = S[1:]
		L = append(L, n)
		var Gnew []*entry
		for _, m := range G {
			if _, ok := m.deps[n.pos]; ok {
				delete(m.deps, n.pos)
			}
			if len(m.deps) == 0 {
				S = append(S, m)
			} else {
				Gnew = append(Gnew, m)
			}
		}
		G = Gnew
	}
	if len(G) > 0 {
		Msg("Cyclic dependencies detected:")
		for _, e := range G {
			eqn := mdl.Eqns[e.pos]
			Msg(">> " + eqn.String())
		}
		res = Failure(ErrModelDependencyLoop)
	} else {
		// build re-ordered equation list
		eqns := make([]*Equation, 0)
		for _, e := range L {
			eqns = append(eqns, mdl.Eqns[e.pos])
		}
		mdl.Eqns = eqns
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
