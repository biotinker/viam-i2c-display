// Package displayapi implements the biotinker:component:display API
package displayapi

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	pb "github.com/biotinker/viam-i2c-display/display/api/proto/component/display/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

// API is the full API definition.
var API = resource.APINamespace("biotinker").WithComponentType("display")

// Named is a helper for getting the named Display's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// FromRobot is a helper for getting the named Display from the given Robot.
func FromRobot(r robot.Robot, name string) (Display, error) {
	return robot.ResourceFromRobot[Display](r, Named(name))
}

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Display]{
		// Reconfigurable, and contents of reconfwrapper.go are only needed for standalone (non-module) uses.
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterDisplayServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.DisplayService_ServiceDesc,
		RPCClient: func(
			ctx context.Context,
			conn rpc.ClientConn,
			remoteName string,
			name resource.Name,
			logger golog.Logger,
		) (Display, error) {
			return NewClientFromConn(conn, remoteName, name, logger), nil
		},
	})
}

// Display defines the Go interface for the component (should match the protobuf methods.)
type Display interface {
	resource.Resource
	DisplayBytes(ctx context.Context, data []byte) error
	WriteString(ctx context.Context, xloc, yloc int, text string) error
	DrawLine(ctx context.Context, x1, y1, x2, y2 int) error
	Reset(ctx context.Context) error
}

// serviceServer implements the Display RPC service from display.proto.
type serviceServer struct {
	pb.UnimplementedDisplayServiceServer
	coll resource.APIResourceCollection[Display]
}

// NewRPCServiceServer returns a new RPC server for the gizmo API.
func NewRPCServiceServer(coll resource.APIResourceCollection[Display]) interface{} {
	return &serviceServer{coll: coll}
}

func (s *serviceServer) DisplayBytes(ctx context.Context, req *pb.DisplayBytesRequest) (*pb.DisplayBytesResponse, error) {
	g, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	err = g.DisplayBytes(ctx, req.Data)
	if err != nil {
		return nil, err
	}
	return &pb.DisplayBytesResponse{}, nil
}

func (s *serviceServer) WriteString(ctx context.Context, req *pb.WriteStringRequest) (*pb.WriteStringResponse, error) {
	g, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	err = g.WriteString(ctx, int(req.Xloc), int(req.Yloc), req.Text)
	if err != nil {
		return nil, err
	}
	return &pb.WriteStringResponse{}, nil
}

func (s *serviceServer) DrawLine(ctx context.Context, req *pb.DrawLineRequest) (*pb.DrawLineResponse, error) {
	g, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	err = g.DrawLine(ctx, int(req.X1), int(req.Y1), int(req.X2), int(req.Y2))
	if err != nil {
		return nil, err
	}
	return &pb.DrawLineResponse{}, nil
}

func (s *serviceServer) Reset(ctx context.Context, req *pb.ResetRequest) (*pb.ResetResponse, error) {
	g, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	err = g.Reset(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ResetResponse{}, nil
}

func (s *serviceServer) DoCommand(ctx context.Context, req *pb.DoCommandRequest) (*pb.DoCommandResponse, error) {
	g, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	resp, err := g.DoCommand(ctx, req.Command.AsMap())
	if err != nil {
		return nil, err
	}
	pbResp, err := protoutils.StructToStructPb(resp)
	if err != nil {
		return nil, err
	}
	return &pb.DoCommandResponse{Result: pbResp}, nil
}

// NewClientFromConn creates a new gizmo RPC client from an existing connection.
func NewClientFromConn(conn rpc.ClientConn, remoteName string, name resource.Name, logger golog.Logger) Display {
	sc := newSvcClientFromConn(conn, remoteName, name, logger)
	return clientFromSvcClient(sc, name.ShortName())
}

func newSvcClientFromConn(conn rpc.ClientConn, remoteName string, name resource.Name, logger golog.Logger) *serviceClient {
	client := pb.NewDisplayServiceClient(conn)
	sc := &serviceClient{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		client: client,
		logger: logger,
	}
	return sc
}

type serviceClient struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	client pb.DisplayServiceClient
	logger golog.Logger
}

// client is an gripper client.
type client struct {
	*serviceClient
	name string
}

func clientFromSvcClient(sc *serviceClient, name string) Display {
	return &client{sc, name}
}

func (c *client) DisplayBytes(ctx context.Context, data []byte) error {
	_, err := c.client.DisplayBytes(ctx, &pb.DisplayBytesRequest{
		Name: c.name,
		Data: data,
	})
	if err != nil {
		return err
	}
	return nil
}
func (c *client) WriteString(ctx context.Context, xloc, yloc int, text string) error {
	_, err := c.client.WriteString(ctx, &pb.WriteStringRequest{
		Name: c.name,
		Xloc: int32(xloc),
		Yloc: int32(yloc),
		Text: text,
	})
	if err != nil {
		return err
	}
	return nil
}
func (c *client) DrawLine(ctx context.Context, x1, y1, x2, y2 int) error {
	_, err := c.client.DrawLine(ctx, &pb.DrawLineRequest{
		Name: c.name,
		X1: int32(x1),
		Y1: int32(y1),
		X2: int32(x2),
		Y2: int32(y2),
	})
	if err != nil {
		return err
	}
	return nil
}
func (c *client) Reset(ctx context.Context) error {
	_, err := c.client.Reset(ctx, &pb.ResetRequest{
		Name: c.name,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	command, err := protoutils.StructToStructPb(cmd)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.DoCommand(ctx, &pb.DoCommandRequest{
		Name:    c.name,
		Command: command,
	})
	if err != nil {
		return nil, err
	}
	return resp.Result.AsMap(), nil
}
