package dist

import (
	"fmt"
	"text/template"
	"time"

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
	TmplStatsText  *template.Template
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

func formatDuration(t time.Duration) string {
	h, m, s := int(t.Hours()), int(t.Minutes()), int(t.Seconds())
	d := h / 24
	y := d / 365
	d %= 365
	h %= 24
	m %= 60
	s %= 60
	return fmt.Sprintf("%4dy %3dd %2dh %2dm %2ds", y, d, h, m, s)
}

func fmtSecondDuration(e int64) string {
	return formatDuration(time.Second * time.Duration(e))
}

func init() {
	TmplStatsText = template.New("tmpl_stats_text")
	TmplStatsText.Funcs(tmplStatsFuncs)
	template.Must(TmplStatsText.Parse(tmplsStatsText))
}
