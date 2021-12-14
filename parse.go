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
	// compact string (trim and remove double spaces)
	compact := func(s string) string {
		s = strings.TrimSpace(s)
		for strings.Contains(s, "  ") {
			s = strings.Replace(s, "  ", " ", -1)
		}
		return s
	}

	// parse a single (complete) line of model code
	var (
		input  string
		lineNo int
		stmtNo int
	)
	parseInput := func() (res *Result) {
		res = Success()
		// skip empty input line
		if len(input) == 0 {
			return
		}
		// create new statement
		stmt := new(Line)
		// dissect inout
		if pos := strings.Index(input, " "); pos != -1 {
			stmt.Mode = input[:pos]
			input = strings.TrimSpace(input[pos:])
			stmt.Stmt = input
			stmt.Comment = ""
			if strings.Contains("CNARLST", stmt.Mode) {
				if pos := strings.Index(input, " "); pos != -1 {
					stmt.Stmt = input[:pos]
					stmt.Comment = compact(input[pos:])
				}
			}
			res = mdl.AddStatement(stmt).SetLine(stmtNo)
		}
		input = ""
		return
	}

	// parse source stream
	brdr := bufio.NewReader(rdr)
	lineNo = 0
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
				res = parseInput()
			} else {
				res = Failure(err).SetLine(lineNo)
			}
			return
		}
		// process line
		line := strings.ToUpper(string(data))
		if len(line) == 0 {
			// skip empty lines
			continue
		}
		// check for continuation line
		if line[0] == 'X' {
			input += strings.TrimSpace(line[1:])
			continue
		}
		// process pending input
		if res = parseInput(); !res.Ok {
			break
		}
		input = line
		stmtNo = lineNo
	}
	res.SetLine(lineNo)
	return
}
