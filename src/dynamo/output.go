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
	"log"
	"os"
)

//======================================================================
// Normal program messages
//======================================================================

// Msg (plain message)
func Msg(msg string) {
	log.Println(msg)
}

// Msgf (formatted message)
func Msgf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

// Fatal terminates the application with plain message
func Fatal(msg string) {
	log.Fatal(msg)
}

// Fatalf terminates the application with formatted message
func Fatalf(format string, args ...interface{}) {
	log.Fatalf(format, args...)
}

//======================================================================
// DEBUG messages
//======================================================================

// Shared debugger instance
var Dbg *Debugger

// Debugger writes debug messages to a file (if defined)
type Debugger struct {
	file    *os.File // reference to debug file (or nil if not defined)
	console bool
}

// SetDebugger instantiates a new Debugger
func SetDebugger(file string) {
	Dbg = new(Debugger)
	if len(file) == 0 {
		Dbg.file = nil
	} else {
		if file == "-" {
			Dbg.console = true
			Dbg.file = os.Stdout
		} else {
			var err error
			if Dbg.file, err = os.Create(file); err != nil {
				Fatal(err.Error())
			}
		}
	}
}

// Close debugger file
func (dbg *Debugger) Close() {
	if dbg != nil && dbg.file != nil && !dbg.console {
		dbg.file.Close()
	}
}

// Msg to write a plain message into the debugger file
func (dbg *Debugger) Msg(msg string) {
	if dbg != nil && dbg.file != nil {
		dbg.file.WriteString(msg + "\n")
	}
}

// Msgf to write a formatted message into the debugger file
func (dbg *Debugger) Msgf(format string, args ...interface{}) {
	if dbg != nil && dbg.file != nil {
		msg := fmt.Sprintf(format, args...)
		dbg.file.WriteString(msg)
	}
}

//======================================================================
// PRINTER for DYNAMO results
//======================================================================

// PrtVar for variables used in result lists
type PrtVar struct {
	Name   string
	Column int
}

// Printer writes print output to a file (if defined)
type Printer struct {
	file *os.File  // reference to print file (or nil if not defined)
	mdl  *Model    // back-ref to model instance
	vars []*PrtVar // variables and assigned columns
}

// NewPrinter instantiates a new printer output.
func NewPrinter(file string, mdl *Model) *Printer {
	prt := new(Printer)
	prt.mdl = mdl
	prt.vars = make([]*PrtVar, 1)
	prt.vars[0] = &PrtVar{
		Name:   "TIME",
		Column: 0,
	}
	// open file for output
	if len(file) == 0 {
		prt.file = nil
	} else {
		var err error
		if prt.file, err = os.Create(file); err != nil {
			Fatal(err.Error())
		}
	}
	return prt
}

// Close a printer if job is complete
func (prt *Printer) Close() {
	if prt.file != nil {
		prt.file.Close()
	}
}

// AddVariable adds a var (level or rate) for output at given column
func (prt *Printer) AddVariable(name string, column int) {
	prt.vars = append(prt.vars, &PrtVar{
		Name:   name,
		Column: column,
	})
}

// Start is called when the model starts executing
func (prt *Printer) Start() {
	if prt.file != nil {
		for i, v := range prt.vars {
			if i > 0 {
				prt.file.WriteString(";")
			}
			prt.file.WriteString(fmt.Sprintf("%s", v.Name))
		}
		prt.file.WriteString("\n")
	}
}

// Add a new line for results in this epoch
func (prt *Printer) Add() *Result {
	if prt.file != nil {
		first := true
		for _, v := range prt.vars {
			val, ok := prt.mdl.Current[v.Name]
			if !ok {
				return Failure(ErrModelNoVariable+": %s", v.Name)
			}
			if !first {
				prt.file.WriteString(";")
			}
			prt.file.WriteString(fmt.Sprintf("%f", val))
			first = false
		}
		prt.file.WriteString("\n")
	}
	return Success()
}

//======================================================================
// PLOTTER for DYNAMO graphs
//======================================================================

// PlotVar represents a "plottable" variable (level or rate)
type PlotVar struct {
	Name     string    // variable name
	Sym      rune      // plotting symbol (in ASCII plots)
	Min, Max float64   // plot range
	Values   []float64 // variable values
}

// Plotter to generate graphs from DYNAMO data
type Plotter struct {
	file *os.File   // reference to debug file (or nil if not defined)
	mdl  *Model     // back-ref to model instance
	vars []*PlotVar // variables to use in graphs
}

// NewPlotter instantiates a new plotter output.
func NewPlotter(file string, mdl *Model) *Plotter {
	plt := new(Plotter)
	if len(file) == 0 {
		plt.file = nil
	} else {
		var err error
		if plt.file, err = os.Create(file); err != nil {
			Fatal(err.Error())
		}
	}
	return plt
}

// Close plotter if model run is complete
func (plt *Plotter) Close() {
	if plt.file != nil {
		plt.file.Close()
	}
}

// Add variable to be plotted.
func (plt *Plotter) AddVariable(name string, symbol rune, min, max float64) {
	pv := &PlotVar{
		Name:   name,
		Sym:    symbol,
		Min:    min,
		Max:    max,
		Values: make([]float64, 0),
	}
	plt.vars = append(plt.vars, pv)
}

// Start a new plot
func (plt *Plotter) Start() {
	if plt.file != nil {
		plt.file.WriteString("Not implemented yet.\n")
	}
}

// Add a new set of results in this epoch.
func (plt *Plotter) Add() *Result {
	return Success()
}
