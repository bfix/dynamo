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
