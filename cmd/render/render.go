package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"sync"
	"time"
)

var (
	povray      = flag.String("povray", "/usr/bin/povray", "Path to povray.")
	schedtool   = flag.String("schedtool", "/usr/bin/schedtool", "Path to schedtool.")
	concurrency = flag.Int("concurrency", 4, "Run this many povrays in parallel.")
	fast        = flag.Bool("fast", false, "Fast rendering.")
	hq          = flag.Bool("hq", true, "High quality.")

	mutex                  sync.Mutex
	totalUser, totalSystem time.Duration
)

func doRender(files []string, done chan<- int) {
	for n, f := range files {
		func() {
			stdout, err := os.Create(fmt.Sprintf("%s.stdout", f))
			if err != nil {
				log.Fatalf("Failed to open stdout file: %v", err)
			}
			defer stdout.Close()

			stats, err := os.Create(fmt.Sprintf("%s.stats", f))
			if err != nil {
				log.Fatalf("Failed to open stdout file: %v", err)
			}
			defer stats.Close()

			stderr, err := os.Create(fmt.Sprintf("%s.stderr", f))
			if err != nil {
				log.Fatalf("Failed to open stderr file: %v", err)
			}
			defer stderr.Close()

			args := []string{
				"-D", "-e",
				*povray,
				"-D",
			}
			if *fast {
				args = append(args, "+Q0", "+W800", "+H450")
			} else if *hq {
				args = append(args, "+Q11", "+A0.1", "+R4", "+W1600", "+H900")
			} else {
				args = append(args, "+W1600", "+H900")
			}
			args = append(args, path.Base(f))
			cmd := exec.Command(*schedtool, args...)
			cmd.Stdout = stdout
			cmd.Stderr = stderr
			cmd.Dir = path.Dir(f)

			st := time.Now()
			if err := cmd.Run(); err != nil {
				log.Fatalf("Failed to render %q: %v", f, err)
			}
			fmt.Fprintf(stats, "Sys: %+v\n", cmd.ProcessState.Sys())
			fmt.Fprintf(stats, "SysUsage: %+v\n", cmd.ProcessState.SysUsage())
			fmt.Fprintf(stats, "System time: %v\n", cmd.ProcessState.SystemTime())
			fmt.Fprintf(stats, "User time: %v\n", cmd.ProcessState.UserTime())
			fmt.Fprintf(stats, "Real time: %v\n", time.Since(st))
			func() {
				mutex.Lock()
				defer mutex.Unlock()
				totalUser += cmd.ProcessState.UserTime()
				totalSystem += cmd.ProcessState.SystemTime()
			}()
		}()
		done <- n
	}
}

func main() {
	flag.Parse()
	done := make(chan int)

	total := len(flag.Args())
	step := total / *concurrency
	st := time.Now()
	for i := 0; i < *concurrency-1; i++ {
		go doRender(flag.Args()[i*step:(i+1)*step], done)
	}
	go doRender(flag.Args()[(*concurrency-1)*step:], done)

	finished := 0
	for _ = range done {
		finished++
		fmt.Printf("Finished %d of %d\n", finished, len(flag.Args()))
		if finished == len(flag.Args()) {
			break
		}
	}
	mutex.Lock()
	defer mutex.Unlock()
	totalTime := time.Since(st)
	fmt.Printf("Total time: %v (%v user + %v system = %g parallelism)\n", totalTime, totalUser, totalSystem, float64(totalUser+totalSystem)/float64(totalTime))
}
