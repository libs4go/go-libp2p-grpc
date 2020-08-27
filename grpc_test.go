package grpc

import (
	"context"
	"fmt"
	"testing"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/libs4go/go-libp2p-grpc/pro"
	"github.com/stretchr/testify/require"
)

//go:generate protoc --proto_path=./pro --go_out=plugins=grpc,paths=source_relative:./pro echo.proto

func makeHost(port int) (host.Host, error) {
	prikey, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 2048)

	if err != nil {
		return nil, err
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port)),
		libp2p.Identity(prikey),
		libp2p.DisableRelay(),
	}

	return libp2p.New(context.Background(), opts...)
}

func TestAddr(t *testing.T) {
	_, pubkey, err := crypto.GenerateKeyPair(crypto.Ed25519, 2048)

	require.NoError(t, err)

	id, err := peer.IDFromPublicKey(pubkey)

	require.NoError(t, err)

	addr := &libp2pAddr{id}

	id2, err := PeerID(addr)

	require.NoError(t, err)

	require.Equal(t, id, id2)
}

type echoServer struct {
}

func (s *echoServer) Say(ctx context.Context, request *pro.Request) (*pro.Response, error) {
	return &pro.Response{Message: request.Message}, nil
}

func TestEcho(t *testing.T) {
	h1, err := makeHost(1812)

	require.NoError(t, err)

	h2, err := makeHost(1813)

	require.NoError(t, err)

	h2.Peerstore().AddAddr(h1.ID(), h1.Addrs()[0], peerstore.PermanentAddrTTL)

	t1 := New(context.Background(), h1)

	t2 := New(context.Background(), h2)

	s1 := Serve(t1)

	pro.RegisterEchoServer(s1, &echoServer{})

	conn, err := Dial(t2, h1.ID())

	require.NoError(t, err)

	client := pro.NewEchoClient(conn)

	resp, err := client.Say(context.Background(), &pro.Request{Message: "hello1"})

	require.NoError(t, err)

	require.Equal(t, "hello1", resp.Message)
}
