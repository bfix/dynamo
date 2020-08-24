package main

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
	"flag"
	"os"

	"dynamo"
)

// main entry point: call DYNAMO interpreter with given arguments
func main() {
	dynamo.Msg("-----------------------------------")
	dynamo.Msg("DYNAMO interpreter v1.0  (20200728)")
	dynamo.Msg("Copyright (C) 2020, Bernd Fix   >Y<")
	dynamo.Msg("-----------------------------------")

	var (
		debugFile string
		printFile string
		plotFile  string
		verbose   bool
	)
	flag.StringVar(&debugFile, "d", "", "Debug file name (default: none)")
	flag.StringVar(&printFile, "p", "", "Printer file name (default: none)")
	flag.StringVar(&plotFile, "g", "", "Plotter file name (default: none)")
	flag.BoolVar(&verbose, "v", false, "More log messages (default: false)")
	flag.Parse()
	if flag.NArg() != 1 {
		dynamo.Fatal("No DYNAMO source file provided.")
	}

	fname := flag.Arg(0)
	dynamo.Msgf("Reading source file '%s'...\n", fname)
	src, err := os.Open(fname)
	if err != nil {
		dynamo.Fatal(err.Error())
	}
	defer src.Close()

	dynamo.Msg("Processing system model...")
	dynamo.SetDebugger(debugFile)
	mdl := dynamo.NewModel(printFile, plotFile)
	mdl.Verbose = verbose
	if res := mdl.Parse(src); !res.Ok {
		dynamo.Fatalf("Line %d: %s\n", res.Line, res.Err.Error())
	}
	dynamo.Msg("   Model processing completed.")
	mdl.Quit()
	dynamo.Msg("Done.")
}
