// -*- html -*-
package main

import (
	"html/template"
)

var tmplStatsHTML *template.Template

const tmplsStatsHTML = `
<html>
  <head>
    <title>QPov stats</title>
    <style>
      .right {
        text-align: right;
      }
      .fixed {
        font-family: monospace;
        white-space: pre;
      }
    </style>
  </head>
  <body>
    <h1>QPov stats</h1>

    <img src="framerate.svg" />
    <img src="cpurate.svg" />
    <img src="leases.svg" />

    <table>
      <tr><th>CPU core time</th><td>{{.CpuTime|cputime2string}}</td></tr>
      <tr><th>Machine time</th><td>{{.MachineTime|cputime2string}}</td></tr>
    </table>
    <h2>Machine CPU time</h2>
    <table>
      {{range .MachineStats}}
      <tr>
        <td>{{.ArchSummary}}</td>
        <td class="right">{{.Jobs}}</td>
        <td class="fixed">{{.CpuTime|sumcpu|div6432 .NumCpu|seconds2string}}</td>
      </tr>
      {{end}}
    </table>

    <h2>CPU core time</h2>
    <table>
      {{range .MachineStats}}
      <tr>
        <td>{{.ArchSummary}}</td>
        <td class="right">{{.Jobs}}</td>
        <td class="fixed">{{.CpuTime|cputime2string}}</td>
      </tr>
      {{end}}
    </table>

    <h2>Per frame machine CPU time</h2>
    <table>
      {{range .MachineStats}}
      <tr>
        <td>{{.ArchSummary}}
        <td class="right">{{.Jobs}}</td>
        <td class="fixed">{{.CpuTime|sumcpu|div6432 .NumCpu|div6464 .Jobs|seconds2string}}</td>
        </tr>
      {{end}}
    </table>

    <h2>Per frame CPU core time</h2>
    <table>
      {{range .MachineStats}}
      <tr>
        <td>{{.ArchSummary}}</td>
        <td class="right">{{.Jobs|sprintf "%4d"}}</td>
        <td class="fixed">{{.CpuTime|sumcpu|div6464 .Jobs|seconds2string}}</td>
      </tr>
      {{end}}
    </table>
  </body>
</html>
`

func init() {
	tmplStatsHTML = template.New("tmpl_stats_html")
	tmplStatsHTML.Funcs(tmplStatsFuncs)
	template.Must(tmplStatsHTML.Parse(tmplsStatsHTML))
}
