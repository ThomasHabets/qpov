package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"text/template"
	"time"
)

type tsInt struct {
	time  time.Time
	value int
}

type tsLine struct {
	OutputFile string
	LineTitle  string
	From, To   time.Time
}

var (
	tsLineTmpl = template.Must(template.New("").Parse(`
set timefmt "%Y-%m-%d_%H:%M:%S"
set xdata time
set format x "%Y-%m-%d"
set xrange [ "{{.From}}":"{{.To}}" ]

set terminal svg size 800,300
set output "{{.OutputFile}}"
plot "-" using 1:2 w l title "{{.LineTitle}}"
`))
)

func graphTimeLine(ts []tsInt, data tsLine) error {
	tsLineTmpl.Funcs(template.FuncMap{
		"fdate": func(t time.Time) string { return t.Format("2006-01-02") },
	})
	if data.OutputFile == "" {
		return fmt.Errorf("must supply an output file name")
	}
	if data.LineTitle == "" {
		data.LineTitle = "data"
	}
	if data.From.IsZero() {
		var err error
		data.From, err = time.Parse("2006-01-02", "2016-01-01")
		if err != nil {
			log.Fatalf("can't happen: %v", err)
		}
	}
	if data.To.IsZero() {
		data.To = time.Now()
	}
	cmd := exec.Command("gnuplot")
	var def bytes.Buffer
	if err := tsLineTmpl.Execute(&def, &data); err != nil {
		return err
	}
	for _, s := range ts {
		fmt.Fprintf(&def, "%v %d\n", s.time.Format("2006-01-02_15:04:05"), s.value)
	}
	cmd.Stdin = &def
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
