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

//----------------------------------------------------------------------
// EQNLIST -- List of DYNAMO equations that constitute a model
//----------------------------------------------------------------------

// EqnList is a list of equations
type EqnList struct {
	eqns []*Equation
}

// NewEqnList returns an empty equation list.
func NewEqnList() *EqnList {
	return &EqnList{
		eqns: make([]*Equation, 0),
	}
}

// Clone an equation list
func (el *EqnList) Clone() *EqnList {
	out := new(EqnList)
	out.eqns = make([]*Equation, el.Len())
	copy(out.eqns, el.eqns)
	return out
}

// Contains return true if equation (target) is in the list.
func (el *EqnList) Contains(eqn *Equation) bool {
	for _, e := range el.eqns {
		if e.Target.Compare(eqn.Target) == NAME_MATCH {
			return true
		}
	}
	return false
}

// Replace equation in list.
func (el *EqnList) Replace(eqn *Equation) {
	for i, e := range el.eqns {
		if e.Target.Compare(eqn.Target) == NAME_MATCH {
			el.eqns[i] = eqn
			break
		}
	}
}

// Dump logs the current equation list in human-readable form into
// the log stream.
func (el *EqnList) Dump(verbose bool) {

	// count equations by type
	cnt := make(map[string]int)
	incr := func(mode string) {
		v, ok := cnt[mode]
		if !ok {
			v = 0
		}
		cnt[mode] = v + 1
	}
	for _, e := range el.eqns {
		incr(e.Mode)
	}
	Msg("-----------------------------------")
	Msgf("   Number of equations: %4d\n", el.Len())
	Msgf("       LEVEL equations: %4d\n", cnt["L"])
	Msgf("        RATE equations: %4d\n", cnt["R"])
	Msgf("         AUX equations: %4d\n", cnt["A"])
	Msgf("       SUPPL equations: %4d\n", cnt["S"])
	Msgf("       CONST equations: %4d\n", cnt["C"])
	Msgf("        INIT equations: %4d\n", cnt["N"])
	Msg("-----------------------------------")
	if verbose {
		for i, e := range el.eqns {
			Msgf("   %5d: %s\n", i+1, e.String())
			if len(e.Dependencies) > 0 {
				Msgf("          Deps=%v\n", e.Dependencies)
			}
			if len(e.References) > 0 {
				Msgf("          Refs=%v\n", e.References)
			}
		}
	}
}

// Add an equation to the list.
func (el *EqnList) Add(eqn *Equation) {
	el.eqns = append(el.eqns, eqn)
}

// AddList appends an equation list.
func (el *EqnList) AddList(list *EqnList) {
	for _, eqn := range list.eqns {
		el.eqns = append(el.eqns, eqn)
	}
}

// List returns iterable equations.
func (el *EqnList) List() []*Equation {
	return el.eqns
}

// Split equation list into two at given position.
func (el *EqnList) Split(pos int) (*EqnList, *EqnList) {
	lower := &EqnList{
		eqns: el.eqns[:pos],
	}
	upper := &EqnList{
		eqns: el.eqns[pos:],
	}
	return lower, upper
}

// Len returns the length of the equation list
func (el *EqnList) Len() int {
	return len(el.eqns)
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
func (el *EqnList) Sort(mdl *Model) (eqns *EqnList, res *Result) {
	res = Success()
	eqns = NewEqnList()

	// Kahn's algorithm (1962) is used for sorting.
	eqnSort := func(list, ref map[string]*eqnEntry) (out []int, res *Result) {
		res = Success()
		for _, entry := range list {
			eqn := el.eqns[entry.pos]
			for _, d := range eqn.Dependencies {
				// skip system variables
				if mdl.IsSystem(d.Name) {
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
					Dbg.Msgf(ErrModelUnknownEqn+": %s\n", d.Name)
					res = Failure(ErrModelUnknownEqn+": %s", d.Name)
					break
				}
			}
		}
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
				eqn := el.eqns[e.pos]
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
	Dbg.Msgf("SortEquations: Sorting %d equations...\n", el.Len())
	eqnInit := make(map[string]*eqnEntry)
	eqnRun := make(map[string]*eqnEntry)
	for i, eqn := range el.eqns {
		name := eqn.Target.Name
		Dbg.Msgf("SortEquations << [%d] %s\n", i, eqn.String())
		if strings.Index("CN", eqn.Mode) != -1 {
			if _, ok := eqnInit[name]; ok {
				return nil, Failure(ErrModelVariabeExists+": [1] %s", name)
			}
			eqnInit[name] = newEntry(i, name)
		} else if strings.Index("ARLS", eqn.Mode) != -1 {
			if _, ok := eqnRun[name]; ok {
				return nil, Failure(ErrModelVariabeExists+": [2] %s", name)
			}
			eqnRun[name] = newEntry(i, name)
		} else {
			return nil, Failure(ErrModelEqnBadMode)
		}
	}
	// sort both lists
	var listInit, listRun []int
	Dbg.Msg("Sorting eqnInit...")
	if listInit, res = eqnSort(eqnInit, nil); res.Ok {
		Dbg.Msg("Sorting eqnRun...")
		if listRun, res = eqnSort(eqnRun, eqnInit); res.Ok {
			// build re-ordered equation list
			for _, i := range listInit {
				eqns.Add(el.eqns[i])
			}
			for _, i := range listRun {
				eqns.Add(el.eqns[i])
			}
			Dbg.Msgf("SortEquations: Finishing %d equations...\n", el.Len())
			for i, eqn := range eqns.List() {
				Dbg.Msgf("SortEquations >> [%d] %s\n", i, eqn.String())
			}
		}
	}
	return
}

//----------------------------------------------------------------------
// Validate equations
//----------------------------------------------------------------------

// Validate all equations in a list (syntax/semantic)
func (el *EqnList) Validate(mdl *Model) *Result {

	// build list of variable equations
	list := make(map[string]*Equation)
	for _, eqn := range el.eqns {
		name := eqn.Target.String()
		if _, ok := list[name]; ok {
			return Failure(ErrModelEqnAmbigious)
		}
		list[name] = eqn
	}
	// check all equations
	for _, eqn := range el.eqns {
		// check if equation has correct dependencies
		if res := el.validateEqn(mdl, eqn, list); !res.Ok {
			Dbg.Msgf("*** %s\n", eqn.String())
			return res
		}
	}
	return Success()
}

// ValidateEqn checks a single equation for correctness.
func (el *EqnList) validateEqn(mdl *Model, eqn *Equation, list map[string]*Equation) (res *Result) {

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
			&Class{NAME_KIND_SUPPL, NAME_STAGE_NEW},
			[]*Class{
				&Class{NAME_KIND_CONST, NAME_STAGE_NONE}, // constants
				&Class{NAME_KIND_AUX, NAME_STAGE_NEW},    // auxilieries
				&Class{NAME_KIND_LEVEL, NAME_STAGE_NEW},  // levels
				&Class{NAME_KIND_RATE, NAME_STAGE_NEW},   // rates
			})
	}
	return
}
