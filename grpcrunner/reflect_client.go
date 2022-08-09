package grpcrunner

import (
	"context"

	"google.golang.org/grpc"
)

type TRPCClientConn struct {
	conn *grpc.ClientConn
	path string
}

func RefClientConnFromConn(conn *grpc.ClientConn, path string) grpc.ClientConnInterface {
	return &TRPCClientConn{
		conn: conn,
		path: path,
	}
}

const errCloseWithoutTrailers = "server closed the stream without sending trailers"
const errCloseWithoutTrailersWithDesc = "rpc error: code = Internal desc = server closed the stream without sending trailers"

func (cc *TRPCClientConn) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	err := cc.conn.Invoke(ctx, cc.path+method, args, reply, opts...)
	if err != nil && err.Error() != errCloseWithoutTrailersWithDesc {
		return err
	}
	return nil
}

// impl ClientConnInterface
func (cc *TRPCClientConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	println("new stream called for ", cc.path+method)
	cs, err := cc.conn.NewStream(ctx, desc, cc.path+method, opts...)
	println("new stream called for ", cc.path+method, " done")
	if err != nil {
		println("new stream called for ", cc.path+method, " err: ", err.Error())
	}
	// ToDo: maybe https://github.com/kubernetes/ingress-nginx/issues/2963
	if err != nil && (err.Error() != errCloseWithoutTrailers || err.Error() != errCloseWithoutTrailersWithDesc) {
		return nil, err
	}
	println("new stream called for ", cc.path+method, " retrieving")
	return cs, nil
}

func (cc *TRPCClientConn) Close() error {
	return cc.conn.Close()
}
