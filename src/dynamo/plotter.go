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
	"math"
	"os"
	"strconv"
	"strings"
)

//======================================================================
// PLOTTER for DYNAMO graphs
//======================================================================

//----------------------------------------------------------------------
// PlotVar
//----------------------------------------------------------------------

// PlotVar is a (time series) variable to be plotted (level or rate)
type PlotVar struct {
	TSVar
	Sym rune // plotting symbol (in ASCII plots)
}

//----------------------------------------------------------------------
// PlotGroup
//----------------------------------------------------------------------

// PlotGroup combines different variables into a single scale
type PlotGroup struct {
	Min, Max   float64  // plot range
	ValidRange bool     // is plot range valid?
	Vars       []string // list of vars in this range
}

// NewPlotGroup creates a new (empty) plot group
func NewPlotGroup() *PlotGroup {
	// create a new plot range instance
	return &PlotGroup{
		Min:        0,
		Max:        0,
		ValidRange: false,
		Vars:       make([]string, 0),
	}
}

// Norm returns the position of the y-value on the axis [0,1]
func (pg *PlotGroup) Norm(y float64) float64 {
	return (y - pg.Min) / (pg.Max - pg.Min)
}

//----------------------------------------------------------------------
// Plot jobs
//----------------------------------------------------------------------

// PlotJob is plottig a graph of selected variables
type PlotJob struct {
	stmt string       // PLOT statement
	plt  *Plotter     // plotter instance
	grps []*PlotGroup // plot ranges
}

// NewPlotJob creates a new plott job for the plotter based on
// the PLOT statement.
func NewPlotJob(stmt string, plt *Plotter) *PlotJob {
	pj := &PlotJob{
		stmt: stmt,
		plt:  plt,
		grps: make([]*PlotGroup, 0),
	}
	return pj
}

//----------------------------------------------------------------------
// Plotter
//----------------------------------------------------------------------

// Plotting modes
const (
	PLT_DYNAMO = iota // Old-style DYNAMO plotting mode (ASCII)
)

// Plotter to generate graphs from DYNAMO data
type Plotter struct {
	file  *os.File            // reference to debug file (or nil if not defined)
	mode  int                 // plotting mode (PLT_????)
	mdl   *Model              // back-ref to model instance
	steps int                 // number of DT steps between plotting points
	vars  map[string]*PlotVar // variables to use in graphs
	x0    float64             // first x position
	dx    float64             // x-step
	xnum  int                 // number of x-values
	jobs  []*PlotJob          // list of plot jobs to perform
	add   bool                // plotter is adding jobs
}

// NewPlotter instantiates a new plotter output.
func NewPlotter(file string, mdl *Model) *Plotter {
	// determine plotting mode from file name
	mode := PLT_DYNAMO
	if pos := strings.LastIndex(file, "."); pos != -1 {
		switch strings.ToUpper(file[pos:]) {
		case ".PRT":
			mode = PLT_DYNAMO
		}
	}
	// create new plotting instance
	plt := &Plotter{
		mdl:  mdl,
		mode: mode,
		vars: make(map[string]*PlotVar),
		xnum: 0,
		jobs: make([]*PlotJob, 0),
		add:  true,
	}
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

// Reset a plotter
func (plt *Plotter) Reset() {
	// clear time-series on PltVar
	for _, v := range plt.vars {
		v.Reset()
	}
	plt.add = false
}

// Generate plot output.
func (plt *Plotter) Generate() *Result {
	if plt.file != nil {
		// do the actual plotting
		return plt.plot()
	}
	return Success()
}

// Close plotter if model run is complete
func (plt *Plotter) Close() (res *Result) {
	res = Success()
	if plt.file != nil {
		if err := plt.file.Close(); err != nil {
			res = Failure(err)
		}
	}
	return
}

// Prepare a plot output job
func (plt *Plotter) Prepare(stmt string) (res *Result) {
	res = Success()

	// if we do not add jobs, clear exisiting jobs and vars
	if !plt.add {
		plt.vars = make(map[string]*PlotVar)
		plt.jobs = make([]*PlotJob, 0)
		plt.add = true
	}

	// create a new print job
	pj := NewPlotJob(stmt, plt)
	plt.jobs = append(plt.jobs, pj)

	// split into groups with same scale first
	var err error
	for _, grp := range strings.Split(stmt, "/") {
		// each group is a PlotGroup instance
		pg := NewPlotGroup()
		pj.grps = append(pj.grps, pg)
		// get scale for group
		if pos := strings.Index(grp, "("); pos != -1 {
			scale := strings.Split(strings.Trim(grp[pos:], "()"), ",")
			if pg.Min, err = strconv.ParseFloat(scale[0], 64); err != nil {
				return Failure(ErrParseNotANumber+": '%s'", scale[0])
			}
			if pg.Max, err = strconv.ParseFloat(scale[1], 64); err != nil {
				return Failure(ErrParseNotANumber+": '%s'", scale[1])
			}
			grp = grp[:pos]
			// plot range in group instance is valid
			pg.ValidRange = true
		}
		// get members of group
		for _, def := range strings.Split(grp, ",") {
			x := strings.Split(def, "=")
			if len(x) != 2 {
				res = Failure(ErrParseSyntax+": '%s'", def)
				return
			}
			pv := &PlotVar{
				TSVar: TSVar{
					Name:   x[0],
					Values: make([]float64, 0),
				},
				Sym: []rune(x[1])[0],
			}
			plt.vars[x[0]] = pv
			// add member to group
			pg.Vars = append(pg.Vars, x[0])
		}
	}
	return
}

// Start a new plot
func (plt *Plotter) Start() (res *Result) {
	res = Success()
	if plt.file != nil {
		// get plot stepping
		x0, ok := plt.mdl.Current["TIME"]
		if !ok {
			return Failure(ErrModelMissingDef + ": TIME")
		}
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
		plt.x0 = float64(x0)
		plt.dx = float64(pp)
		plt.steps = steps
	}
	return
}

// Add a new set of results in this epoch.
func (plt *Plotter) Add(epoch int) (res *Result) {
	res = Success()
	if plt.file != nil {
		// check for output epoch
		if plt.steps > 1 && epoch%plt.steps != 1 {
			return
		}
		// get values for graphed variables
		for name, pv := range plt.vars {
			val, ok := plt.mdl.Current[name]
			if !ok {
				return Failure(ErrModelNoVariable+": %s", name)
			}
			pv.Add(float64(val))
		}
		plt.xnum++
	}
	return
}

var (
	// LOG_FACTOR for range bounds (equidistant in log scale)
	LOG_FACTOR = []float64{0.5, 1, 2, 5, 10}
)

// Plot the collected data
func (plt *Plotter) plot() (res *Result) {
	res = Success()

	// calibrate ranges
	calib := func(y float64, side int) float64 {
		yl := math.Log10(math.Abs(y))
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
		if side < 0 && y > 0 {
			yk = yk - 1
		}
		return LOG_FACTOR[yk] * math.Pow10(int(yb))
	}

	Msgf("      Generating plot(s)...")
	for _, pj := range plt.jobs {
		// get actual range for each plot group (if not defined in PLOT statement)
		for _, grp := range pj.grps {
			if grp.ValidRange {
				continue
			}
			for _, name := range grp.Vars {
				pv, ok := plt.vars[name]
				if !ok {
					return Failure(ErrPlotNoVar+": %s", name)
				}
				grp.Min = math.Min(grp.Min, pv.Min)
				grp.Max = math.Max(grp.Max, pv.Max)
			}
			grp.ValidRange = true

			// compute plot/segment width (plot.width = 4 * seg.width)
			w := 4 * calib((grp.Max-grp.Min)/4, 1)
			ymin := math.Copysign(calib(grp.Min, -1), grp.Min)
			ymax := math.Copysign(calib(grp.Max, 1), grp.Max)
			// adjust boundaries to width; use bound with smaller error
			if grp.Max < ymin+w {
				grp.Min = ymin
				grp.Max = ymin + w
			} else if grp.Min < ymax-w {
				grp.Max = ymax
				grp.Min = ymax - w
			} else {
				res = Failure(ErrPlotRange)
			}
		}
		// now do the actual plotting
		switch plt.mode {
		case PLT_DYNAMO:
			res = plt.plot_dyn(pj)
		default:
			res = Failure(ErrPlotMode)
		}
	}
	return
}

//----------------------------------------------------------------------
// Plot routines
//----------------------------------------------------------------------

func (plt *Plotter) plot_dyn(pj *PlotJob) *Result {

	// make horizontal plot line without graph
	mkLine := func(x float64, i int) string {
		line := make([]byte, 102)
		for j := range line {
			line[j] = ' '
			if i%10 == 0 {
				if j%2 == 0 {
					line[j] = '-'
				}
			} else {
				if j%25 == 0 {
					line[j] = '.'
				}
			}
		}
		if i%10 == 0 {
			return fmt.Sprintf("%10.3f%s", x, line)
		}
		return fmt.Sprintf("          %s", line)
	}

	// emit plot header
	fmt.Fprintf(plt.file, "\n\n")
	fmt.Fprintf(plt.file, "Plot for '%s'\n", plt.mdl.RunID)
	fmt.Fprintf(plt.file, "         %s\n", pj.stmt)
	fmt.Fprintln(plt.file)

	// emit plot y-axis (multiple scales; one per plot group)
	for _, grp := range pj.grps {
		s := ""
		for _, v := range grp.Vars {
			pv := plt.vars[v]
			if len(s) > 0 {
				s += ","
			}
			s += fmt.Sprintf("%s=%c", pv.Name, pv.Sym)
		}
		w := (grp.Max - grp.Min) / 4.
		y0 := FormatNumber(grp.Min, 4, 3)
		y1 := FormatNumber(grp.Min+w, 4, 3)
		y2 := FormatNumber(grp.Min+2*w, 4, 3)
		y3 := FormatNumber(grp.Min+3*w, 4, 3)
		y4 := FormatNumber(grp.Max, 4, 3)
		fmt.Fprintf(plt.file, "%14s%25s%25s%25s%25s %s\n", y0, y1, y2, y3, y4, s)
	}
	// draw graph
	for x, i := plt.x0, 0; i < plt.xnum; x, i = x+plt.dx, i+1 {
		line := []rune(mkLine(x, i))
		for _, grp := range pj.grps {
			for _, v := range grp.Vars {
				pv := plt.vars[v]
				pos := int(math.Round(100*grp.Norm(pv.Values[i]))) + 11
				if pos < 11 || pos > 111 {
					Msgf("y=%f, range=(%f,%f)\n", pv.Values[i], grp.Min, grp.Max)
				}
				line[pos] = pv.Sym
			}
		}
		fmt.Fprintln(plt.file, string(line))
	}
	return Success()
}
