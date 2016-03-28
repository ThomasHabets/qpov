package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/fcgi"
	"os"
	"path"
	"sync"
	"time"

	"github.com/ThomasHabets/go-uuid/uuid"
	"github.com/golang/protobuf/proto"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"

	"github.com/ThomasHabets/qpov/dist"
	pb "github.com/ThomasHabets/qpov/dist/qpov"
)

const (
	userAgent = "dscheduler-webui"

	leaseTmpl = `
<html>
  <head>
  </head>
  <body>
    <h1>Lease {{.Lease.LeaseId}}</h1>
    <pre>{{.Ascii}}</pre>
    <hr>
    Page server time: {{.PageTime}}
  </body>
</html>
`
)

var (
	pageDeadline = flag.Duration("page_deadline", time.Second, "Page timeout.")
	socketPath   = flag.String("socket", "", "Unix socket to listen to.")
	caFile       = flag.String("ca_file", "", "Server CA file.")
	certFile     = flag.String("cert_file", "", "Client cert file.")
	keyFile      = flag.String("key_file", "", "Client key file.")
	schedAddr    = flag.String("scheduler", "", "Scheduler address.")

	sched    pb.SchedulerClient
	tmplRoot template.Template

	forwardRPCKeys = []string{"id", "source", "http.remote_addr", "http.cookie"}
)

func httpContext(r *http.Request) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "source", "http")
	ctx = context.WithValue(ctx, "id", uuid.New())
	ctx = context.WithValue(ctx, "http.remote_addr", r.RemoteAddr)
	if c, err := r.Cookie("qpov"); err == nil {
		ctx = context.WithValue(ctx, "http.cookie", c.Value)
	}
	return ctx
}

func fmsdate(s string, ms int64) string {
	return time.Unix(ms/1000, 0).Format(s)
}

func fileonly(s string) string {
	return path.Base(s)
}

func fmssub(a, b int64) string {
	return time.Unix(a/1000, 0).Sub(time.Unix(b/1000, 0)).String()
}

func fmsuntil(ms int64) string {
	now := time.Now().UnixNano() / 1000000000
	return time.Unix(ms/1000, 0).Sub(time.Unix(now, 0)).String()
}

func fmssince(ms int64) string {
	now := time.Now().UnixNano() / 1000000000
	return time.Unix(now, 0).Sub(time.Unix(ms/1000, 0)).String()
}

func getLeases(ctx context.Context, done bool) ([]*pb.Lease, error) {
	stream, err := sched.Leases(ctx, &pb.LeasesRequest{
		Done:  done,
		Order: true,
	})
	if err != nil {
		return nil, fmt.Errorf("Leases RPC: %v", err)
	}
	var leases []*pb.Lease
	for {
		r, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Leases RPC stream: %v", err)
		}
		leases = append(leases, r.Lease)
	}
	return leases, nil
}

func handleImage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(httpContext(r), time.Minute)
	defer cancel()

	lease, ok := mux.Vars(r)["leaseID"]
	if !ok {
		log.Printf("Internal error: leaseID not passed in to handleImage")
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	stream, err := sched.Result(ctx, &pb.ResultRequest{
		LeaseId: lease,
		Data:    true,
	})
	if err != nil {
		http.Error(w, "Backend broke :-(", http.StatusBadGateway)
		return
	}

	ch := make(chan []byte, 1000)
	writerDone := make(chan struct{}, 1)
	go func() {
		defer func() {
			close(writerDone)
			for _ = range ch {
			}
		}()
		for data := range ch {
			if _, err := w.Write(data); err != nil {
				log.Printf("Failed streaming result to client: %v", err)
				return
			}
		}
	}()

	func() {
		defer close(ch)
		for {
			select {
			case <-writerDone:
				return
			default:
			}
			r, err := stream.Recv()
			if err == io.EOF {
				break
			} else if err != nil {
				log.Printf("Failed streaming result over RPC: %v", err)
				return
			}
			// Only sent on first packet.
			if r.ContentType != "" {
				w.Header().Set("Content-Type", r.ContentType)
			}
			ch <- r.Data
		}
	}()
	<-writerDone
}

func handleLease(ctx context.Context, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(httpContext(r), 10*time.Second)
	defer cancel()

	// Get lease.
	lease, ok := mux.Vars(r)["leaseID"]
	if !ok {
		log.Printf("leaseID not passed in to handleLease")
		return nil, fmt.Errorf("no lease provided")
	}
	reply, err := sched.Lease(ctx, &pb.LeaseRequest{LeaseId: lease})
	if err != nil {
		if grpc.Code(err) == codes.Unauthenticated {
			return nil, httpError(http.StatusForbidden, "Unauthenticated", "Unauthenticated")
		} else {
			log.Printf("Backend call: %v", err)
			return nil, fmt.Errorf("backend broke :-(")
		}
	}
	return &struct {
		Lease    *pb.Lease
		Ascii    string
		PageTime time.Duration
	}{
		Lease:    reply.Lease,
		Ascii:    proto.MarshalTextString(reply.Lease),
		PageTime: time.Since(startTime),
	}, nil
}

func handleRoot(ctx context.Context, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	startTime := time.Now()
	var errs []error
	var m sync.Mutex
	var wg sync.WaitGroup

	// Get Stats.
	var st *pb.StatsReply
	wg.Add(1)
	go func() {
		var err error
		defer wg.Done()
		st, err = sched.Stats(ctx, &pb.StatsRequest{
			SchedulingStats: true,
		})
		if err != nil {
			m.Lock()
			defer m.Unlock()
			errs = append(errs, err)
			return
		}
	}()

	// Get active Leases.
	var leases []*pb.Lease
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		leases, err = getLeases(ctx, false)
		if err != nil {
			m.Lock()
			defer m.Unlock()
			errs = append(errs, fmt.Errorf("getLeases(false): %v", err))
			return
		}
	}()

	// Get done Leases.
	var doneLeases []*pb.Lease
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		doneLeases, err = getLeases(ctx, true)
		if err != nil {
			m.Lock()
			defer m.Unlock()
			errs = append(errs, fmt.Errorf("getLeases(true): %v", err))
			return
		}
	}()
	wg.Wait()
	if len(errs) > 0 {
		log.Printf("Errors: %v", errs)
	}
	ret := &struct {
		Stats           *pb.StatsReply
		Leases          []*pb.Lease
		DoneLeases      []*pb.Lease
		UnstartedOrders int64
		Errors          []error
		PageTime        time.Duration
	}{
		Stats:      st,
		Leases:     leases,
		DoneLeases: doneLeases,
		Errors:     errs,
		PageTime:   time.Since(startTime),
	}
	if st != nil {
		ret.UnstartedOrders = st.SchedulingStats.Orders - st.SchedulingStats.DoneOrders
	}
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	return ret, nil
}

type handleFunc func(ctx context.Context, w http.ResponseWriter, r *http.Request) (interface{}, error)
type handler struct {
	tmpl *template.Template
	f    handleFunc
}

type httpErr struct {
	code            int
	private, public string
}

func httpError(code int, pub, priv string) *httpErr {
	return &httpErr{
		code:    code,
		private: priv,
		public:  pub,
	}
}
func (e *httpErr) Error() string {
	return fmt.Sprintf("HTTP Error %d: %v", e.code, e.private)
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Strict-Transport-Security", "max-age=2592000")
	ctx, cancel := context.WithTimeout(httpContext(r), *pageDeadline)
	defer cancel()
	data, err := h.f(ctx, w, r)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		code := http.StatusInternalServerError
		msg := "Internal error"
		e2, ok := err.(*httpErr)
		if ok {
			code = e2.code
			msg = e2.public
		}
		w.WriteHeader(code)
		fmt.Fprintf(w, "Error: %v", msg)
		log.Printf("Error rendering page: %v", err)
		return
	}
	var buf bytes.Buffer
	if err := h.tmpl.Execute(&buf, data); err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Template rendering failed: %v", err)
		fmt.Fprintf(w, "Internal error: Failed to render page")
		return
	}
	if r.Method != "HEAD" {
		if _, err := w.Write(buf.Bytes()); err != nil {
			log.Printf("Failed to write page to network: %v", err)
			return
		}
	}
}

func wrap(f handleFunc, t string) *handler {
	tmpl := template.New("blah")
	tmpl.Funcs(template.FuncMap{
		"fmsdate":  fmsdate,
		"fmsuntil": fmsuntil,
		"fmssince": fmssince,
		"fmssub":   fmssub,
		"fileonly": fileonly,
	})
	template.Must(tmpl.Parse(t))
	return &handler{f: f, tmpl: tmpl}
}

func connectScheduler(addr string) error {
	caStr := dist.CacertClass1
	if *caFile != "" {
		b, err := ioutil.ReadFile(*caFile)
		if err != nil {
			return fmt.Errorf("reading %q: %v", *caFile, err)
		}
		caStr = string(b)
	}

	// Root CA.
	cp := x509.NewCertPool()
	if ok := cp.AppendCertsFromPEM([]byte(caStr)); !ok {
		return fmt.Errorf("failed to add root CAs")
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("failed to split host/port out of %q", addr)
	}

	tlsConfig := tls.Config{
		ServerName: host,
		RootCAs:    cp,
	}

	// Client Cert.
	if *certFile != "" {
		cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			return fmt.Errorf("failed to load client keypair %q/%q: %v", *certFile, *keyFile, err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	cr := credentials.NewTLS(&tlsConfig)
	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(cr),
		grpc.WithPerRPCCredentials(&perRPC{}),
		grpc.WithUserAgent(userAgent),
	)
	if err != nil {
		return fmt.Errorf("dialing scheduler %q: %v", addr, err)
	}
	sched = pb.NewSchedulerClient(conn)
	return nil
}

type perRPC struct{}

func (*perRPC) RequireTransportSecurity() bool {
	return true
}

func (*perRPC) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	ret := make(map[string]string)
	for _, k := range forwardRPCKeys {
		t, ok := ctx.Value(k).(string)
		if ok {
			ret[k] = t
		}
	}
	return ret, nil
}

func main() {
	flag.Parse()

	if err := connectScheduler(*schedAddr); err != nil {
		log.Fatalf("Failed to connect to scheduler: %v", err)
	}

	sock, err := net.Listen("unix", *socketPath)
	if err != nil {
		log.Fatalf("Unable to listen to socket: %v", err)
	}
	if err = os.Chmod(*socketPath, 0666); err != nil {
		log.Fatalf("Unable to chmod socket: %v", err)
	}

	r := mux.NewRouter()
	r.Handle("/", wrap(handleRoot, rootTmpl)).Methods("GET", "HEAD")
	r.HandleFunc("/image/{leaseID}", handleImage).Methods("GET", "HEAD")
	r.Handle("/lease/{leaseID}", wrap(handleLease, leaseTmpl)).Methods("GET", "HEAD")
	log.Printf("Running dscheduler webui...")
	if err := fcgi.Serve(sock, r); err != nil {
		log.Fatal("Failed to start serving fcgi: ", err)
	}
}
