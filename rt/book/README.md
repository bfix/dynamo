
Introduction to System Dynamics Modeling with DYNAMO
====================================================

The models in this section are taken from the book
**Introduction to System Dynamics Modeling with DYNAMO** by George P.
Richardson and Alexander L. Pugh III, The MIT Press, 1981 (ISBN
0-262-18102-9).

## flu

**Epidemic model for flu**

<table>
<tr>
<td width="30%" valign="top">
  <img src="plots/flu_(1).svg" alt="flu.dynamo graph (simple)" />
  <p>Simple epidemic model (Page 96, Figure 3.7)</p>
</td>
<td width="30%" valign="top">
  <img src="plots/flu_(2).svg" alt="flu.dynamo graph (delay)" />
  <p>Improved epidemic model with incubation (Page 104, Figure 3.4)</p>
</td>
</tr>
<tr>
<td width="30%" valign="top">
  <hr/>
  <img src="plots/flu-model-1.svg" alt="flu model (simple)" width="80%" />
  <p>Model for simple epidemic model (Page 95, Figure 3.6)</p>
</td>
<td width="30%" valign="top">
  <hr/>
  <img src="plots/flu-model-2.svg" alt="flu model (delay)" width="80%" />
  <p>Model for epidemic model with incubation (Page 105, Figure 3.14)</p>
</td>
</tr>
</table>

## inventory

A simple inventory control model (Page 124, Figure 3.25)

This is a base model to show the effects of different test functions (STEP,
RAMP, NOISE,...) in a model run. See the following specific models which
were executed for 100 epochs (101 iterations):

<table>
<tr>
<td width="30%" valign="top">
<hr/>
<b>inv-1.dynamo</b>

Run with `TEST1=1` (Page 125, Figure 3.26) to enable the `STEP` function.

<p align="center">
  <img src="plots/inventory_(1).svg" alt="inv-1.dynamo graph" />
</p>
</td>
<td width="30%" valign="top">
<hr/>
<b>inv-2.dynamo</b>

Run with `TEST2=1` (Page 126, Figure 3.27) to enable the `RAMP` function.

<p align="center">
  <img src="plots/inventory_(2).svg" alt="inv-2.dynamo graph" />
</p>
</td>
</tr>
<tr>
<td width="30%" valign="top">
<hr/>
<b>inv-3.dynamo</b>

Run with `TEST3=1` and `INTVL=200` (Page 128, Figure 3.28) to enable the `PULSE` function.

<p align="center">
  <img src="plots/inventory_(3).svg" alt="inv-3.dynamo graph" />
</p>
</td>
<td width="30%" valign="top">
<hr/>
<b>inv-4.dynamo</b>

Run with `TEST3=1` and `INTVL=5` (Page 129, Figure 3.29) to enable the `PULSE` function.

<p align="center">
  <img src="plots/inventory_(4).svg" alt="inv-4.dynamo graph" />
</p>
</td>
</tr>
<tr>
<td width="30%" valign="top">
<hr/>
<b>inv-5.dynamo</b>

Run with `TEST4=1` and `PER=5` (Page 130, Figure 3.30) to enable the `SIN` function.

<p align="center">
  <img src="plots/inventory_(5).svg" alt="inv-5.dynamo graph" />
</p>
</td>
<td width="30%" valign="top">
<hr/>
<b>inv-6.dynamo</b>

Run with `TEST4=1` and `PER=10` (Page 131, Figure 3.31) to enable the `SIN` function.

<p align="center">
  <img src="plots/inventory_(6).svg" alt="inv-6.dynamo graph" />
</p>
</td>
</tr>
<tr>
<td width="30%" valign="top">
<hr/>
<b>inv-7.dynamo</b>

Run with `TEST5=1` (Page 132, Figure 3.32) to enable the `NOISE` function.

N.B.: The graph is slightly different from the book,as `NOISE` involves
randomness; re-running the model would also generate a different graph.

<p align="center">
  <img src="plots/inventory_(7).svg" alt="inv-7.dynamo graph" />
</p>
</td>
<td/>
</tr>
</table>

## prison

This is a **simple model for prison population** as described on pages
181-185; the model listing can be found on pages 211-213:

<table>
<tr>
<td width="30%" valign="top">
  <img src="plots/prison_(1).svg" alt="prison.dynamo graph (below capacity)" />
  <p>Prison population below capacity (Page 183, Figure 4.22)</p>
</td>
<td width="30%" valign="top">
  <img src="plots/prison_(2).svg" alt="prison.dynamo graph (above capacity)" />
  <p>Prison population above capacity (Page 184, Figure 4.23)</p>
</td>
</tr>
<tr>
<td width="30%" valign="top">
  <img src="plots/prison-model.svg" alt="prison flowchart" width="80%" />
</td>
<td width="30%" valign="top">
</td>
</tr>
</table>

## project

Designing and implementing a **simple project model** is described in the book
on pages 190 to 213; the model code can be found on pages 211 to 213:

<table>
<tr>
<td width="30%" valign="top">
  <img src="plots/project_(1).svg" alt="project.dynamo graph" />
  <p>Base run of the project model (Page 210, Figure 4.32)</p>
</td>
<td width="30%" valign="top">
</td>
</tr>
</table>
