package dist

import (
	"encoding/json"
	"regexp"
	"syscall"
	"time"

	pb "github.com/ThomasHabets/qpov/dist/qpov"
)

type istats struct {
	User string // TODO

	Order string

	// Run stats of POV-Ray.
	Start                time.Time
	End                  time.Time
	SystemTime, UserTime time.Duration
	Rusage               syscall.Rusage

	// System info.
	Hostname string   // os.Hostname
	Uname    struct { // syscall.Uname
		Sysname    string
		Nodename   string
		Release    string
		Version    string
		Machine    string
		Domainname string
	}
	NumCPU       int    // runtime.CPUInfo
	Version      string // runtime.Version
	Cloud        string // Type of cloud. "Google" or "Amazon"
	InstanceType string // E.g. "c4.8xlarge"
	Comment      string // Custom comment.
	CPUInfo      string // /proc/cpuinfo
}

func tv2us(i syscall.Timeval) int64 {
	return int64(i.Sec)*1000000 + int64(i.Usec)
}

func ParseLegacyJSON(buf []byte) (*pb.RenderingMetadata, error) {
	var ist istats
	if err := json.Unmarshal(buf, &ist); err != nil {
		return nil, err
	}

	st := pb.RenderingMetadata{
		User:        ist.User,
		OrderString: ist.Order,

		StartMs:  ist.Start.UnixNano() / 1000000,
		EndMs:    ist.End.UnixNano() / 1000000,
		SystemMs: ist.SystemTime.Nanoseconds() / 1000000,
		UserMs:   ist.UserTime.Nanoseconds() / 1000000,
		Rusage: &pb.Rusage{
			Utime:    tv2us(ist.Rusage.Utime),
			Stime:    tv2us(ist.Rusage.Stime),
			Maxrss:   int64(ist.Rusage.Maxrss),
			Ixrss:    int64(ist.Rusage.Ixrss),
			Idrss:    int64(ist.Rusage.Idrss),
			Isrss:    int64(ist.Rusage.Isrss),
			Minflt:   int64(ist.Rusage.Minflt),
			Majflt:   int64(ist.Rusage.Majflt),
			Nswap:    int64(ist.Rusage.Nswap),
			Inblock:  int64(ist.Rusage.Inblock),
			Oublock:  int64(ist.Rusage.Oublock),
			Msgsnd:   int64(ist.Rusage.Msgsnd),
			Msgrcv:   int64(ist.Rusage.Msgrcv),
			Nsignals: int64(ist.Rusage.Nsignals),
			Nvcsw:    int64(ist.Rusage.Nvcsw),
			Nivcsw:   int64(ist.Rusage.Nivcsw),
		},
		Hostname: ist.Hostname,
		Uname: &pb.Uname{
			Sysname:    ist.Uname.Sysname,
			Nodename:   ist.Uname.Nodename,
			Release:    ist.Uname.Release,
			Version:    ist.Uname.Version,
			Machine:    ist.Uname.Machine,
			Domainname: ist.Uname.Domainname,
		},
		NumCpu:  int32(ist.NumCPU),
		Version: ist.Version,
		Comment: ist.Comment,
		Cpuinfo: ist.CPUInfo,
	}
	if ist.Cloud != "" {
		st.Cloud = &pb.Cloud{
			Provider:     ist.Cloud,
			InstanceType: ist.InstanceType,
		}
	}
	var order Order
	if err := json.Unmarshal([]byte(ist.Order), &order); err != nil {
		return nil, err
	}
	st.Order = LegacyOrderToOrder(&order)
	return &st, nil
}

func LegacyOrderToOrder(in *Order) *pb.Order {
	return &pb.Order{
		Package: in.Package,
		Dir:     in.Dir,
		File:    in.File,
		Args:    in.Args,
	}
}

func Arch(m *pb.RenderingMetadata) string {
	for _, a := range []struct {
		name  string
		match *regexp.Regexp
	}{
		{
			"Banana Pi",
			regexp.MustCompile(`^processor\t: 0
model name\t: ARMv7 Processor rev 4 \(v7l\)
BogoMIPS\t: [0-9.]+
Features\t: half thumb fastmult vfp edsp neon vfpv3 tls vfpv4 idiva idivt vfpd32 lpae evtstrm 
CPU implementer\t: 0x41
CPU architecture: 7
CPU variant\t: 0x0
CPU part\t: 0xc07
CPU revision\t: 4

processor\t: 1
model name\t: ARMv7 Processor rev 4 \(v7l\)
BogoMIPS\t: [0-9.]+
Features\t: half thumb fastmult vfp edsp neon vfpv3 tls vfpv4 idiva idivt vfpd32 lpae evtstrm 
CPU implementer\t: 0x41
CPU architecture: 7
CPU variant\t: 0x0
CPU part\t: 0xc07
CPU revision\t: 4

Hardware\t: Allwinner sun7i \(A20\) Family
Revision\t: 0000
Serial\t\t: \w+
$`),
		},
	} {
		if a.match.MatchString(m.Cpuinfo) {
			return a.name
		}
	}
	return "Unknown"
}
