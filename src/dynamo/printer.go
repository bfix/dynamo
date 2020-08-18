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
// PRINTER for DYNAMO results
//======================================================================

//----------------------------------------------------------------------
// PrtVar
//----------------------------------------------------------------------

// PrtVar represents a printed variable
type PrtVar struct {
	TSVar
	Scale float64
}

// NewPrtVar creates a new named variable for print output.
func NewPrtVar(name string) *PrtVar {
	return &PrtVar{
		TSVar: TSVar{
			Name:   name,
			Values: make([]float64, 0),
		},
		Scale: 1.0,
	}
}

// Calculate optimal scale of data series
func (pv *PrtVar) calcScale() {
	x := int(math.Round(math.Log10(math.Max(math.Abs(pv.Max), math.Abs(pv.Min))))) - 2
	if x > 0 {
		pv.Scale = math.Pow10(x)
	}
}

//----------------------------------------------------------------------
// PrtCol
//----------------------------------------------------------------------

// PrtCol has an ordered list of variables to appear in a column
type PrtCol struct {
	Vars  []string
	Scale float64
}

// NewPrtCol instantiates a new column (multi-label)
func NewPrtCol() *PrtCol {
	return &PrtCol{
		Vars:  make([]string, 0),
		Scale: -1.0,
	}
}

// Add a name to the colum
func (pc *PrtCol) Add(name string) *PrtCol {
	pc.Vars = append(pc.Vars, name)
	return pc
}

// Merge scales of sub-columns
func (pc *PrtCol) mergeScale(scale float64) {
	if scale > pc.Scale {
		pc.Scale = scale
	}
}

//----------------------------------------------------------------------
// Printer
//----------------------------------------------------------------------

// Printing modes
const (
	PRT_DYNAMO = iota // Old-style DYNAMO printing
	PRT_CSV           // CSV-formatted print
)

// Printer writes print output to a file (if defined)
type Printer struct {
	file  *os.File           // reference to print file (or nil if not defined)
	mode  int                // printing mode (PRT_????)
	mdl   *Model             // back-ref to model instance
	stmt  string             // PRINT statement
	steps int                // number of DT steps between printed points
	vars  map[string]*PrtVar // variables to use in print
	cols  map[int]*PrtCol    // print columns
	xnum  int                // number of x-values
}

// NewPrinter instantiates a new printer output.
func NewPrinter(file string, mdl *Model) *Printer {
	// determine printing mode from file name
	mode := PRT_DYNAMO
	if pos := strings.LastIndex(file, "."); pos != -1 {
		switch strings.ToUpper(file[pos:]) {
		case ".PRT":
			mode = PRT_DYNAMO
		case ".CSV":
			mode = PRT_CSV
		}
	}
	// create new printer instance
	prt := &Printer{
		mdl:  mdl,
		mode: mode,
		vars: make(map[string]*PrtVar),
		cols: make(map[int]*PrtCol),
	}
	// Add TIME as first column
	prt.vars["TIME"] = NewPrtVar("TIME")
	prt.cols[0] = NewPrtCol().Add("TIME")
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
func (prt *Printer) Close() (res *Result) {
	res = Success()
	if prt.file != nil {
		defer prt.file.Close()
		// we do the actual printing before closing down.
		res = prt.print()
	}
	return
}

// Prepare the printer for output ased on the PRINT statement
func (prt *Printer) Prepare(stmt string) (res *Result) {
	res = Success()
	var err error
	prt.stmt = stmt
	// split into column groups
	grps := strings.Split(stmt, "/")
	if len(grps) == 1 {
		// we only have one column group: flat list of columns
		for pos, label := range strings.Split(grps[0], ",") {
			pv := &PrtVar{
				TSVar: TSVar{
					Name:   label,
					Values: make([]float64, 0),
				},
				Scale: 1.0,
			}
			prt.vars[label] = pv
			prt.cols[pos+1] = NewPrtCol().Add(label)
		}
	} else {
		// parse column groups
		for pos, grp := range grps {
			// parse optional column index.
			col := pos + 1
			if delim := strings.Index(grp, ")"); delim != -1 {
				if col, err = strconv.Atoi(grp[:delim]); err != nil {
					return Failure(err)
				}
				grp = grp[delim+1:]
			}
			// add labels to column
			column := NewPrtCol()
			prt.cols[col] = column
			for _, label := range strings.Split(grp, ",") {
				// add variable
				pv := &PrtVar{
					TSVar: TSVar{
						Name:   label,
						Values: make([]float64, 0),
					},
					Scale: 1.0,
				}
				prt.vars[label] = pv
				// add to column
				column.Add(label)
			}
		}
	}
	return
}

// Start is called when the model starts executing
func (prt *Printer) Start() (res *Result) {
	res = Success()
	if prt.file != nil {
		// get print stepping
		pp, ok := prt.mdl.Current["PRTPER"]
		if !ok {
			return Failure(ErrModelMissingDef + ": PRTPER")
		}
		dt, ok := prt.mdl.Current["DT"]
		if !ok {
			return Failure(ErrModelMissingDef + ": DT")
		}
		prt.steps = int(pp / dt)
		if compare(float64(pp), float64(prt.steps)*float64(dt)) != 0 {
			Msgf("WARNING: PRTPER != n * DT")
		}
	}
	return
}

// Add a new line for results in this epoch
func (prt *Printer) Add(epoch int) (res *Result) {
	res = Success()
	if prt.file != nil {
		// check for output epoch
		if prt.steps > 1 && epoch%prt.steps != 1 {
			return
		}
		// get values for printed variables
		for name, pv := range prt.vars {
			val, ok := prt.mdl.Current[name]
			if !ok {
				return Failure(ErrModelNoVariable+": %s", name)
			}
			pv.Add(float64(val))
		}
		prt.xnum++
	}
	return
}

//----------------------------------------------------------------------
// Print routines
//----------------------------------------------------------------------

// Print collected data
func (prt *Printer) print() (res *Result) {
	res = Success()

	Msgf("   Generating print...")
	switch prt.mode {
	case PRT_DYNAMO:
		res = prt.print_dyn()
	case PRT_CSV:
		res = prt.print_csv()
	default:
		res = Failure(ErrPrintMode)
	}
	return
}

// Print data in classic DYNAMO style
func (prt *Printer) print_dyn() (res *Result) {
	res = Success()

	// print intro
	if len(prt.mdl.Title) > 0 {
		fmt.Fprintf(prt.file, "***** %s *****\n", prt.mdl.Title)
		fmt.Fprintln(prt.file)
	}
	if len(prt.mdl.RunID) > 0 {
		fmt.Fprintf(prt.file, "Print results for run '%s'\n", prt.mdl.RunID)
		fmt.Fprintln(prt.file)
	}
	// compute optimal scale for printed variables
	for _, pv := range prt.vars {
		pv.calcScale()
	}
	// assemble array of columns with sub-columns (in print order)
	list := make([][]string, 20)
	maxcol := 0
	maxsub := 0
	for col := 0; col < 20; col++ {
		list[col] = nil
		if pc, ok := prt.cols[col]; ok {
			list[col] = pc.Vars
			maxcol = col + 1
			for _, name := range pc.Vars {
				pc.mergeScale(prt.vars[name].Scale)
			}
			if len(pc.Vars) > maxsub {
				maxsub = len(pc.Vars)
			}
		}
	}
	// print header
	for sub := 0; sub < maxsub; sub++ {
		for col := 0; col < maxcol; col++ {
			vl := list[col]
			if vl == nil || sub >= len(vl) {
				fmt.Fprintf(prt.file, "         ")
			} else {
				fmt.Fprintf(prt.file, "  %7s", vl[sub])
			}
		}
		fmt.Fprintln(prt.file)
	}
	// print scales
	for col := 0; col < maxcol; col++ {
		vl := list[col]
		if vl == nil {
			fmt.Fprintf(prt.file, "         ")
		} else {
			scale := fmt.Sprintf("%E", prt.cols[col].Scale)
			pos := strings.LastIndex(scale, "E")
			fmt.Fprintf(prt.file, "  %7s", scale[pos:])
		}
	}
	fmt.Fprintln(prt.file)
	// print data
	for x := 0; x < prt.xnum; x++ {
		for sub := 0; sub < maxsub; sub++ {
			for col := 0; col < maxcol; col++ {
				vl := list[col]
				if vl == nil || sub >= len(vl) {
					fmt.Fprintf(prt.file, "         ")
				} else {
					val := prt.vars[vl[sub]].Values[x] / prt.cols[col].Scale
					fmt.Fprintf(prt.file, "  %7.3f", val)
				}
			}
			fmt.Fprintln(prt.file)
		}
	}
	return
}

// Print data into a CSV file
func (prt *Printer) print_csv() (res *Result) {
	res = Success()

	// get (flat) list of labels
	var list []string
	for col := 0; col < 20; col++ {
		if pc, ok := prt.cols[col]; ok {
			for _, name := range pc.Vars {
				list = append(list, name)
			}
		}
	}
	// emit header
	for i, name := range list {
		if i > 0 {
			prt.file.WriteString(";")
		}
		prt.file.WriteString(name)
	}
	fmt.Println(prt.file)
	// emit data
	for x := 0; x < prt.xnum; x++ {
		for i, name := range list {
			if i > 0 {
				prt.file.WriteString(";")
			}
			pv, ok := prt.vars[name]
			if !ok {
				return Failure(ErrPrintNoVar)
			}
			fmt.Fprintf(prt.file, "%f", pv.Values[x])
		}
		fmt.Println(prt.file)
	}

	return
}

//----------------------------------------------------------------------
// Helper methods
//----------------------------------------------------------------------

var (
	// scale names
	SCALE = []rune{' ', 'K', 'M', 'G', 'T'}
)

/// FormatNumber  a number in short form with scale
func FormatNumber(x float64, frmt string) string {
	i, v := 0, x
	for math.Abs(v) > 1000 {
		i++
		v /= 1000
	}
	return fmt.Sprintf(frmt, v, SCALE[i])
}
