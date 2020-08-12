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
	Name     string    // variable name
	Sym      rune      // plotting symbol (in ASCII plots)
	Min, Max float64   // plot range
	Values   []float64 // variable values
}

// Add a PlotVar value
func (pv *PlotVar) Add(y float64) {
	if len(pv.Values) == 0 {
		pv.Min = y
		pv.Max = y
	} else if y < pv.Min {
		pv.Min = y
	} else if y > pv.Max {
		pv.Max = y
	}
	pv.Values = append(pv.Values, y)
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

const (
	// PLT_OVERLAP: y-range overlap for merging vars into single scale
	PLT_OVERLAP = 0.65
)

// Merge plot ranges if applicable (or forced). Returns true if merged.
func (pr *PlotGroup) Merge(pv *PlotVar, forced bool) bool {
	// check for "no intersection"
	width := math.Min(pr.Max, pv.Max) - math.Max(pr.Min, pv.Min)
	if forced || (width > 0 && width/(pr.Max-pr.Min) > PLT_OVERLAP && width/(pv.Max-pv.Min) > PLT_OVERLAP) {
		// we can merge the ranges
		pr.Min = math.Min(pr.Min, pv.Min)
		pr.Max = math.Max(pr.Max, pv.Max)
		pr.Vars = append(pr.Vars, pv.Name)
		return true
	}
	return false
}

//----------------------------------------------------------------------
// Plotter
//----------------------------------------------------------------------

// Plotter to generate graphs from DYNAMO data
type Plotter struct {
	file  *os.File            // reference to debug file (or nil if not defined)
	mdl   *Model              // back-ref to model instance
	stmt  string              // PLOT statement
	steps int                 // number of DT steps between plotting points
	vars  map[string]*PlotVar // variables to use in graphs
	grps  []*PlotGroup        // plot ranges
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
	plt.grps = make([]*PlotGroup, 0)
	return plt
}

// Close plotter if model run is complete
func (plt *Plotter) Close() (res *Result) {
	res = Success()
	if plt.file != nil {
		defer plt.file.Close()
		// we do the actual plotting before closing down.
		res = plt.plot()
	}
	return
}

// Prepare a plot output
func (plt *Plotter) Prepare(stmt string) (res *Result) {
	res = Success()
	var err error
	plt.stmt = stmt
	// split into groups with same scale first
	for _, grp := range strings.Split(stmt, "/") {
		// each group is a PlotGroup instance
		pg := NewPlotGroup()
		plt.grps = append(plt.grps, pg)
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
				res = Failure(ErrParseSyntax)
				return
			}
			pv := &PlotVar{
				Name:   x[0],
				Sym:    []rune(x[1])[0],
				Min:    0,
				Max:    0,
				Values: make([]float64, 0),
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
			pv.Add(float64(val))
		}
	}
	return
}

var (
	LOG_FACTOR = []float64{0.5, 1, 2, 5, 10}
)

// Plot the collected data
func (plt *Plotter) plot() (res *Result) {
	res = Success()

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
	// get actual range for each plot group (if not defined in PLOT statement)
	for _, grp := range plt.grps {
		if !grp.ValidRange {
			for _, name := range grp.Vars {
				pv, ok := plt.vars[name]
				if !ok {
					return Failure(ErrPlotNoVar+": %s", name)
				}
				grp.Min = math.Min(grp.Min, pv.Min)
				grp.Max = math.Max(grp.Max, pv.Max)
			}
			grp.ValidRange = true
		}
		// compute plot/segment width (plot.width = 4 * seg.width)
		w := 4 * calib((grp.Max-grp.Min)/4, 1)
		ymin := math.Copysign(calib(math.Abs(grp.Min), -1), grp.Min)
		ymax := math.Copysign(calib(math.Abs(grp.Max), 1), grp.Max)
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
	// now do the actual "plotting": the first DYNAMO implementations used
	// ASCII-based plots on long sheets of paper; this one creates SVG graphs
	// of the data, but with a similar "approach" like the old plot routines.

	return
}
