package proxyservice

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	rpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type Service struct {
	Addr       string
	Name       string // 服务注册名
	Conn       *grpc.ClientConn
	InvokeList []*Invoke
}

type InvokeTarget struct {
	Host     string
	Srv      string
	Function string
}

func NewInvokeTarget(h, s, f string) *InvokeTarget {
	return &InvokeTarget{Host: h, Srv: s, Function: f}
}

var XO *ServiceSlice

// 理论要结合框架做优雅启动，并且提供更新机制
// 缺少更新机制
func init() {
	XO = NewServiceInfo()
}

type ServiceSlice struct {
	list []*Service
	m    map[string]map[string]map[string]*Method
	c    map[string]*grpc.ClientConn
	once *sync.Once
	lock *sync.RWMutex
}

func (s *ServiceSlice) ListServices() []*Service {
	s.lock.RLock()
	x := s.list
	s.lock.RUnlock()
	return x
}

func (s *ServiceSlice) Client(host string) *grpc.ClientConn {
	s.lock.RLock()
	defer s.lock.RUnlock()
	v := s.c[host]
	return v
}

func onceDo(list []*Service) (map[string]map[string]map[string]*Method, map[string]*grpc.ClientConn) {
	// 这里用ip+port区分
	//key1:ip+port
	//key2:service
	//key3: function name
	var (
		m = make(map[string]map[string]map[string]*Method)
		c = make(map[string]*grpc.ClientConn)
	)

	for _, v1 := range list {
		if _, ok := c[v1.Addr]; !ok {
			c[v1.Addr] = v1.Conn
		}
		if _, ok := m[v1.Addr]; !ok {
			m[v1.Addr] = make(map[string]map[string]*Method)
		}
		for _, v2 := range v1.InvokeList {
			if _, ok := m[v1.Addr][v2.Name]; !ok {
				m[v1.Addr][v2.Name] = make(map[string]*Method)
			}
			for _, v3 := range v2.MethodList {
				m[v1.Addr][v2.Name][v3.Func] = v3
			}
		}
	}
	return m, c
}

func (s *ServiceSlice) ByInvoke(i *InvokeTarget) *Method {
	s.lock.RLock()
	defer s.lock.RUnlock()

	s.once.Do(func() {
		s.m, s.c = onceDo(s.list)
	})

	v4, ok := s.m[i.Host]
	if !ok {
		return nil
	}
	v5, ok := v4[i.Srv]
	if !ok {
		return nil
	}
	v6, ok := v5[i.Function]
	if !ok {
		return nil
	}
	return v6
}

type Invoke struct {
	Name       string
	MethodList []*Method
}

type Method struct {
	Func     string
	Args     protoreflect.MessageDescriptor
	Reply    protoreflect.MessageDescriptor
	FullName string
}

func NewServiceInfo() *ServiceSlice {
	var result = &ServiceSlice{lock: &sync.RWMutex{}, once: &sync.Once{}}
	var ctx = context.Background()

	// 注册中心或者配置平台拉取需要代理的服务
	for _, h := range []string{"127.0.0.1:8888"} {
		s, err := getServiceInfo(ctx, h)
		if err != nil {
			fmt.Printf("get srv info err %s\n", err)
			continue
		}
		result.list = append(result.list, s)
	}
	return result
}

func getServiceInfo(ctx context.Context, addr string) (*Service, error) {
	// 流式调用，应该用IP（待确定
	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	// don't need
	// defer conn.Close()
	c := rpb.NewServerReflectionClient(conn)
	stream, err := c.ServerReflectionInfo(ctx, grpc.WaitForReady(true))
	if err != nil {
		return nil, err
	}
	err = stream.Send(&rpb.ServerReflectionRequest{
		MessageRequest: &rpb.ServerReflectionRequest_ListServices{},
	})
	if err != nil {
		return nil, err
	}
	r1, err := stream.Recv()
	if err != nil {
		return nil, err
	}
	var srvInfo = &Service{Addr: addr, Conn: conn}

	for _, s := range r1.GetListServicesResponse().GetService() {
		if s.GetName() == "grpc.health.v1.Health" || s.GetName() == "grpc.reflection.v1alpha.ServerReflection" {
			continue
		}
		err = stream.Send(&rpb.ServerReflectionRequest{
			MessageRequest: &rpb.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: s.GetName(),
			},
		})
		if err != nil {
			return nil, err
		}
		r2, err := stream.Recv()
		if err != nil {
			return nil, err
		}

		for _, d := range r2.GetFileDescriptorResponse().GetFileDescriptorProto() {
			pb := &descriptorpb.FileDescriptorProto{}
			if err := proto.Unmarshal(d, pb); err != nil {
				return nil, err
			}
			fd, err := protodesc.NewFile(pb, nil)
			if err != nil {
				return nil, err
			}
			srv := fd.Services()
			for i, j := 0, srv.Len(); i < j; i++ {
				info := srv.Get(i)
				invoke := &Invoke{Name: string(info.Name())}
				for x, y := 0, info.Methods().Len(); x < y; x++ {
					m := info.Methods().Get(x)
					invoke.MethodList = append(invoke.MethodList, &Method{
						Func:     string(m.Name()),
						Args:     m.Input(),
						Reply:    m.Output(),
						FullName: fmt.Sprintf("/%s/%s", info.FullName(), m.Name()),
					})
				}
				srvInfo.InvokeList = append(srvInfo.InvokeList, invoke)
			}
		}
	}
	return srvInfo, nil
}
