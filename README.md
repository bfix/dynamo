
dynamo: Interpreter for the DYNAMO programming language
=======================================================

(c) 2020,2021 Bernd Fix <brf@hoi-polloi.org>   >Y<

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
triggered environmental awareness globally. You can find the WORLD3
implementation (as well as its predecessor WORLD2) in the `rt/world` folder.

## The DYNAMO interpreter

This is work-in-progress at a rather early stage. It is capable of running
simple models (see the `rt/` folder for examples), but can fail (or need
adjustment) on anything more complex. If you have some old DYNAMO models and
they don't work with the current version, please let me know (email to
brf@hoi-polloi.org) and I am happy to work on the interpreter to make them
run again.

This interpreter was build as a "clean-room implementation"; it is not based on
any other DYNAMO compiler code but was derived from publically available
documentation (like the _DYNAMO User's Manual_ by Alexander L. Pugh, Alexander
L. Pugh III from 1983) or from looking at the few DYNAMO models that can be
found on the internet.

DYNAMO itself was developed over a course of some 30 years from 1960 to 1990;
so some features were not available in all versions or some language rules were
less strict and only enforced in later versions. This DYNAMO interpreter tries
to follow the most advanced language version as possible, but there are some
limitations that need to be noticed:

* The interpreter does not support macros (MACRO/MEND blocks) yet. The delay
functions normally implemented as macros are hard-coded in the interpreter.

* The DELAYP function is not available. A workaround is to use the DELAY3
function and two additional equations:

```
R   OUT.KL=DELAY3(IN.JK,DEL)
L   INTRN.K=INTRN.J+(DT)(IN.JK-OUT.JK)
N   INTRN=IN*DEL
```

* No interactive edit mode: The interpreter provides a "EDIT" directive to
allow the editing (replacing and adding equations) of a model in the source
code. The examples in `rt/book/` folder make use of this feature; have a look
at the models to understand the use of the edit functionality.

* A print symbol ***** or **#** in the PLOT statement will trigger "point" mode
(instead of "line" mode) in the GNUplot graph.

* `PLTPER` and `PRTPER` can't be variable in the current version of the DYNAMO
interpreter.

### Build the interpreter

At the moment no pre-built binaries of the DYNAMO interpreter are provided; to
build the application, you need a working installation of Go
(https://golang.org/) on your computer; make sure your Go installation is
up-to-date (at least Go1.11; Go1.17 recommended).

In the base directory of this repository issue the following commands:

```bash
# install dependencies
go mod tidy
# build library
go build
# install interpreter
go install github.com/bfix/dynamo/cmd/dynamo
```

### Running a DYNAMO model

Change into the `rt/` (runtime) folder; below that folder you can find sample
DYNAMO models to play around with. For example you can run the epidemic model
with the following command:

```bash
../dynamo -p ~/flu.prt book/flu.dynamo
```

This will run the `flu.dynamo` model and generate a print output in the file
`~/flu.prt`.

The following command line options are available:

* `-v`: verbose output; show more status messages during processing.
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

See the README in the `rt/` folder (and subfolders) for more details on the
example models provided.

## DYNAMO flowchart shapes

To create DYNAMO flowcharts in "Dia" (a GNU/Linux diagrammer), you can install
custom shapes and a corresponding sheet from the `dia/` folder of this
repository. All DYNAMO flowcharts in the `rt/` folder have been done that way.
