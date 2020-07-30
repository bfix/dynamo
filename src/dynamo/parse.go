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
	"bufio"
	"io"
	"strings"
)

//----------------------------------------------------------------------
// Parser for DYNAMO-formatted source files
//----------------------------------------------------------------------

// Parser-related constants
const (
	MAX_LINE_LENGTH = 72 // max. length of line in 'strict' mode
)

// Parse a DYNAMO source file and return a model instance for it.
func (mdl *Model) Parse(rdr io.Reader) *Result {

	// parse source file
	brdr := bufio.NewReader(rdr)
	var (
		stmtLine string
		stmtMode string
		stmtNo   int
		mode     string
	)
	lineNo := 0
	for {
		// read next line and check length limit
		data, _, err := brdr.ReadLine()
		lineNo++
		if strict && len(data) > MAX_LINE_LENGTH {
			return Failure(ErrParseLineLength).SetLine(lineNo)
		}
		// handle read error
		if err != nil {
			if err == io.EOF {
				// add last pending statement
				return mdl.AddStatement(stmtLine, stmtMode).SetLine(stmtNo)
			}
			return Failure(err).SetLine(stmtNo)
		}
		// process line
		line := strings.ToUpper(string(data))
		if pos := strings.Index(line, " "); pos != -1 {
			mode = line[:pos]
			line = strings.TrimSpace(line[pos:])
		} else {
			mode = line
			line = ""
		}
		// extension line (continuation)?
		if mode == "X" {
			stmtLine += line
			continue
		}
		// process pending statement
		if res := mdl.AddStatement(stmtLine, stmtMode); !res.Ok {
			return res.SetLine(stmtNo)
		}
		stmtMode = mode
		stmtLine = line
		stmtNo = lineNo
	}
	return Success().SetLine(lineNo)
}
