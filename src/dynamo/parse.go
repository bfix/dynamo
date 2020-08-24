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
	"fmt"
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

// Line represents a line in a DYNAMO source code stream. It consists of a
// mode, a statement and an optional comment
type Line struct {
	Mode    string
	Stmt    string
	Comment string
}

// String returns a human-readable representatun of a line
func (l *Line) String() string {
	return fmt.Sprintf("[%s] %s {%s}", l.Mode, l.Stmt, l.Comment)
}

// Parse a DYNAMO source file and return a model instance for it.
func (mdl *Model) Parse(rdr io.Reader) (res *Result) {
	res = Success()
	// parse source stream
	brdr := bufio.NewReader(rdr)
	var (
		stmt    *Line
		stmtNo  int
		mode    string
		comment string
	)
	lineNo := 0
	for {
		// read next line and check length limit
		data, _, err := brdr.ReadLine()
		lineNo++
		if strict && len(data) > MAX_LINE_LENGTH {
			res = Failure(ErrParseLineLength).SetLine(lineNo)
			return
		}
		// handle read error
		if err != nil {
			if err == io.EOF {
				// add last pending statement
				res = mdl.AddStatement(stmt).SetLine(stmtNo)
			} else {
				res = Failure(err).SetLine(lineNo)
			}
			return
		}
		// process line
		line := strings.ToUpper(string(data))
		if pos := strings.Index(line, " "); pos != -1 {
			mode = line[:pos]
			line = strings.TrimSpace(line[pos:])
			if strings.Index("LRCNA", mode) != -1 {
				if pos := strings.Index(line, " "); pos != -1 {
					comment = strings.TrimSpace(line[pos:])
					line = line[:pos]
				}
			}
		} else {
			mode = line
			line = ""
		}
		// extension line (continuation)?
		if mode == "X" {
			stmt.Stmt += line
			if len(comment) > 0 {
				stmt.Comment += " " + comment
			}
			continue
		}
		// process pending statement
		if res = mdl.AddStatement(stmt); !res.Ok {
			res.SetLine(stmtNo)
			break
		}
		stmt = new(Line)
		stmt.Mode = mode
		stmt.Stmt = line
		stmt.Comment = comment
		stmtNo = lineNo
	}
	res.SetLine(lineNo)
	return
}
