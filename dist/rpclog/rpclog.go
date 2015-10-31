package rpclog

import (
	"encoding/gob"
	"io"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	_ "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/transport"

	pb "github.com/ThomasHabets/qpov/dist/rpclog/proto"
)

type Logger struct {
	sync.Mutex
	in, out *gob.Encoder
}

func getAddr(ctx context.Context) string {
	t, ok := transport.StreamFromContext(ctx)
	if !ok {
		return ""
	}
	return t.ServerTransport().RemoteAddr().String()
}

func getCN(ctx context.Context) string {
	a, ok := credentials.FromContext(ctx)
	if !ok {
		return ""
	}
	at, ok := a.(credentials.TLSInfo)
	if !ok {
		return ""
	}
	if len(at.State.PeerCertificates) != 1 {
		return ""
	}
	return at.State.PeerCertificates[0].Subject.CommonName
}

func (l *Logger) Log(ctx context.Context, uuid, method string, st time.Time,
	inType string, in proto.Message,
	outErr error, outType string, out proto.Message) {
	var loggingErrors []string
	now := time.Now()
	inm, err := proto.Marshal(in)
	if err != nil {
		loggingErrors = append(loggingErrors, err.Error())
	}
	outm, err := proto.Marshal(out)
	if err != nil {
		loggingErrors = append(loggingErrors, err.Error())
	}
	errstr := ""
	if outErr != nil {
		errstr = outErr.Error()
	}
	e := pb.Entry{
		Uuid: uuid,
		Peer: &pb.Peer{
			Address:    getAddr(ctx),
			CommonName: getCN(ctx),
		},
		Method:  method,
		StartNs: st.UnixNano(),
		EndNs:   now.UnixNano(),
		Request: &pb.Any{
			TypeUrl: inType,
			Value:   inm,
		},
		Error: errstr,
		Response: &pb.Any{
			TypeUrl: outType,
			Value:   outm,
		},
		LoggingError: loggingErrors,
	}
	b, err := proto.Marshal(&e)
	if err != nil {
		grpclog.Fatalf("Failed to marshal RPC log: %v", err)
		return
	}
	l.Lock()
	defer l.Unlock()
	if err := l.out.Encode(b); err != nil {
		grpclog.Fatalf("Failed to log RPC: %v", err)
	}
}

func New(in io.Writer, out io.Writer) *Logger {
	return &Logger{
		in:  gob.NewEncoder(in),
		out: gob.NewEncoder(out),
	}
}

type Reader struct {
	g *gob.Decoder
}

func (r *Reader) Stream() <-chan *pb.Entry {
	ch := make(chan *pb.Entry)
	go func() {
		defer close(ch)
		for {
			var e []byte
			if err := r.g.Decode(&e); err == io.EOF {
				return
			} else if err != nil {
				grpclog.Printf("Broken file: %v", err)
				return
			}
			var e2 pb.Entry
			if err := proto.Unmarshal(e, &e2); err != nil {
				grpclog.Printf("Deserialization fail: %v", err)
			} else {
				ch <- &e2
			}
		}
	}()
	return ch
}

func NewReader(a io.Reader) *Reader {
	return &Reader{
		g: gob.NewDecoder(a),
	}
}
