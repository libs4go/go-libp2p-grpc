package grpc

import (
	"context"
	"net"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libs4go/errors"
	"github.com/libs4go/smf4go/service/grpcservice"
	"google.golang.org/grpc"
)

const protocol = "/grpc/0.0.1"

// ScopeOfAPIError .
const errVendor = "pgrpc"

// errors
var (
	ErrInternal = errors.New("the internal error", errors.WithVendor(errVendor), errors.WithCode(-1))
	ErrAddr     = errors.New("invalid libp2p net.addr", errors.WithVendor(errVendor), errors.WithCode(-2))
	ErrClosed   = errors.New("transport closed", errors.WithVendor(errVendor), errors.WithCode(-3))
)

// Transport the grpc transport over libp2p
type Transport grpcservice.Provider

type libp2pTransport struct {
	ctx    context.Context
	cancel context.CancelFunc
	host   host.Host
	in     chan net.Conn
}

// Option transport create option
type Option func(transport *libp2pTransport)

// New create transport object for grpc over libp2p
func New(ctx context.Context, host host.Host, options ...Option) Transport {

	newCtx, cancel := context.WithCancel(ctx)

	transport := &libp2pTransport{
		ctx:    newCtx,
		cancel: cancel,
		host:   host,
		in:     make(chan net.Conn),
	}

	host.SetStreamHandler(protocol, transport.streamHandler)

	return transport
}

func (t *libp2pTransport) Accept() (net.Conn, error) {

	conn, ok := <-t.in

	if !ok {
		return nil, ErrClosed
	}

	return conn, nil
}

func (t *libp2pTransport) Close() error {
	t.cancel()

	return nil
}

func (t *libp2pTransport) Addr() net.Addr {
	return &libp2pAddr{t.host.ID()}
}

func (t *libp2pTransport) streamHandler(stream network.Stream) {

	select {
	case <-t.ctx.Done():
		return
	case t.in <- &libp2pConn{stream}:
	}
}

func (t *libp2pTransport) Listener() net.Listener {
	return t
}

func (t *libp2pTransport) Connect(ctx context.Context, remote string) (net.Conn, error) {
	id, err := peer.Decode(remote)

	if err != nil {
		return nil, errors.Wrap(err, "parse peer.ID error %s", remote)
	}

	stream, err := t.host.NewStream(t.ctx, id, protocol)

	if err != nil {
		return nil, errors.Wrap(err, "create peer.ID %s stream error", remote)
	}

	return &libp2pConn{Stream: stream}, nil
}

type libp2pAddr struct {
	peer.ID
}

func (addr *libp2pAddr) Network() string {
	return "libp2p"
}

func (addr *libp2pAddr) String() string {
	return addr.ID.Pretty()
}

type libp2pConn struct {
	network.Stream
}

func (conn *libp2pConn) LocalAddr() net.Addr {
	return &libp2pAddr{conn.Conn().LocalPeer()}
}

func (conn *libp2pConn) RemoteAddr() net.Addr {
	return &libp2pAddr{conn.Conn().RemotePeer()}
}

// PeerID cast net.Addr to peer.ID
func PeerID(addr net.Addr) (peer.ID, error) {
	if addr.Network() != "libp2p" {
		return "", errors.Wrap(ErrAddr, "invalid libp2p addr %s:%s ", addr.Network(), addr.String())
	}

	id, err := peer.Decode(addr.String())

	if err != nil {
		return "", errors.Wrap(err, "invalid libp2p addr %s:%s ", addr.Network(), addr.String())
	}

	return id, nil
}

// Serve create and run grpc.Server with libp2p transport
func Serve(transport Transport, options ...grpc.ServerOption) *grpc.Server {
	server := grpc.NewServer(options...)

	go server.Serve(transport.Listener())

	return server
}

func (t *libp2pTransport) dialOption(ctx context.Context) grpc.DialOption {
	return grpc.WithDialer(func(remote string, timeout time.Duration) (net.Conn, error) {
		id, err := peer.Decode(remote)

		if err != nil {
			return nil, errors.Wrap(err, "parse peer.ID error %s", remote)
		}

		stream, err := t.host.NewStream(t.ctx, id, protocol)

		if err != nil {
			return nil, errors.Wrap(err, "create peer.ID %s stream error", remote)
		}

		return &libp2pConn{Stream: stream}, nil
	})
}

// Dial connect to grpc Server with libp2p transport
func Dial(transport Transport, id peer.ID, dialOpts ...grpc.DialOption) (*grpc.ClientConn, error) {

	t := (transport).(*libp2pTransport)

	dialOpsPrepended := append([]grpc.DialOption{t.dialOption(t.ctx), grpc.WithInsecure()}, dialOpts...)

	return grpc.DialContext(t.ctx, id.Pretty(), dialOpsPrepended...)
}
