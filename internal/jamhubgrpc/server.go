package jamhubgrpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"embed"
	"log"
	"net"
	"path/filepath"

	"github.com/zdgeier/jamhub/gen/pb"
	"github.com/zdgeier/jamhub/internal/jamenv"
	"github.com/zdgeier/jamhub/internal/jamhub/changestore"
	"github.com/zdgeier/jamhub/internal/jamhub/db"
	"github.com/zdgeier/jamhub/internal/jamhub/opdatastorebranch"
	"github.com/zdgeier/jamhub/internal/jamhub/opdatastorecommit"
	"github.com/zdgeier/jamhub/internal/jamhub/oplocstorebranch"
	"github.com/zdgeier/jamhub/internal/jamhub/oplocstorecommit"
	"github.com/zdgeier/jamhub/internal/jamhubgrpc/serverauth"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/reflection"
)

//go:embed clientkey.pem
var prodF embed.FS

//go:embed devclientkey.cer
var devF embed.FS

type JamHub struct {
	db                db.JamHubDb
	opdatastorebranch *opdatastorebranch.LocalStore
	opdatastorecommit *opdatastorecommit.LocalStore
	oplocstorebranch  *oplocstorebranch.LocalOpLocStore
	oplocstorecommit  *oplocstorecommit.LocalOpLocStore
	changestore       changestore.LocalChangeStore
	pb.UnimplementedJamHubServer
}

func New() (closer func(), err error) {
	jamhub := JamHub{
		db:                db.New(),
		opdatastorebranch: opdatastorebranch.NewOpDataStoreBranch(),
		opdatastorecommit: opdatastorecommit.NewOpDataStoreCommit(),
		oplocstorebranch:  oplocstorebranch.NewOpLocStoreBranch(),
		oplocstorecommit:  oplocstorecommit.NewOpLocStoreCommit(),
		changestore:       changestore.NewLocalChangeStore(),
	}

	var cert tls.Certificate
	if jamenv.Env() == jamenv.Prod {
		cert, err = tls.LoadX509KeyPair("/etc/jamhub/fullchain.pem", "/etc/jamhub/privkey.pem")
	} else {
		cert, err = tls.LoadX509KeyPair(filepath.Clean("x509/publickey.cer"), filepath.Clean("x509/private.key"))
	}
	if err != nil {
		return nil, err
	}

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(serverauth.EnsureValidToken),
		grpc.Creds(credentials.NewServerTLSFromCert(&cert)),
	}

	server := grpc.NewServer(opts...)
	reflection.Register(server)
	pb.RegisterJamHubServer(server, jamhub)

	tcplis, err := net.Listen("tcp", "0.0.0.0:14357")
	if err != nil {
		return nil, err
	}
	go func() {
		if err := server.Serve(tcplis); err != nil {
			log.Printf("error serving server: %v", err)
		}
	}()

	return func() { server.Stop() }, nil
}

func Connect(accessToken *oauth2.Token) (client pb.JamHubClient, closer func(), err error) {
	opts := []grpc.DialOption{
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			raddr, err := net.ResolveTCPAddr("tcp", addr)
			if err != nil {
				return nil, err
			}

			conn, err := net.DialTCP("tcp", nil, raddr)
			if err != nil {
				return nil, err
			}

			return conn, err
		}),
	}
	if jamenv.Env() != jamenv.Local {
		perRPC := oauth.TokenSource{TokenSource: oauth2.StaticTokenSource(accessToken)}
		opts = append(opts, grpc.WithPerRPCCredentials(perRPC))
	}
	var creds credentials.TransportCredentials
	if jamenv.Env() == jamenv.Prod {
		cp := x509.NewCertPool()
		certData, err := prodF.ReadFile("clientkey.pem")
		if err != nil {
			return nil, nil, err
		}
		cp.AppendCertsFromPEM(certData)
		creds = credentials.NewClientTLSFromCert(cp, "jamhub.dev")
	} else {
		cp := x509.NewCertPool()
		certData, err := devF.ReadFile("devclientkey.cer")
		if err != nil {
			return nil, nil, err
		}
		cp.AppendCertsFromPEM(certData)
		creds = credentials.NewClientTLSFromCert(cp, "jamhub.dev")
	}
	opts = append(opts, grpc.WithTransportCredentials(creds))

	addr := "0.0.0.0:14357"

	if jamenv.Env() == jamenv.Prod {
		addr = "prod.jamhub.dev:14357"
	}
	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		log.Panicf("could not connect to jamhub server: %s", err)
	}
	client = pb.NewJamHubClient(conn)
	closer = func() {
		if err := conn.Close(); err != nil {
			log.Panic("could not close server connection")
		}
	}

	return client, closer, err
}
