package v1beta1

import (
	"context"
	"fmt"
	nethttp "net/http"

	"github.com/project-kessel/inventory-client-go/common"

	"github.com/authzed/grpcutil"
	"github.com/go-kratos/kratos/v2/transport/http"
	kesselrel "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/relationships"
	kessel "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Inventory interface{}

type InventoryClient struct {
	K8sClusterService                              kessel.KesselK8SClusterServiceClient
	K8SPolicyIsPropagatedToK8SClusterServiceClient kesselrel.KesselK8SPolicyIsPropagatedToK8SClusterServiceClient
	PolicyServiceClient                            kessel.KesselK8SPolicyServiceClient
	gRPCConn                                       *grpc.ClientConn
	tokenClient                                    *common.TokenClient
}

type InventoryHttpClient struct {
	K8sClusterService                                  kessel.KesselK8SClusterServiceHTTPClient
	K8SPolicyIsPropagatedToK8SClusterServiceHTTPClient kesselrel.KesselK8SPolicyIsPropagatedToK8SClusterServiceHTTPClient
	PolicyServiceClient                                kessel.KesselK8SPolicyServiceHTTPClient
	tokenClient                                        *common.TokenClient
}

var (
	_ Inventory = &InventoryHttpClient{}
	_ Inventory = &InventoryClient{}
)

func New(config *common.Config) (*InventoryClient, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.EmptyDialOption{})
	var tokencli *common.TokenClient
	if config.EnableOIDCAuth {
		tokencli = common.NewTokenClient(config)
	}

	if config.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		tlsConfig, err := grpcutil.WithSystemCerts(grpcutil.VerifyCA)
		if err != nil {
			return nil, err
		}
		opts = append(opts, tlsConfig)
	}

	conn, err := grpc.NewClient(
		config.Url,
		opts...,
	)
	if err != nil {
		return nil, err
	}

	return &InventoryClient{
		K8sClusterService: kessel.NewKesselK8SClusterServiceClient(conn),
		K8SPolicyIsPropagatedToK8SClusterServiceClient: kesselrel.NewKesselK8SPolicyIsPropagatedToK8SClusterServiceClient(conn),
		PolicyServiceClient:                            kessel.NewKesselK8SPolicyServiceClient(conn),
		gRPCConn:                                       conn,
		tokenClient:                                    tokencli,
	}, err
}

func NewHttpClient(ctx context.Context, config *common.Config) (*InventoryHttpClient, error) {
	var tokencli *common.TokenClient
	if config.EnableOIDCAuth {
		tokencli = common.NewTokenClient(config)
	}

	var opts []http.ClientOption
	if config.HttpUrl != "" {
		opts = append(opts, http.WithEndpoint(config.HttpUrl))
	}

	if !config.Insecure {
		opts = append(opts, http.WithTLSConfig(config.TlsConfig))
	}

	if config.Timeout > 0 {
		opts = append(opts, http.WithTimeout(config.Timeout))
	}

	client, err := http.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &InventoryHttpClient{
		K8sClusterService: kessel.NewKesselK8SClusterServiceHTTPClient(client),
		K8SPolicyIsPropagatedToK8SClusterServiceHTTPClient: kesselrel.NewKesselK8SPolicyIsPropagatedToK8SClusterServiceHTTPClient(client),
		PolicyServiceClient: kessel.NewKesselK8SPolicyServiceHTTPClient(client),
		tokenClient:         tokencli,
	}, nil
}

func (a InventoryClient) GetTokenCallOption() ([]grpc.CallOption, error) {
	var opts []grpc.CallOption
	opts = append(opts, grpc.EmptyCallOption{})
	token, err := a.tokenClient.GetToken()
	if err != nil {
		return nil, err
	}
	if a.tokenClient.Insecure {
		opts = append(opts, common.WithInsecureBearerToken(token.AccessToken))
	} else {
		opts = append(opts, common.WithBearerToken(token.AccessToken))
	}

	return opts, nil
}

func (a InventoryHttpClient) GetTokenHTTPOption() ([]http.CallOption, error) {
	var opts []http.CallOption
	token, err := a.tokenClient.GetToken()
	if err != nil {
		return nil, err
	}
	header := nethttp.Header{}
	header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	opts = append(opts, http.Header(&header))
	return opts, nil
}
