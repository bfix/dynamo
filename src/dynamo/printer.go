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
	"strings"
)

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
