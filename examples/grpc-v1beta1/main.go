package main

import (
	"context"
	"fmt"

	kessel "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	"github.com/project-kessel/inventory-client-go/common"
	"github.com/project-kessel/inventory-client-go/v1beta1"
)

func main() {
	client, err := v1beta1.New(common.NewConfig(
		common.WithgRPCUrl("localhost:9081"),
		common.WithTLSInsecure(true),
		common.WithAuthEnabled("", "", ""),
	))
	if err != nil {
		fmt.Println(err)
	}

	request := kessel.CreateK8SClusterRequest{K8SCluster: &kessel.K8SCluster{
		Metadata: &kessel.Metadata{
			ResourceType: "k8s-cluster",
			WorkspaceId:  "",
		},
		ReporterData: &kessel.ReporterData{
			ReporterType:       kessel.ReporterData_ACM,
			ReporterInstanceId: "service-account-svc-test",
			ConsoleHref:        "www.example.com",
			ApiHref:            "www.example.com",
			LocalResourceId:    "1",
			ReporterVersion:    "0.1",
		},
	}}

	opts, err := client.GetTokenCallOption()
	if err != nil {
		fmt.Println(err)
	}
	_, err = client.K8sClusterService.CreateK8SCluster(context.Background(), &request, opts...)
	if err != nil {
		fmt.Println(err)
	}
}
