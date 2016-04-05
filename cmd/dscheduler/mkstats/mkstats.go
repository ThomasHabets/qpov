package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	_ "github.com/lib/pq"

	pb "github.com/ThomasHabets/qpov/dist/qpov"
)

var (
	db *sql.DB
	// TODO: Ask the scheduler for the leases instead of getting them from the DB directly.
	dbConnect = flag.String("db", "", "Database connect string.")
)

type event struct {
	time    time.Time
	lease   int
	cpuTime int64
}

type byTime []event

func (a byTime) Len() int           { return len(a) }
func (a byTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byTime) Less(i, j int) bool { return a[i].time.Before(a[j].time) }

// Return descriptive specs, short name, and core count
func getMachine(cloud *pb.Cloud, cpuinfo string) (string, string, int) {
	cNamePrefix := ""
	if cloud != nil {
		t := proto.Clone(cloud).(*pb.Cloud)
		if t.Provider == "Amazon" && t.InstanceType == "unavailable\n" {
			t.Provider = "DigitalOcean"
			t.InstanceType = "unknown"
		}
		cNamePrefix = strings.TrimSpace(fmt.Sprintf("%s/%s", t.Provider, t.InstanceType)) + " "
	}
	cpuRE := regexp.MustCompile(`(?m)^model name\s+:\s+(.*)$`)
	spacesRE := regexp.MustCompile(`\s+`)
	m := cpuRE.FindAllStringSubmatch(cpuinfo, -1)
	if len(m) != 0 {
		name := m[0][1]
		num := len(m)
		desc := spacesRE.ReplaceAllString(fmt.Sprintf("%s%d x %s", cNamePrefix, num, name), " ")
		short, _ := map[string]string{
			// Yes, the order here is correct. Pi2 has 5, Pi3 has 4.
			`1 x ARMv6-compatible processor rev 7 (v6l)`: "Raspberry Pi 1",
			`4 x ARMv7 Processor rev 5 (v7l)`:            "Raspberry Pi 2",
			`4 x ARMv7 Processor rev 4 (v7l)`:            "Raspberry Pi 3",
			`2 x ARMv7 Processor rev 4 (v7l)`:            "Banana Pi",
		}[desc]
		return desc, short, num
	}
	return "unknown", "", 0
}

func streamMeta(metaChan chan<- *pb.RenderingMetadata) error {
	rows, err := db.Query(`
SELECT metadata
FROM leases
WHERE done=TRUE
AND metadata IS NOT NULL
`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var metas sql.NullString
		if err := rows.Scan(&metas); err != nil {
			return err
		}
		meta := &pb.RenderingMetadata{}
		if err := json.Unmarshal([]byte(metas.String), &meta); err != nil {
			log.Printf("Parsing %q: %v", metas.String, err)
			continue
		}
		metaChan <- meta
	}
	if rows.Err() != nil {
		return err
	}
	return nil
}

func mkstats(metaChan <-chan *pb.RenderingMetadata) (*pb.StatsOverall, error) {
	stats := &pb.StatsOverall{
		StatsTimestamp: int64(time.Now().Unix()),
		CpuTime:        &pb.StatsCPUTime{},
		MachineTime:    &pb.StatsCPUTime{},
	}

	// Deltas.
	var events []event
	machine2cloud := make(map[string]*pb.Cloud)
	machine2numcpu := make(map[string]int)
	machine2cpu := make(map[string]string)
	machine2jobs := make(map[string]int)
	machine2userTime := make(map[string]int64)
	machine2systemTime := make(map[string]int64)
	for meta := range metaChan {
		machine, name, cores := getMachine(meta.Cloud, meta.Cpuinfo)
		if name != "" {
			machine = fmt.Sprintf("%s: %s", name, machine)
		}
		machine2numcpu[machine] = cores
		machine2cloud[machine] = meta.Cloud

		events = append(events,
			event{
				time:  time.Unix(meta.StartMs/1000, meta.StartMs%1000*1000000),
				lease: 1,
			},
			event{
				time:  time.Unix(meta.EndMs/1000, meta.EndMs%1000*1000000),
				lease: -1,
			})
		machine2jobs[machine]++
		machine2userTime[machine] += meta.Rusage.Utime
		machine2systemTime[machine] += meta.Rusage.Stime
	}

	for _, k := range sortedMapKeysSI(machine2jobs) {
		stats.MachineTime.UserSeconds += int64(machine2userTime[k]) / 1000000 / int64(machine2numcpu[k])
		stats.MachineTime.UserSeconds += int64(machine2systemTime[k]) / 1000000 / int64(machine2numcpu[k])
		stats.CpuTime.UserSeconds += int64(machine2userTime[k]) / 1000000
		stats.CpuTime.SystemSeconds += int64(machine2systemTime[k]) / 1000000
		stats.MachineStats = append(stats.MachineStats, &pb.MachineStats{
			ArchSummary: k,
			Cpu:         machine2cpu[k],
			Cloud:       machine2cloud[k],
			NumCpu:      int32(machine2numcpu[k]),
			CpuTime: &pb.StatsCPUTime{
				UserSeconds:   int64(machine2userTime[k]) / 1000000,
				SystemSeconds: int64(machine2systemTime[k]) / 1000000,
			},
			Jobs: int64(machine2jobs[k]),
		})
	}

	sort.Sort(byTime(events))
	var leases int
	var data []tsInt
	for _, e := range events {
		data = append(data, tsInt{e.time, leases})
		leases += e.lease
		data = append(data, tsInt{e.time, leases})
	}
	if err := graphTimeLine(data, tsLine{
		LineTitle:  "Active leases",
		OutputFile: "line.svg",
	}); err != nil {
		return nil, err
	}
	return stats, nil
}

func sortedMapKeysSI(m map[string]int) []string {
	var ret []string
	for k := range m {
		ret = append(ret, k)
	}
	sort.Strings(ret)
	return ret
}

func formatInt(i int64) string {
	var parts []string
	for i > 0 {
		parts = append([]string{fmt.Sprintf("%03d", i%1000)}, parts...)
		i /= 1000
	}
	ret := strings.TrimPrefix(strings.Join(parts, ","), "0")
	if ret == "" {
		ret = "0"
	}
	return ret
}

func formatFloat(in float64) string {
	i, f := math.Modf(in)
	fs := fmt.Sprint(f)[1:]
	return fmt.Sprintf("%s%s", formatInt(int64(i)), fs)
}

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

func main() {
	flag.Parse()
	if flag.NArg() > 0 {
		log.Fatalf("Got extra args on cmdline: %q", flag.Args())
	}
	log.Printf("Running mkstats")
	// Connect to database.
	var err error
	db, err = sql.Open("postgres", *dbConnect)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("db ping: %v", err)
	}

	metaChan := make(chan *pb.RenderingMetadata)
	streamErr := make(chan error, 1)
	go func() {
		defer close(metaChan)
		defer close(streamErr)
		streamErr <- streamMeta(metaChan)
	}()
	stats, err := mkstats(metaChan)
	if err != nil {
		log.Fatal(err)
	}
	if err := <-streamErr; err != nil {
		log.Fatal(err)
	}

	if err := tmplStatsText.Execute(os.Stdout, stats); err != nil {
		log.Fatal(err)
	}
}
