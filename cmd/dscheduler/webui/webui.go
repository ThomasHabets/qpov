package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/fcgi"
	"os"
	"path"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"

	"github.com/ThomasHabets/qpov/pkg/dist"
	pb "github.com/ThomasHabets/qpov/pkg/dist/qpov"
)

const (
	userAgent = "dscheduler-webui"

	leaseTmpl = `
<h1>Lease {{.Lease.LeaseId}}</h1>
<pre>{{.Ascii}}</pre>
`

	orderTmpl = `
{{range .Order}}
<h1>Order {{.OrderId}}</h1>
{{end}}
<pre>{{.Ascii}}</pre>
`
)

var (
	pageDeadline  = flag.Duration("page_deadline", time.Second, "Page timeout.")
	socketPath    = flag.String("socket", "", "Unix socket to listen to.")
	caFile        = flag.String("ca_file", "", "Server CA file.")
	certFile      = flag.String("cert_file", "", "Client cert file.")
	keyFile       = flag.String("key_file", "", "Client key file.")
	schedAddr     = flag.String("scheduler", "", "Scheduler address.")
	root          = flag.String("root", "", "Path under root of domain that the web UI runs.")
	oauthClientID = flag.String("oauth_client_id", "", "Google OAuth Client ID.")

	// TODO: move to GCS?
	statsDir = flag.String("stats_dir", "", "Dir containing stats data.")

	sched         pb.SchedulerClient
	cookieMonster pb.CookieMonsterClient

	//go:embed root.html
	rootTmpl string
	tmplRoot template.Template

	//go:embed batch.html
	batchTmpl string

	//go:embed design.html
	tmplDesignString string
	tmplDesign       = template.Must(template.New("design").Parse(tmplDesignString))

	//go:embed stats.html
	tmplStatsString string
	tmplStats       *template.Template

	//go:embed done.html
	doneTmpl string

	//go:embed join.html
	joinTmpl string

	forwardRPCKeys = []string{"id", "source", "http.remote_addr", "http.cookie"}
)

func init() {
	tmplStats = template.New("stats")
	tmplStats.Funcs(dist.TmplStatsFuncs)
	template.Must(tmplStats.Parse(tmplStatsString))
}

func httpContext(r *http.Request) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "source", "http")
	ctx = context.WithValue(ctx, "id", uuid.New().String())
	ctx = context.WithValue(ctx, "http.remote_addr", r.RemoteAddr)
	if c, err := r.Cookie("qpov"); err == nil {
		ctx = context.WithValue(ctx, "http.cookie", c.Value)
	}
	return ctx
}

// Format milliseconds as a date using format string `s`.
func fmsdate(s string, ms int64) string {
	return time.Unix(ms/1000, 0).UTC().Format(s)
}

// take two milliseconds, subtract them as time.Duration and return as string.
func fmssub(a, b int64) string {
	return dist.FmtSecondDuration((a - b) / 1000)
}

// time from now until `ms`.
func fmsuntil(ms int64) string {
	return dist.FmtSecondDuration((ms - time.Now().UnixNano()/1000000) / 1000)
}

func fmssince(ms int64) string {
	return dist.FmtSecondDuration((time.Now().UnixNano()/1000000 - ms) / 1000)
}

func getLeases(ctx context.Context, done bool) ([]*pb.Lease, error) {
	stream, err := sched.Leases(ctx, &pb.LeasesRequest{
		Done:  done,
		Order: true,
		Since: time.Now().Add(-24 * 7 * time.Hour).Unix(),
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
		if r.Lease.GetHostname() == "" {
			r.Lease.Hostname = r.Lease.GetMetadata().GetHostname()
		}
		leases = append(leases, r.Lease)
	}
	return leases, nil
}

func rpcErrorToHTTPError(err error) int {
	m := map[codes.Code]int{
		codes.NotFound:        http.StatusNotFound,
		codes.Unauthenticated: http.StatusUnauthorized,
	}
	n, found := m[grpc.Code(err)]
	if !found {
		log.Errorf("Unmapped grpc code %v", grpc.Code(err))
		return http.StatusInternalServerError
	}
	return n
}

func handleCert(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(httpContext(r), time.Minute)
	defer cancel()

	// Cookie is attached to context.
	resp, err := sched.Certificate(ctx, &pb.CertificateRequest{})
	if err != nil {
		log.Errorf("Certificate: %v", err)
		w.WriteHeader(rpcErrorToHTTPError(err))
		if _, err := fmt.Fprintf(w, "Error: %v", err); err != nil {
			log.Errorf("Also failed to write that to client: %v", err)
		}
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	if _, err := w.Write(resp.Pem); err != nil {
		log.Errorf("Failed to write pem data: %v", err)
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	log.Infof("handleLogin() called")
	ctx, cancel := context.WithTimeout(httpContext(r), time.Minute)
	defer cancel()

	j := r.PostFormValue("jwt")

	// Reuse old cookie, if available.
	var q string
	if c, err := r.Cookie("qpov"); err == nil {
		q = c.Value
	}

	resp, err := cookieMonster.Login(ctx, &pb.LoginRequest{Jwt: j, Cookie: q})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorf("Login: failed to RPC: %v", err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "qpov",
		Value:    resp.Cookie,
		HttpOnly: true,
		MaxAge:   30 * 86400,
		Secure:   true,
	})
	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write([]byte("OK\n")); err != nil {
		log.Errorf("Login: failed to write: %v", err)
	}
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(httpContext(r), time.Minute)
	defer cancel()

	var q string
	if c, err := r.Cookie("qpov"); err == nil {
		q = c.Value
	}

	if _, err := cookieMonster.Logout(ctx, &pb.LogoutRequest{Cookie: q}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorf("Logout: failed to RPC: %v", err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "qpov",
		Value:    "",
		HttpOnly: true,
		MaxAge:   -1,
		Secure:   true,
	})
	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write([]byte("OK\n")); err != nil {
		log.Warningf("Logout: failed to write: %v", err)
	}
}

func handleImage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(httpContext(r), time.Minute)
	defer cancel()

	lease, ok := mux.Vars(r)["leaseID"]
	if !ok {
		log.Errorf("Internal error: leaseID not passed in to handleImage")
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

	// TODO: This is needlessly complicated, and it should just provide a buffered writer to write to.
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
				log.Warningf("Failed streaming result to client: %v", err)
				return
			}
		}
	}()

	func() {
		defer close(ch)
		sentAnything := false
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
				if !sentAnything {
					// Still time to make an error page.
					w.Header().Set("Content-Type", "text/plain")
					w.WriteHeader(rpcErrorToHTTPError(err))
					fmt.Fprintf(w, "Failed to stream image data: %v\n", grpc.ErrorDesc(err))
				}
				log.Errorf("Failed streaming result over RPC: %v", err)
				return
			}
			// Only sent on first packet.
			if r.ContentType != "" {
				w.Header().Set("Content-Type", r.ContentType)
			}
			if len(r.Data) > 0 {
				sentAnything = true
				ch <- r.Data
			}
		}
	}()
	<-writerDone
}

func handleLease(ctx context.Context, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	ctx, cancel := context.WithTimeout(httpContext(r), 10*time.Second)
	defer cancel()

	// Get lease.
	lease, ok := mux.Vars(r)["leaseID"]
	if !ok {
		log.Warningf("leaseID not passed in to handleLease")
		return nil, fmt.Errorf("no lease provided")
	}
	reply, err := sched.Lease(ctx, &pb.LeaseRequest{LeaseId: lease})
	if err != nil {
		switch grpc.Code(err) {
		case codes.InvalidArgument:
			msg := fmt.Sprintf("Bad request: %v", grpc.ErrorDesc(err))
			return nil, httpError(http.StatusBadRequest, msg, msg)
		case codes.Unauthenticated:
			return nil, httpError(http.StatusForbidden, "Unauthenticated", fmt.Sprintf("Unauthenticated: %v", err))
		case codes.NotFound:
			msg := fmt.Sprintf("Lease %q not found", lease)
			return nil, httpError(http.StatusNotFound, msg, msg)
		default:
			log.Errorf("Backend call: %v", err)
			return nil, fmt.Errorf("backend broke :-(")
		}
	}
	return &struct {
		Root  string
		Lease *pb.Lease
		Ascii string
	}{
		Root:  *root,
		Lease: reply.Lease,
		Ascii: proto.MarshalTextString(reply.Lease),
	}, nil
}

func readStatsOverall() (*pb.StatsOverall, error) {
	f, err := os.Open(path.Join(*statsDir, "overall.pb"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	stats := &pb.StatsOverall{}
	if err := proto.Unmarshal(b, stats); err != nil {
		return nil, err
	}
	return stats, nil
}

func handleStats(ctx context.Context, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	stats, err := readStatsOverall()
	if err != nil {
		return nil, err
	}
	return &struct {
		Stats *pb.StatsOverall
		Root  *string
	}{
		Stats: stats,
		Root:  root,
	}, nil
}

func handleRoot(ctx context.Context, w http.ResponseWriter, r *http.Request) (interface{}, error) {
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

	wg.Wait()
	if len(errs) > 0 {
		log.Errorf("Errors: %v", errs)
	}
	statsOverall, err := readStatsOverall()
	if err != nil {
		log.Errorf("Failed to read overall stats: %v", err)
		statsOverall = &pb.StatsOverall{}
	}
	ret := &struct {
		Root            string
		Stats           *pb.StatsReply
		Leases          []*pb.Lease
		UnstartedOrders int64
		Errors          []error
		StatsOverall    *pb.StatsOverall
	}{
		Root:         *root,
		Stats:        st,
		Leases:       leases,
		Errors:       errs,
		StatsOverall: statsOverall,
	}
	if st != nil {
		ret.UnstartedOrders = st.SchedulingStats.Orders - st.SchedulingStats.DoneOrders
	}
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	return ret, nil
}

func handleJoin(ctx context.Context, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return nil, nil
}

func handleOrder(ctx context.Context, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	ctx, cancel := context.WithTimeout(httpContext(r), 10*time.Second)
	defer cancel()

	// Get order.
	orderID, err := getUUID(r, "orderID")
	if err != nil {
		log.Warningf("orderID not passed in to handleOrder")
		return nil, err
	}
	reply, err := sched.Order(ctx, &pb.OrderRequest{OrderId: []string{orderID.String()}})
	if err != nil {
		return nil, err
	}
	return &struct {
		Root  string
		Order []*pb.Order
		Ascii string
	}{
		Root:  *root,
		Order: reply.Order,
		Ascii: proto.MarshalTextString(reply),
	}, nil
}

func getUUID(r *http.Request, key string) (*uuid.UUID, error) {
	v, ok := mux.Vars(r)[key]
	if !ok {
		return nil, fmt.Errorf("key %q not found in request", key)
	}
	u, err := uuid.Parse(v)
	return &u, err
}

func handleBatch(ctx context.Context, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	ctx, cancel := context.WithTimeout(httpContext(r), 10*time.Second)
	defer cancel()

	batchID, err := getUUID(r, "batchID")
	if err != nil {
		return nil, err
	}

	return &struct {
		Root    string
		BatchID *uuid.UUID
	}{
		Root:    *root,
		BatchID: batchID,
	}, nil
}

func handleDone(ctx context.Context, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var errs []error

	// Get done Leases.
	var doneLeases []*pb.Lease
	{
		var err error
		doneLeases, err = getLeases(ctx, true)
		if err != nil {
			log.Errorf("Error: %v", err)
			errs = append(errs, fmt.Errorf("getLeases(true): %v", err))
		}
	}
	ret := &struct {
		Root       string
		DoneLeases []*pb.Lease
		Errors     []error
	}{
		Root:       *root,
		DoneLeases: doneLeases,
		Errors:     errs,
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

func getLoggedIn(ctx context.Context) bool {
	c, ok := ctx.Value("http.cookie").(string)
	if !ok {
		return false
	}
	_, err := cookieMonster.CheckCookie(ctx, &pb.CheckCookieRequest{Cookie: c})
	if err != nil {
		log.Warningf("Cookie invalid: %v", err)
		return false
	}
	return true
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	w.Header().Set("Strict-Transport-Security", "max-age=2592000")
	ctx, cancel := context.WithTimeout(httpContext(r), *pageDeadline)
	defer cancel()
	data, err := h.f(ctx, w, r)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		code := http.StatusInternalServerError
		msg := "Internal error"
		if grpc.Code(err) != codes.Unknown {
			code = rpcErrorToHTTPError(err)
			msg = err.Error()
		}
		e2, ok := err.(*httpErr)
		if ok {
			code = e2.code
			msg = e2.public
		}
		w.WriteHeader(code)
		fmt.Fprintf(w, "Error: %v", msg)
		log.Errorf("Error rendering page: %v", err)
		return
	}
	var buf bytes.Buffer
	if err := h.tmpl.Execute(&buf, data); err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorf("Template rendering failed: %v", err)
		fmt.Fprintf(w, "Internal error: Failed to render page")
		return
	}

	var buf2 bytes.Buffer
	if err := tmplDesign.Execute(&buf2, &struct {
		OAuthClientID string
		LoggedIn      bool
		Root          string
		Errors        []string
		Content       template.HTML
		PageTime      time.Duration
	}{
		LoggedIn:      getLoggedIn(ctx),
		OAuthClientID: *oauthClientID,
		Root:          *root,
		Content:       template.HTML(buf.String()),
		PageTime:      time.Since(startTime),
	}); err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorf("Template rendering failed: %v", err)
		fmt.Fprintf(w, "Internal error: Failed to render page")
		return
	}
	if r.Method != "HEAD" {
		if _, err := w.Write(buf2.Bytes()); err != nil {
			log.Warningf("Failed to write page to network: %v", err)
			return
		}
	}
}

func wrap(f handleFunc, t string) *handler {
	tmpl := template.New("blah")
	tmpl.Funcs(template.FuncMap{
		"fmtpercent":     func(a, b int64) string { return fmt.Sprintf("%.2f", 100.0*float64(a)/float64(b)) },
		"fsdate":         func(s string, n int64) string { return fmsdate(s, n*1000) },
		"sumcpu":         func(c *pb.StatsCPUTime) int64 { return c.UserSeconds + c.SystemSeconds },
		"seconds2string": func(s int64) string { return dist.FmtSecondDuration(s) },
		"tailchar": func(i int, s string) string {
			if len(s) < i {
				return s
			}
			return s[len(s)-i:]
		},
		"fmsdate":  fmsdate,
		"fmsuntil": fmsuntil,
		"fmssince": fmssince,
		"fmssub":   fmssub,
		"fileonly": path.Base,
	})
	template.Must(tmpl.Parse(t))
	return wrapTmpl(f, tmpl)
}

func wrapTmpl(f handleFunc, tmpl *template.Template) *handler {
	return &handler{f: f, tmpl: tmpl}
}

func connectScheduler(addr string) error {
	var cp *x509.CertPool
	if *caFile != "" {
		var caStr string
		// caStr := dist.CacertClass1
		b, err := ioutil.ReadFile(*caFile)
		if err != nil {
			return fmt.Errorf("reading %q: %v", *caFile, err)
		}
		caStr = string(b)

		// Root CA.
		cp := x509.NewCertPool()
		if ok := cp.AppendCertsFromPEM([]byte(caStr)); !ok {
			return fmt.Errorf("failed to add root CAs")
		}
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

	// Connect to scheduler.
	{
		cr := credentials.NewTLS(&tlsConfig)
		conn, err := grpc.Dial(addr,
			grpc.WithTransportCredentials(cr),
			grpc.WithPerRPCCredentials(dist.NewPerRPC(forwardRPCKeys)),
			grpc.WithUserAgent(userAgent),
		)
		if err != nil {
			return fmt.Errorf("dialing scheduler %q: %v", addr, err)
		}
		sched = pb.NewSchedulerClient(conn)
		cookieMonster = pb.NewCookieMonsterClient(conn)
	}
	return nil
}

func main() {
	flag.Parse()
	if flag.NArg() > 0 {
		log.Fatalf("Got extra args on cmdline: %q", flag.Args())
	}

	if err := connectScheduler(*schedAddr); err != nil {
		log.Fatalf("Failed to connect to scheduler: %v", err)
	}

	os.Remove(*socketPath)

	sock, err := net.Listen("unix", *socketPath)
	if err != nil {
		log.Fatalf("Unable to listen to socket: %v", err)
	}
	if err = os.Chmod(*socketPath, 0666); err != nil {
		log.Fatalf("Unable to chmod socket: %v", err)
	}

	if *root != "" {
		if (*root)[0] != '/' {
			log.Fatalf("-root must be empty or begin with slash")
		}
		if (*root)[len(*root)-1] == '/' {
			log.Fatalf("-root must not end with slash")
		}
	}

	r := mux.NewRouter()
	r.Handle(*root+"/", wrap(handleRoot, rootTmpl)).Methods("GET", "HEAD")
	r.Handle(path.Join("/", *root, "done"), wrap(handleDone, doneTmpl)).Methods("GET", "HEAD")
	r.Handle(path.Join("/", *root, "join"), wrap(handleJoin, joinTmpl)).Methods("GET", "HEAD")
	r.HandleFunc(path.Join("/", *root, "image/{leaseID}"), handleImage).Methods("GET", "HEAD")
	r.Handle(path.Join("/", *root, "order/{orderID}"), wrap(handleOrder, orderTmpl)).Methods("GET", "HEAD")
	r.Handle(path.Join("/", *root, "batch/{batchID}"), wrap(handleBatch, batchTmpl)).Methods("GET", "HEAD")
	r.Handle(path.Join("/", *root, "lease/{leaseID}"), wrap(handleLease, leaseTmpl)).Methods("GET", "HEAD")
	r.Handle(path.Join("/", *root, "stats"), wrapTmpl(handleStats, tmplStats)).Methods("GET", "HEAD")
	r.HandleFunc(path.Join("/", *root, "cert"), handleCert).Methods("GET", "HEAD")
	r.HandleFunc(path.Join("/", *root, "login"), handleLogin).Methods("POST")
	r.HandleFunc(path.Join("/", *root, "logout"), handleLogout).Methods("POST")
	r.Handle(path.Join("/", *root, "stats/{blaha}"), http.FileServer(http.Dir(*statsDir))).Methods("GET", "HEAD")
	log.Infof("Running dscheduler webui...")
	if err := fcgi.Serve(sock, r); err != nil {
		log.Fatal("Failed to start serving fcgi: ", err)
	}
}
