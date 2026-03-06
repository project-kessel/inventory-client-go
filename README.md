# Inventory API Go Client

# ⛔️ This repository is no longer actively maintained. Please consider using [kessel-sdk-go](https://github.com/project-kessel/kessel-sdk-go) instead ⛔️

```go
  client, err := v1beta1.New(v1beta1.NewConfig(
        v1beta1.WithgRPCUrl("localhost:9081"),
		v1beta1.WithTLSInsecure(true),
	))
  
  request := kessel.CreateRhelHostRequest{RhelHost: &kessel.RhelHost{
		Metadata: &kessel.Metadata{
			ResourceType: "rhel-host",
			Workspace:    "",
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

	_, err = client.RhelHostServiceClient.CreateRhelHost(context.Background(), &request)
	if err != nil {
		fmt.Println(err)
	}
```

# Using OIDC authentication

Set the service account: `clientId`, `secret`, `sso token url`

```go
    client, err := v1beta1.New(v1beta1.NewConfig(
        v1beta1.WithgRPCUrl("localhost:9081"),
        v1beta1.WithTLSInsecure(true),
        v1beta1.WithAuthEnabled("test-svc", "secret", "https://host:port/auth/realms/$REALM/protocol/openid-connect/token"),
	))
	if err != nil {
		fmt.Println(err)
	}
	
	request := kessel.CreateRhelHostRequest{RhelHost: &kessel.RhelHost{
		Metadata: &kessel.Metadata{
			ResourceType: "rhel-host",
			Workspace:    "",
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
	_, err = client.RhelHostServiceClient.CreateRhelHost(context.Background(), &request, opts...)
	if err != nil {
		fmt.Println(err)
	}


```
