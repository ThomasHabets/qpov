package main

import (
	"fmt"
	"text/template"

	pb "github.com/ThomasHabets/qpov/dist/qpov"
)

const tmplsStatsText = `CPU core time: {{.CpuTime|cputime2string}}
Machine time:  {{.MachineTime|cputime2string}}
Machine CPU time:
{{range .MachineStats}}  {{.ArchSummary|sprintf "%-70s" -}}
: {{.Jobs|sprintf "%4d"}} (time: {{.CpuTime|sumcpu|div6432 .NumCpu|seconds2string}}
{{end}}
CPU core time:
{{range .MachineStats}}  {{.ArchSummary|sprintf "%-70s" -}}
: {{.Jobs|sprintf "%4d"}} (time: {{.CpuTime|cputime2string}}
{{end}}
Per frame machine CPU time:
{{range .MachineStats}}  {{.ArchSummary|sprintf "%-70s" -}}
: {{.Jobs|sprintf "%4d"}} (time: {{.CpuTime|sumcpu|div6432 .NumCpu|div6464 .Jobs|seconds2string}}
{{end}}
Per frame machine CPU time:
{{range .MachineStats}}  {{.ArchSummary|sprintf "%-70s" -}}
: {{.Jobs|sprintf "%4d"}} (time: {{.CpuTime|sumcpu|div6464 .Jobs|seconds2string}}
{{end}}`

var (
	tmplStatsText  *template.Template
	tmplStatsFuncs = map[string]interface{}{
		"sprintf": fmt.Sprintf,
		"sumcpu":  func(c *pb.StatsCPUTime) int64 { return c.UserSeconds + c.SystemSeconds },
		"div6432": func(b int32, a int64) int64 { return a / int64(b) },
		"div6464": func(b, a int64) int64 { return a / b },
		"seconds2string": func(s int64) string {
			return fmtSecondDuration(s)
		},
		"cputime2string": func(c *pb.StatsCPUTime) string {
			return fmtSecondDuration(c.UserSeconds + c.SystemSeconds)
		},
	}
)

func init() {
	tmplStatsText = template.New("tmpl_stats_text")
	tmplStatsText.Funcs(tmplStatsFuncs)
	template.Must(tmplStatsText.Parse(tmplsStatsText))
}
