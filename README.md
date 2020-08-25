
dynamo: Interpreter for the DYNAMO programming language
=======================================================

(c) 2020 Bernd Fix <brf@hoi-polloi.org>   >Y<

Dynamo is free software: you can redistribute it and/or modify it
under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License,
or (at your option) any later version.

Dynamo is distributed in the hope that it will be useful, but
WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.

SPDX-License-Identifier: AGPL3.0-or-later

## About DYNAMO

DYNAMO (see https://en.wikipedia.org/wiki/DYNAMO_(programming_language) for
details) is a rather ancient system dynamics modelling language that was used
some 50 years ago in various areas; among others:

* Project CYBERSYN (https://en.wikipedia.org/wiki/Project_Cybersyn) used
DYNAMO for the CHECO part of CYBERSTRIDE to simulate the CHilean ECOnomy during
the Allende presidency (the project died with Allende on September
11<sup>th</sup>, 1973. You can find a snapshot of the CHECO implementation from
September 1972 in the `rt/checo` folder, along with charts and graphs.

* "THE LIMITS TO GROWTH" (https://en.wikipedia.org/wiki/The_Limits_to_Growth)
used DYNAMO to simulate global developments leading to the famous "Club of
Rome" (https://en.wikipedia.org/wiki/Club_of_Rome) publication in 1972 that
triggered environmental awareness globally.

## Caveats

This is work-in-progress at a rather early stage. It is capable of running
simple models (see the `rt/` folder for examples), but will most likely fail
on anything more complex. If you have some old DYNAMO models and they don't
work with the current version, please let me know (email to brf@hoi-polloi.org)
and I am happy to work on the interpreter to make them run again.

## Build the interpreter

At the moment no pre-built binaries of the DYNAMO interpreter are provided; to
build the application, you need a working installation of Go
(https://golang.org/) on your computer; make sure your Go installation is
up-to-date (at least Go1.11).

In the base directory of this repository issue the following command:

```bash
GOPATH=$(pwd) go build -o dynamo src/cmd/dynamo/main.go
```

## Running a DYNAMO model

Change into the `rt/` (runtime) folder; below that folder you can find sample
DYNAMO models to play around with. For example you can run the epidemic model
with the following command:

```bash
../dynamo -p ~/flu.prt book/flu/flu.dynamo
```

This will run the `flu.dynamo` model from the specified subfolder and generate
print output in the file `~/flu.prt`.

The following options are available:

* `-d <debug-file>`: write debug output to specified file. Use `-` to log to
console.
* `-p <print-file>`: write printer output to file: the extension used in the
filename specifies which print format to use:
    * `.prt`: Generate classic DYNAMO print output (line printer)
    * `.csv`: Generate CSV-compatible files (e.g. for import into other apps)
* `-g <plot-file>`: write plot output to file: the extension used in the
filename specifies whicht plot format to use:
    * `.plt`: Generate classic DYNAMO plot output (line printer)
    * `.gnuplot`: Generate GNUplot script (SVG generator)

See the README in the `rt/` folder and subfolders for more details on the
example models provided.
