// -*- html -*-
package dist

import (
	"html/template"
)

var TmplStatsHTML *template.Template

const tmplsStatsHTML = `
{{ $root := . }}
<h1>QPov stats</h1>

<img src="{{.Root}}/stats/framerate.svg" />
<img src="{{.Root}}/stats/cpurate.svg" />
<img src="{{.Root}}/stats/leases.svg" />

<table>
  <tr><th>CPU core time</th><td>{{.Stats.CpuTime|cputime2string}}</td></tr>
  <tr><th>Machine time</th><td>{{.Stats.MachineTime|cputime2string}}</td></tr>
</table>
<h2>Machine CPU time</h2>
<table>
  {{range .Stats.MachineStats}}
  <tr>
    <td>{{.ArchSummary}}</td>
    <td class="right">{{.Jobs}}</td>
    <td class="fixed">{{.CpuTime|sumcpu|div6432 .NumCpu|seconds2string}}</td>
  </tr>
  {{end}}
</table>

<h2>CPU core time</h2>
<table>
  {{range .Stats.MachineStats}}
  <tr>
    <td>{{.ArchSummary}}</td>
    <td class="right">{{.Jobs}}</td>
    <td class="fixed">{{.CpuTime|cputime2string}}</td>
  </tr>
  {{end}}
</table>

<h2>Per frame machine CPU time</h2>
<table>
  {{range .Stats.MachineStats}}
  <tr>
    <td>{{.ArchSummary}}
    <td class="right">{{.Jobs}}</td>
    <td class="fixed">{{.CpuTime|sumcpu|div6432 .NumCpu|div6464 .Jobs|seconds2string}}</td>
  </tr>
  {{end}}
</table>

<h2>Per frame CPU core time</h2>
<table>
  {{range .Stats.MachineStats}}
  <tr>
    <td>{{.ArchSummary}}</td>
    <td class="right">{{.Jobs|sprintf "%4d"}}</td>
    <td class="fixed">{{.CpuTime|sumcpu|div6464 .Jobs|seconds2string}}</td>
  </tr>
  {{end}}
</table>
`

func init() {
	TmplStatsHTML = template.New("tmpl_stats_html")
	TmplStatsHTML.Funcs(tmplStatsFuncs)
	template.Must(TmplStatsHTML.Parse(tmplsStatsHTML))
}
