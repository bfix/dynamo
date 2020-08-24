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
	Msgf("       SUPPL equations: %4d\n", cnt["S"])
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

	case "L", "R", "C", "N", "A", "S":
		//--------------------------------------------------------------
		// Level and rate equations
		var eqns []*Equation
		if eqns, res = NewEquation(stmt); !res.Ok {
			break
		}
	loop:
		for _, eqn := range eqns {
			// check if equation has correct temporality and kind
			// (don't check dependencies at this stage)
			if res = mdl.validateEqn(eqn, nil); !res.Ok {
				break
			}
			// check if equation is already defined.
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
			for _, d := range eqn.Dependencies {
				// skip system variables
				if mdl.IsSystem(d.Name) {
					continue
				}
				// check if dependency is refering to previous stage
				if d.Stage == NAME_STAGE_OLD || !eqn.ForcedDeps {
					continue
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
				Msgf(">> [%d] %s {%v}\n", e.pos, eqn.String(), e.deps)
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
		if strings.Index("CNA", eqn.Mode) != -1 {
			if _, ok := eqnInit[name]; ok {
				return Failure(ErrModelVariabeExists+": %s", name)
			}
			eqnInit[name] = newEntry(i, name)
		} else if strings.Index("RL", eqn.Mode) != -1 {
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
	res = Failure(ErrModelNoVariable+": '%s'", name.String())
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

func (mdl *Model) IsSystem(name string) bool {
	// check for pre-defined variables
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
// Validate equations in model
//----------------------------------------------------------------------

func (mdl *Model) Validate() *Result {
	// build list of variable equations
	list := make(map[string]*Equation)
	for _, eqn := range mdl.Eqns {
		name := eqn.Target.String()
		if _, ok := list[name]; ok {
			return Failure(ErrModelEqnAmbigious)
		}
		list[name] = eqn
	}
	// check all equations
	for _, eqn := range mdl.Eqns {
		// check if equation has correct dependencies
		if res := mdl.validateEqn(eqn, list); !res.Ok {
			Dbg.Msgf("*** %s\n", eqn.String())
			return res
		}
	}
	return Success()
}

// Validate checks the equation for correctness.
func (mdl *Model) validateEqn(eqn *Equation, list map[string]*Equation) (res *Result) {

	// check equation target and dependencies.
	check := func(target *Class, deps []*Class) *Result {
		if eqn.Target.Kind != target.Kind {
			return Failure(ErrModelEqnBadTargetKind+": %d", eqn.Target.Kind)
		}
		if eqn.Target.Stage != target.Stage {
			return Failure(ErrModelEqnBadTargetStage+": %d", eqn.Target.Stage)
		}
		if list != nil {
			for _, d := range eqn.Dependencies {
				if mdl.IsSystem(d.Name) {
					continue
				}
				found := false
				name := d.String()
				ref, ok := list[name]
				if !ok {
					// if the missing equation is for a constant, check if we
					// have a matching initializer. If it is for a level, look
					// for a matching auxilliary.
					if strings.HasSuffix(name, "/C") {
						name = d.Name + "/I"
						ref, ok = list[name]
					} else if strings.HasSuffix(name, "/L") {
						name = d.Name + "/A"
						ref, ok = list[name]
					}
					if !ok {
						Dbg.Msgf("[1]*** %v\n", eqn)
						Dbg.Msgf("[2]*** %v\n", list)
						return Failure(ErrModelUnknownEqn+": %s", name)
					}
				}
				for _, cl := range deps {
					if ref.Target.Kind == cl.Kind {
						if d.Stage == cl.Stage {
							found = true
							break
						}
					}
				}
				if !found {
					Msgf("[A]*** %v -- %s\n", eqn, eqn.Target.String())
					Msgf("[B]*** %v  -- %s\n", ref, ref.Target.String())
					return Failure(ErrModelEqnBadDependClass+": %s", d.String())
				}
			}
		}
		return Success()
	}
	// perform validation
	switch eqn.Mode {
	case "C":
		// Constant eqn.
		res = check(
			&Class{NAME_KIND_CONST, NAME_STAGE_NONE},
			[]*Class{
				&Class{NAME_KIND_CONST, NAME_STAGE_NONE}, // other constants
			})
	case "N":
		// Initializer eqn.
		res = check(
			&Class{NAME_KIND_INIT, NAME_STAGE_NONE},
			[]*Class{
				&Class{NAME_KIND_CONST, NAME_STAGE_NONE}, // constants
				&Class{NAME_KIND_INIT, NAME_STAGE_NONE},  // other initializers
			})
		if !res.Ok {
			Msgf("   WARN: %s\n", res.Err.Error())
			res = Success()
		}
	case "L":
		// Constant eqn.
		res = check(
			&Class{NAME_KIND_LEVEL, NAME_STAGE_NEW},
			[]*Class{
				&Class{NAME_KIND_CONST, NAME_STAGE_NONE}, // constants
				&Class{NAME_KIND_LEVEL, NAME_STAGE_NEW},  // currnt levels
				&Class{NAME_KIND_LEVEL, NAME_STAGE_OLD},  // old levels
				&Class{NAME_KIND_RATE, NAME_STAGE_OLD},   // rates
			})
	case "R":
		// Rate eqn.
		res = check(
			&Class{NAME_KIND_RATE, NAME_STAGE_NEW},
			[]*Class{
				&Class{NAME_KIND_CONST, NAME_STAGE_NONE}, // constants
				&Class{NAME_KIND_LEVEL, NAME_STAGE_NEW},  // levels
				&Class{NAME_KIND_AUX, NAME_STAGE_NEW},    // aux
				&Class{NAME_KIND_RATE, NAME_STAGE_OLD},   // other rates
			})
		if !res.Ok {
			Msgf("   WARN: %s\n", res.Err.Error())
			res = Success()
		}
	case "A":
		// Auxilliary eqn.
		res = check(
			&Class{NAME_KIND_AUX, NAME_STAGE_NEW},
			[]*Class{
				&Class{NAME_KIND_CONST, NAME_STAGE_NONE}, // constants
				&Class{NAME_KIND_INIT, NAME_STAGE_NONE},  // initializers
				&Class{NAME_KIND_AUX, NAME_STAGE_NEW},    // other auxilieries
				&Class{NAME_KIND_LEVEL, NAME_STAGE_NEW},  // levels
				&Class{NAME_KIND_RATE, NAME_STAGE_NEW},   // rates
			})
	case "S":
		// Supplementary eqn.
		res = check(
			&Class{NAME_KIND_CONST, NAME_STAGE_NONE},
			[]*Class{
				&Class{NAME_KIND_CONST, NAME_STAGE_NONE}, // constants
				&Class{NAME_KIND_AUX, NAME_STAGE_NEW},    // auxilieries
				&Class{NAME_KIND_LEVEL, NAME_STAGE_NEW},  // levels
				&Class{NAME_KIND_RATE, NAME_STAGE_NEW},   // rates
			})
	}
	return
}

//----------------------------------------------------------------------
// DYNAMO model runtime
//----------------------------------------------------------------------

// Run a DYNAMO model.
func (mdl *Model) Run() (res *Result) {
	res = Success()

	// compute all equations with specified mode
	compute := func(modes string, eqns []*Equation) (res *Result) {
		res = Success()
		for _, eqn := range eqns {
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
	for i, eqn := range mdl.Eqns {
		if strings.Contains("CN", eqn.Mode) {
			split = i + 1
		}
	}
	if mdl.Verbose {
		Msgf("   INFO: Splitting equations: INIT=[1..%d], RUN=[%d..%d]\n", split, split+1, len(mdl.Eqns))
	}
	initEqns := mdl.Eqns[:split]
	runEqns := mdl.Eqns[split:]

	//------------------------------------------------------------------
	// Initialize state:
	//------------------------------------------------------------------
	Msg("   Initializing state...")

	// set predefined (system) variables if not defined
	setDef := func(name string, val Variable) {
		if _, ok := mdl.Current[name]; !ok {
			Msgf("      INFO: Setting '%s' to %f\n", name, val)
			mdl.Current[name] = val
		}
	}
	setDef("TIME", 0)
	setDef("DT", 0.1)
	setDef("LENGTH", 10)
	setDef("PRTPER", 0)
	setDef("PLTPER", 0)

	// initialize from equations
	if res = compute("CNRA", initEqns); !res.Ok {
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
		if level[0] == '_' {
			continue
		}
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
		if mdl.IsSystem(level) {
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
		if res = compute("AR", runEqns); !res.Ok {
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
		if res = compute("L", runEqns); !res.Ok {
			break
		}
		Dbg.Msgf("[%d] %v\n", epoch, mdl.Current)
		// propagate in time
		mdl.Current["TIME"] = mdl.Current["TIME"] + mdl.Current["DT"]
	}
	return
}
