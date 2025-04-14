package v1beta1

import (
	"context"
	"fmt"
	"github.com/project-kessel/inventory-client-go/common"
	nethttp "net/http"

	"github.com/authzed/grpcutil"
	"github.com/go-kratos/kratos/v2/transport/http"
	kesselv2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Inventory interface{}

type InventoryClient struct {
	KesselResourceService kesselv2.KesselResourceServiceClient
	KesselCheckService    kesselv2.KesselCheckServiceClient
	KesselLookupService   kesselv2.KesselLookupServiceClient
	gRPCConn              *grpc.ClientConn
	tokenClient           *common.TokenClient
}

type InventoryHttpClient struct {
	KesselResourceService kesselv2.KesselResourceServiceHTTPClient
	KesselCheckService    kesselv2.KesselCheckServiceHTTPClient
	tokenClient           *common.TokenClient
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
		KesselResourceService: kesselv2.NewKesselResourceServiceClient(conn),
		KesselCheckService:    kesselv2.NewKesselCheckServiceClient(conn),
		KesselLookupService:   kesselv2.NewKesselLookupServiceClient(conn),
		gRPCConn:              conn,
		tokenClient:           tokencli,
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

	client, err := http.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &InventoryHttpClient{
		KesselResourceService: kesselv2.NewKesselResourceServiceHTTPClient(client),
		KesselCheckService:    kesselv2.NewKesselCheckServiceHTTPClient(client),
		tokenClient:           tokencli,
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
