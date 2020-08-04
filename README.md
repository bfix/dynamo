
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
the Allende presidency (the project died with Allende on 9/11/1973)

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

In the base directory of this repository issue the following command to build
the DYNAMO interpreter:

```bash
GOPATH=$(pwd) go build -o dynamo src/cmd/dynamo/main.go
```

Make sure your Go installation is up-to-date (Go1.11+) and available.

## Running a DYNAMO model

Change into the `rt/` (runtime) folder; here you can find some sample DYNAMO
models to play around with. For example you run the epidemic model with the
following command:

```bash
../dynamo -p flu.prt flu.dynamo
```

This will run the `flu.dynamo` model and generate print output in the file
`flu.prt` (the plotter output is not working yet).

See the README in the `rt/` folder for more details on the example models
provided.
