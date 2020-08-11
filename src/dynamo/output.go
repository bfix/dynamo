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
	"math"
	"os"
	"strconv"
	"strings"
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
func (dbg *Debugger) Close() *Result {
	if dbg != nil && dbg.file != nil && !dbg.console {
		dbg.file.Close()
	}
	return Success()
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
func (prt *Printer) Close() *Result {
	if prt.file != nil {
		prt.file.Close()
	}
	return Success()
}

// Prepare the printer for output ased on the PRINT statement
func (prt *Printer) Prepare(stmt string) *Result {
	// parse printer statement  (simplified)
	for i, level := range strings.Split(strings.Replace(stmt, "/", ",", -1), ",") {
		prt.vars = append(prt.vars, &PrtVar{
			Name:   level,
			Column: i + 1,
		})
	}
	return Success()
}

// Start is called when the model starts executing
func (prt *Printer) Start() (res *Result) {
	res = Success()
	if prt.file != nil {
		for i, v := range prt.vars {
			if i > 0 {
				prt.file.WriteString(";")
			}
			prt.file.WriteString(fmt.Sprintf("%s", v.Name))
		}
		prt.file.WriteString("\n")
	}
	return
}

// Add a new line for results in this epoch
func (prt *Printer) Add(epoch int) *Result {
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

const (
	PLT_OVERLAP = 0.65
)

var (
	LOG_FACTOR = []float64{0.5, 1, 2, 5, 10}
)

// PlotVar represents a "plottable" variable (level or rate)
type PlotVar struct {
	Name     string    // variable name
	Sym      rune      // plotting symbol (in ASCII plots)
	Min, Max float64   // plot range
	Values   []float64 // variable values
}

// PlotRange combines different variables into a single scale
type PlotRange struct {
	Min, Max float64  // plot range
	Vars     []string // list of vars in this range
}

// NewPlotRange creates a new plot range from a plot variable
func NewPlotRange(pv *PlotVar) *PlotRange {
	// create a new plot range instance
	return &PlotRange{
		Min:  pv.Min,
		Max:  pv.Max,
		Vars: []string{pv.Name},
	}
}

// Merge plot ranges if applicable. Returns true if merged,
func (pr *PlotRange) Merge(pv *PlotVar) bool {
	// check for "no intersection"
	width := math.Min(pr.Max, pv.Max) - math.Max(pr.Min, pv.Min)
	if width > 0 {
		// compute intersection interval
		if width/(pr.Max-pr.Min) > PLT_OVERLAP && width/(pv.Max-pv.Min) > PLT_OVERLAP {
			// we can merge the ranges
			pr.Min = math.Min(pr.Min, pv.Min)
			pr.Max = math.Max(pr.Max, pv.Max)
			pr.Vars = append(pr.Vars, pv.Name)
			return true
		}
	}
	return false
}

// Plotter to generate graphs from DYNAMO data
type Plotter struct {
	file  *os.File            // reference to debug file (or nil if not defined)
	mdl   *Model              // back-ref to model instance
	steps int                 // number of DT steps between plotting points
	vars  map[string]*PlotVar // variables to use in graphs
	rngs  []*PlotRange        // plot ranges
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
	plt.mdl = mdl
	plt.vars = make(map[string]*PlotVar)
	return plt
}

// Close plotter if model run is complete
func (plt *Plotter) Close() (res *Result) {
	res = Success()
	if plt.file != nil {
		defer plt.file.Close()
		res = plt.plot()
	}
	return
}

// Prepare a plot output
func (plt *Plotter) Prepare(stmt string) (res *Result) {
	res = Success()
	var err error
	// split into groups with same scale first
	for _, grp := range strings.Split(stmt, "/") {
		// get scale for group
		min, max := -1., -1.
		if pos := strings.Index(grp, "("); pos != -1 {
			scale := strings.Split(strings.Trim(grp[pos:], "()"), ",")
			if min, err = strconv.ParseFloat(scale[0], 64); err != nil {
				return Failure(ErrParseNotANumber+": '%s'", scale[0])
			}
			if max, err = strconv.ParseFloat(scale[1], 64); err != nil {
				return Failure(ErrParseNotANumber+": '%s'", scale[1])
			}
			grp = grp[:pos]
		}
		// get members of group
		for _, def := range strings.Split(grp, ",") {
			x := strings.Split(def, "=")
			if len(x) != 2 {
				res = Failure(ErrParseSyntax)
				return
			}
			pv := &PlotVar{
				Name:   x[0],
				Sym:    []rune(x[1])[0],
				Min:    min,
				Max:    max,
				Values: make([]float64, 0),
			}
			plt.vars[x[0]] = pv
		}
	}
	return
}

// Start a new plot
func (plt *Plotter) Start() (res *Result) {
	res = Success()
	if plt.file != nil {
		// get plot stepping
		pp, ok := plt.mdl.Current["PLTPER"]
		if !ok {
			return Failure(ErrModelMissingDef + ": PLTPER")
		}
		dt, ok := plt.mdl.Current["DT"]
		if !ok {
			return Failure(ErrModelMissingDef + ": DT")
		}
		steps := int(pp / dt)
		if compare(float64(pp), float64(steps)*float64(dt)) != 0 {
			Msgf("WARNING: PLTPER != n * DT")
		}
		plt.steps = steps
		// print graph heading
	}
	return
}

// Add a new set of results in this epoch.
func (plt *Plotter) Add(epoch int) (res *Result) {
	res = Success()
	if plt.file != nil {
		// check for output epoch
		if epoch%plt.steps != 1 {
			return
		}
		// get values for graphed variables
		for name, pv := range plt.vars {
			val, ok := plt.mdl.Current[name]
			if !ok {
				return Failure(ErrModelNoVariable+": %s", name)
			}
			pv.Values = append(pv.Values, float64(val))
		}
	}
	return
}

// Plot the collected data
func (plt *Plotter) plot() (res *Result) {
	res = Success()

	// get actual range for each variable (if not defined in PLOT statement)
	for _, pv := range plt.vars {
		if pv.Min < 0 || pv.Max < 0 {
			if len(pv.Values) > 0 {
				pv.Min = pv.Values[0]
				pv.Max = pv.Min
				for _, val := range pv.Values {
					if val < pv.Min {
						pv.Min = val
					}
					if val > pv.Max {
						pv.Max = val
					}
				}
			} else {
				return Failure(ErrModelNoData)
			}
		}
	}
loop:
	// try to merge variables into existing plot ranges.
	for _, pv := range plt.vars {
		for _, pr := range plt.rngs {
			if pr.Merge(pv) {
				continue loop
			}
		}
		// no merge: add new range
		plt.rngs = append(plt.rngs, NewPlotRange(pv))
	}
	// calibrate ranges
	calib := func(y float64, side int) float64 {
		yl := math.Log10(y)
		yb := math.Floor(yl)
		yf := (yl - yb)
		yk := 1
		switch {
		case yf > 0.699:
			yk = 4
		case yf > 0.301:
			yk = 3
		case yf > 0:
			yk = 2
		}
		if side < 0 {
			yk = yk - 1
		}
		return LOG_FACTOR[yk] * math.Pow10(int(yb))
	}
	for _, pr := range plt.rngs {
		// compute plot/segment width (plot.width = 4 * seg.width)
		w := 4 * calib((pr.Max-pr.Min)/4, 1)
		ymin := math.Copysign(calib(math.Abs(pr.Min), -1), pr.Min)
		ymax := math.Copysign(calib(math.Abs(pr.Max), 1), pr.Max)
		// adjust boundaries to width; use bound with smaller error
		if pr.Max < ymin+w {
			pr.Min = ymin
			pr.Max = ymin + w
		} else if pr.Min < ymax-w {
			pr.Max = ymax
			pr.Min = ymax - w
		} else {
			res = Failure(ErrPlotRange)
		}
	}
	// now do the actual "plotting": the first DYNAMO implementations used
	// ASCII-based plots on long sheets of paper; this one creates SVG graphs
	// of the data, but with a similar "approach" like the old plot routines.

	return
}
