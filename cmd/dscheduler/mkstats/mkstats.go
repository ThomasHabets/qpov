package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path"
	"regexp"
	"runtime/pprof"
	"sort"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/lib/pq"
	"golang.org/x/net/context"

	"github.com/ThomasHabets/qpov/pkg/dist"
	pb "github.com/ThomasHabets/qpov/pkg/dist/qpov"
)

var (
	db dist.DBWrap
	// TODO: Ask the scheduler for the leases instead of getting them from the DB directly.
	dbConnect  = flag.String("db", "", "Database connect string.")
	outDir     = flag.String("out", ".", "Directory to write stats files to.")
	cpuprofile = flag.String("cpuprofile", "", "Write cpu profile to file.")

	// cpu usage.
	batch2user = make(map[string]float64)
	batch2sys  = make(map[string]float64)
)

type event struct {
	time    time.Time
	lease   int
	arch    string
	cpuRate int64
}

type byTime []event

func (a byTime) Len() int           { return len(a) }
func (a byTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byTime) Less(i, j int) bool { return a[i].time.Before(a[j].time) }

// Return descriptive specs, model, short name, and core count
var cpuRE = regexp.MustCompile(`^model name\s+:\s+(.*)`)
var modelRE = regexp.MustCompile(`^model\s+:\s+(.*)`)
var spacesRE = regexp.MustCompile(`\s+`)
var revisionRE = regexp.MustCompile(`^Revision\s*:\s+(\w+)$`)

// Get machine type from cloud and cpuinfo data.
// Returns machine, model, short, ncpu.
//   machine = "X x CPU DESCRIPTION"
//   model = "CPU DESCRIPTION"
//   short = "Raspberry pi 3"
//   ncpu = X
func getMachine(cloud *pb.Cloud, cpuinfo string) (string, string, string, int) {
	var cpuName string
	var models int
	var processors int
	var revision string
	for _, l := range strings.Split(cpuinfo, "\n") {
		if strings.HasPrefix(l, "model name") {
			if models == 0 {
				m := cpuRE.FindStringSubmatch(l)
				cpuName = m[1]
			}
			models++
		}

		if strings.Contains(l, "Raspberry Pi Zero 2 W Rev 1.0") {
			processors = 4
			cpuName = "Raspberry Pi Zero 2"
		}
		if strings.Contains(l, "sifive,u74-mc") {
			cpuName = "SiFive U74 RISC-V"
		}
		if strings.HasPrefix(l, "processor\t") {
			processors++
		}
		if strings.Contains(l, "Raspberry Pi 4 Model") {
			processors = 4
			cpuName = "RaspberryPi4"
			break
		}
		if strings.HasPrefix(l, "Revision") && revision == "" {
			m := revisionRE.FindStringSubmatch(l)
			if len(m) == 0 {
				log.Println(l)
			}
			revision = m[1]
		}
	}
	cpus := models
	if cpus == 0 {
		cpus = processors
	}

	cNamePrefix := ""
	if cloud != nil {
		t := cloud
		if t.Provider == "Amazon" && t.InstanceType == "unavailable\n" {
			t = proto.Clone(cloud).(*pb.Cloud)
			t.Provider = "DigitalOcean"
			t.InstanceType = "unknown"
		}
		cNamePrefix = strings.TrimSpace(fmt.Sprintf("%s/%s", t.Provider, t.InstanceType)) + " "
	}
	if cpus != 0 {
		desc := fmt.Sprintf("%s%d x %s", cNamePrefix, cpus, cpuName)
		short := ""
		switch desc {
		case `1 x ARMv6-compatible processor rev 7 (v6l)`:
			short = "Raspberry Pi 1"
			if revision == "900093" {
				short = "Raspberry Pi Zero"
			}
		case `4 x ARMv7 Processor rev 5 (v7l)`: // Yes, the order here is correct. Pi2 has 5, Pi3 has 4.
			short = "Raspberry Pi 2"
		case `4 x ARMv7 Processor rev 4 (v7l)`:
			short = "Raspberry Pi 3"
		case `4 x RaspberryPi4`:
			short = "Raspberry Pi 4"
		case `2 x ARMv7 Processor rev 4 (v7l)`:
			short = "Banana Pi"
		case `4 x Raspberry Pi Zero 2`:
			short = "Raspberry Pi Zero 2"
		}
		return desc, cpuName, short, cpus
	}
	return "unknown", "unknown", "", 0
}

type sortGraphsT struct {
	t string
	d []tsInt
}
type sortGraphsTT []sortGraphsT

func (a sortGraphsTT) Len() int           { return len(a) }
func (a sortGraphsTT) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a sortGraphsTT) Less(i, j int) bool { return a[i].t < a[j].t }

func sortGraphs(lineTitles []string, data [][]tsInt) ([]string, [][]tsInt) {
	var s []sortGraphsT
	for n := range lineTitles {
		s = append(s, sortGraphsT{t: lineTitles[n], d: data[n]})
	}
	sort.Sort(sortGraphsTT(s))
	var l []string
	var d [][]tsInt
	for n := range lineTitles {
		l = append(l, s[n].t)
		d = append(d, s[n].d)
	}
	//lineTitles, data = sortGraphs(lineTitles, data)
	return l, d
}

// return mapping from order to slice of leases.
func getAllMetas(ctx context.Context, metaChan chan<- *pb.RenderingMetadata) error {
	rows, err := db.QueryContext(ctx, `
SELECT batch.batch_id, lease_id, leases.metadata
FROM batch
JOIN orders ON batch.batch_id=orders.batch_id
JOIN leases ON orders.order_id=leases.order_id
WHERE metadata IS NOT NULL`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var batch string
		var lease string
		var metas string
		if err := rows.Scan(&batch, &lease, &metas); err != nil {
			return err
		}
		p := pb.Lease{
			Metadata: &pb.RenderingMetadata{},
		}
		if err := json.Unmarshal([]byte(metas), p.Metadata); err != nil {
			return err
		}
		//ret[batch] = append(ret[batch], p)
		batch2sys[batch] += float64(p.Metadata.SystemMs) / 1000.0
		batch2user[batch] += float64(p.Metadata.UserMs) / 1000.0
		metaChan <- p.Metadata
	}
	if rows.Err() != nil {
		return err
	}
	return nil

}

func mkstatsBatch(ctx context.Context) ([]*pb.BatchStats, error) {
	var ret []*pb.BatchStats
	rows, err := db.QueryContext(ctx, `
SELECT
  a.batch_id,
  MAX(a.comment) AS comment,
  MAX(a.ctime) AS ctime,
  SUM(a.count) AS total,
  COALESCE(SUM(b.count), 0) AS done
FROM (
  SELECT
    batch.batch_id,
    MAX(batch.comment) AS comment,
    MAX(batch.ctime) AS ctime,
    COUNT(orders.order_id)
  FROM batch
  RIGHT OUTER JOIN orders
  ON batch.batch_id=orders.batch_id
  GROUP BY batch.batch_id
) a
FULL OUTER JOIN (
  SELECT
    batch.batch_id,
    COUNT(DISTINCT leases.order_id)
  FROM batch
  RIGHT OUTER JOIN orders
  ON batch.batch_id=orders.batch_id
  JOIN leases
  ON orders.order_id=leases.order_id
  WHERE leases.done=TRUE
  GROUP BY batch.batch_id
) b
ON a.batch_id=b.batch_id
GROUP BY a.batch_id
ORDER BY ctime NULLS FIRST
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var batch sql.NullString
		var total int64
		var done int64
		var ctime pq.NullTime
		var comment sql.NullString
		if err := rows.Scan(&batch, &comment, &ctime, &total, &done); err != nil {
			return nil, fmt.Errorf("scanning in mkStatsBatch: %v", err)
		}
		e := &pb.BatchStats{
			BatchId: batch.String,
			Total:   total,
			Done:    done,
			Comment: comment.String,
			CpuTime: &pb.StatsCPUTime{},
		}
		if ctime.Valid {
			e.Ctime = int64(ctime.Time.Unix())
		}
		user := batch2user[batch.String]
		sys := batch2sys[batch.String]
		// TODO: when we calculate computrons.
		compute := float64(0) // computeSeconds(&l)
		e.CpuTime.UserSeconds = int64(user)
		e.CpuTime.SystemSeconds = int64(sys)
		e.CpuTime.ComputeSeconds = int64(compute)
		ret = append(ret, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ret, nil
}

func computeSeconds(lease *pb.Lease) float64 {
	t := float64(lease.Metadata.UserMs+lease.Metadata.UserMs) / 1000.0

	_, model, short, _ := getMachine(lease.Metadata.Cloud, lease.Metadata.Cpuinfo)

	// Map well-known machines.
	// Calculated from average CPU time for rendering compared to reference.
	if mult, found := map[string]float64{
		"Raspberry Pi 1": 0.053754376708303145,
		"Raspberry Pi 2": 0.14210898167050498,
		"Raspberry Pi 3": 0.20090175794013015,
		"Banana Pi":      0.16817860869808196,
	}[short]; found {
		return t * mult
	}

	// Map well-known CPUs.
	// Calculated from average CPU time for rendering compared to reference.
	if mult, found := map[string]float64{
		"Intel(R) Core(TM) i7-2600 CPU @ 3.40GHz":     0.9839319678702234,
		"Intel(R) Core(TM)2 Duo CPU P8600 @ 2.40GHz":  1.0777118930049654,
		"Intel(R) Core(TM)2 Duo CPU T7700 @ 2.40GHz":  0.9075024501702071,
		"Intel(R) Core(TM)2 Quad CPU Q6600 @ 2.40GHz": 1,
		"Intel(R) Xeon(R) CPU E5-2420 0 @ 1.90GHz":    0.6615457783320475,
		"Intel(R) Xeon(R) CPU E5-2630 v3 @ 2.40GHz":   0.987832553206935,
		"Intel(R) Xeon(R) CPU E5-2630L v2 @ 2.40GHz":  1.1992535654892824,
		"Intel(R) Xeon(R) CPU E5-2666 v3 @ 2.90GHz":   1.161253218756725,
	}[model]; found {
		return t * mult
	}
	log.Printf("Could not map %q, defaulting to reference multiplier", model)
	return t
}

func mkstats(ctx context.Context, metaChan <-chan *pb.RenderingMetadata) (*pb.StatsOverall, error) {
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
		machine, _, name, cores := getMachine(meta.Cloud, meta.Cpuinfo)
		if name != "" {
			machine = fmt.Sprintf("%s: %s", name, machine)
		}
		machine2numcpu[machine] = cores
		machine2cloud[machine] = meta.Cloud

		cpuRate := ((meta.Rusage.Utime + meta.Rusage.Stime) / 1000) / (meta.EndMs - meta.StartMs)
		events = append(events,
			event{
				time:    time.Unix(meta.StartMs/1000, meta.StartMs%1000*1000000),
				cpuRate: cpuRate,
				arch:    meta.Uname.Machine,
				lease:   1,
			},
			event{
				time:    time.Unix(meta.EndMs/1000, meta.EndMs%1000*1000000),
				cpuRate: -cpuRate,
				arch:    meta.Uname.Machine,
				lease:   -1,
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

	from, err := time.Parse("2006-01-02", "2015-11-01")
	if err != nil {
		log.Fatal(err)
	}
	to := time.Now()

	// Simple non-cumulative graphs.
	{
		pos := make(map[string]int)        // Mapping from arch to line index.
		state := make(map[string][]int64)  // Map of graph name to current value.
		data := make(map[string][][]tsInt) // Map of graph name to a slice of line->linedata.

		types := map[string]struct {
			yAxisLabel string
			extractor  func(e event) int64
		}{
			"cpurate": {"CPU s/s", func(e event) int64 { return e.cpuRate }},
			"leases":  {"Leases", func(e event) int64 { return int64(e.lease) }},
		}

		var lineTitles []string
		for _, e := range events {
			n, found := pos[e.arch]
			if !found {
				n = len(pos)
				pos[e.arch] = n
				lineTitles = append(lineTitles, e.arch)
				for k := range types {
					state[k] = append(state[k], 0)
					data[k] = append(data[k], []tsInt{})
				}
			}
			for k, v := range types {
				data[k][n] = append(data[k][n], tsInt{e.time, state[k][n]})
				state[k][n] += v.extractor(e)
				data[k][n] = append(data[k][n], tsInt{e.time, state[k][n]})
			}
		}
		for k, v := range types {
			t, d := sortGraphs(lineTitles, data[k])
			if err := graphTimeLine(d, tsLine{
				LineTitles: t,
				From:       from,
				To:         to,
				YAxisLabel: v.yAxisLabel,
				OutputFile: path.Join(*outDir, fmt.Sprintf("%s.png", k)),
			}); err != nil {
				return nil, err
			}
		}
	}

	// Cumulative graphs (only frame rate for now).
	// TODO: add hertz.
	{
		var data []tsInt
		cur := 0
		for i := int64(from.Unix()) % 86400; i < int64(to.Unix()); i += 86400 {
			rfrom := i - 86400
			rto := i
			for cur > 0 && (events[cur].time.Unix() > rfrom) {
				cur--
			}
			// backed one too many.
			if events[cur].time.Unix() < rfrom && len(events) > cur+1 {
				cur++
			}
			var n int64
			for cur < len(events) && events[cur].time.Unix() < rto {
				if events[cur].lease < 0 {
					// Lease return action.
					n++
				}
				cur++
			}
			data = append(data, tsInt{time.Unix(i, 0), n})
		}
		if err := graphTimeLine([][]tsInt{data}, tsLine{
			LineTitles: []string{"Frame rate"},
			YAxisLabel: "Frames per day",
			From:       from,
			To:         to,
			OutputFile: path.Join(*outDir, "framerate.png"),
		}); err != nil {
			return nil, err
		}
	}

	s, err := mkstatsBatch(ctx)
	if err != nil {
		log.Fatalf("mkstatsBatch: %v", err)
	}
	stats.BatchStats = s
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

func main() {
	ctx := context.Background()
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)
	flag.Parse()
	if flag.NArg() > 0 {
		log.Fatalf("Got extra args on cmdline: %q", flag.Args())
	}
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatalf("Creating %q: %v", *cpuprofile, err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	log.Printf("Running mkstats")
	// Connect to database.
	var err error
	{
		t, err := sql.Open("postgres", *dbConnect)
		if err != nil {
			log.Fatal(err)
		}
		if err := t.PingContext(ctx); err != nil {
			log.Fatalf("db ping: %v", err)
		}
		db = dist.NewDBWrap(t, log.New(os.Stderr, "", log.LstdFlags))
	}

	metaChan := make(chan *pb.RenderingMetadata)
	go func() {
		defer close(metaChan)
		if err := getAllMetas(ctx, metaChan); err != nil {
			log.Fatal(err)
		}
	}()
	stats, err := mkstats(ctx, metaChan)
	if err != nil {
		log.Fatal(err)
	}

	if false {
		if err := dist.TmplStatsText.Execute(os.Stdout, stats); err != nil {
			log.Fatal(err)
		}
	}

	// Write stats to file.
	{
		bin, err := proto.Marshal(stats)
		if err != nil {
			log.Fatal(err)
		}
		fo, err := os.Create(path.Join(*outDir, "overall.pb"))
		if err != nil {
			log.Fatal(err)
		}
		defer fo.Close()
		if _, err := fo.Write(bin); err != nil {
			log.Fatal(err)
		}
		if err := fo.Close(); err != nil {
			log.Fatal(err)
		}
	}

	// Write HTML to file.
	{
		/*
			fo, err := os.Create(path.Join(*outDir, "index.html"))
			if err != nil {
				log.Fatal(err)
			}
			defer fo.Close()
			if err := dist.TmplStatsHTML.Execute(fo, &struct {
				Root  *string
				Stats *pb.StatsOverall
			}{Stats: stats}); err != nil {
				log.Fatal(err)
			}
			if err := fo.Close(); err != nil {
				log.Fatal(err)
			}
		*/
	}
}
