package dist

import (
	"fmt"
	"strings"
	"text/template"
	"time"

	pb "github.com/ThomasHabets/qpov/pkg/dist/qpov"
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
		"sprintf":        fmt.Sprintf,
		"sumcpu":         func(c *pb.StatsCPUTime) int64 { return c.UserSeconds + c.SystemSeconds },
		"div6432":        func(b int32, a int64) int64 { return a / int64(b) },
		"div6464":        func(b, a int64) int64 { return a / b },
		"seconds2string": func(s int64) string { return FmtSecondDuration(s) },
		"cputime2string": func(c *pb.StatsCPUTime) string {
			return FmtSecondDuration(c.UserSeconds + c.SystemSeconds)
		},
	}
)

func FormatDuration(t time.Duration) string {
	h, m, s := int(t.Hours()), int(t.Minutes()), int(t.Seconds())
	var ret []string
	d := h / 24
	y := d / 365
	d %= 365
	h %= 24
	m %= 60
	s %= 60
	if y > 0 {
		ret = append(ret, fmt.Sprintf("%4dy", y))
	}
	if len(ret) > 0 || d > 0 {
		ret = append(ret, fmt.Sprintf("%3dd", d))
	}
	if len(ret) > 0 || h > 0 {
		ret = append(ret, fmt.Sprintf("%2dh", h))
	}
	if len(ret) > 0 || m > 0 {
		ret = append(ret, fmt.Sprintf("%2dm", m))
	}
	ret = append(ret, fmt.Sprintf("%2ds", s))
	return strings.Join(ret, " ")
}

const (
	secondsPerYear int64 = 86400 * 365.24
)

func FmtSecondDuration(e int64) string {
	var years string
	if d := secondsPerYear; e > d {
		years = fmt.Sprintf("%dy ", e/d)
		e %= d
	}
	return years + FormatDuration(time.Second*time.Duration(e))
}

func init() {
	TmplStatsText = template.New("tmpl_stats_text")
	TmplStatsText.Funcs(tmplStatsFuncs)
	template.Must(TmplStatsText.Parse(tmplsStatsText))
}
