package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"text/template"
	"time"
)

type tsInt struct {
	time  time.Time
	value int64
}

type tsLine struct {
	OutputFile string
	LineTitles []string
	YAxisLabel string
	From, To   time.Time
}

var (
	tsLineTmpl  *template.Template
	tsLineTmpls = `
{{$root := .}}
set timefmt "%Y-%m-%d_%H:%M:%S"
set xdata time
set ylabel "{{.Meta.YAxisLabel}}"
set format x "%Y-%m-%d"
set xrange [ "{{.Meta.From}}":"{{.Meta.To}}" ]

set terminal svg size 800,300
# set terminal png size 800,300 truecolor
set output "{{.Meta.OutputFile}}"
plot \
{{range enumerate .Count -}}
    "{{$root.Dir}}/data.txt" using 1:(sum [col={{.|add 2}}:{{add $root.Count 1}}] column(col)) \
    w filledcurves x1 title "{{lookup $root.Meta.LineTitles .}}"{{maybeComma . $root.Count}} \
{{end}}

# filledcurves x1
#  for [i=2:{{.Count}}] \
#    sprintf("{{$root.Dir}}/data.txt", i) using 1:(sum [col=i:{{.Count}}] column(col)) \
#      notitle \
#      with lines lc rgb "#000000" lt -1 lw 1
`
)

func init() {
	tsLineTmpl = template.New("tslinetmpl")
	tsLineTmpl.Funcs(template.FuncMap{
		"add":    func(a int, b int) int64 { return int64(a) + int64(b) },
		"lookup": func(m []string, n int) string { return m[n] },
		"fdate":  func(t time.Time) string { return t.Format("2006-01-02") },
		"maybeComma": func(a, b int) string {
			if a == b-1 {
				return ""
			} else {
				return ","
			}
		},
		"enumerate": func(n int) []int {
			var ret []int
			for i := 0; i < n; i++ {
				ret = append(ret, i)
			}
			return ret
		},
	})
	template.Must(tsLineTmpl.Parse(tsLineTmpls))
}

type flipPoint struct {
	time   time.Time
	values []int64
}

func flip(ts [][]tsInt) []flipPoint {
	pos := make([]int, len(ts), len(ts))
	var ret []flipPoint
	first := true
	for {
		min := tsInt{time: time.Now()}
		minStream := -1
		for n, cur := range ts {
			try := pos[n] + 1
			if try < len(cur) && min.time.After(cur[try].time) {
				minStream = n
				min = cur[try]
			}
		}
		if minStream == -1 {
			break
		}
		if first {
			first = false
			var vs []int64
			for n := range ts {
				vs = append(vs, ts[n][pos[n]].value)
			}
			ret = append(ret, flipPoint{time: ts[minStream][pos[minStream]].time, values: vs})
		}
		pos[minStream]++
		var vs []int64
		for n := range ts {
			vs = append(vs, ts[n][pos[n]].value)
		}
		ret = append(ret, flipPoint{time: min.time, values: vs})
	}
	return ret
}

func graphTimeLine(ts [][]tsInt, data tsLine) error {
	if data.OutputFile == "" {
		return fmt.Errorf("must supply an output file name")
	}
	//	if data.LineTitle == "" {
	//		data.LineTitle = "data"
	//	}
	if data.From.IsZero() {
		var err error
		data.From, err = time.Parse("2006-01-02", "2015-11-01")
		if err != nil {
			log.Fatalf("can't happen: %v", err)
		}
	}
	if data.To.IsZero() {
		data.To = time.Now()
	}

	// Write data files.
	dir, err := ioutil.TempDir("", "qpov_dscheduler_mkstats")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	fo, err := os.Create(path.Join(dir, "data.txt"))
	if err != nil {
		return err
	}
	for _, line := range flip(ts) {
		fmt.Fprintf(fo, "%v", line.time.Format("2006-01-02_15:04:05"))
		for _, v := range line.values {
			fmt.Fprintf(fo, " %d", v)
		}
		fmt.Fprintf(fo, "\n")
	}
	if err := fo.Close(); err != nil {
		return err
	}

	// Run gnuplot.
	cmd := exec.Command("gnuplot")
	var def bytes.Buffer
	if err := tsLineTmpl.Execute(&def, &struct {
		Count int
		Dir   string
		Meta  *tsLine
	}{
		Count: len(ts),
		Dir:   dir,
		Meta:  &data,
	}); err != nil {
		return err
	}
	cmd.Stdin = &def
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
