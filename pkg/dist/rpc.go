package dist

import (
	"os"

	"golang.org/x/net/context"
)

// PerRPC is a magic callback that the gRPC framework calls on every RPC.
// Here it's used to turn `context.Context` "values" into gRPC "Metadata".
// On the other end of the RPC they can later be fetched using `grpcmetadata.FromContext(ctx)`
type PerRPC struct {
	forwardRPCKeys []string
}

func NewPerRPC(forward []string) *PerRPC {
	return &PerRPC{
		forwardRPCKeys: forward,
	}
}

func (*PerRPC) RequireTransportSecurity() bool {
	return true
}

func (p *PerRPC) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	ret := make(map[string]string)
	if h, err := os.Hostname(); err == nil {
		ret["hostname"] = h
	}
	for _, k := range p.forwardRPCKeys {
		t, ok := ctx.Value(k).(string)
		if ok {
			ret[k] = t
		}
	}
	return ret, nil
}
