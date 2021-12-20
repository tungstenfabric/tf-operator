module github.com/tungstenfabric/tf-operator/contrib/tools

go 1.14

require (
	github.com/golangci/golangci-lint v1.23.6
	sigs.k8s.io/controller-tools v0.4.0
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/client-go => k8s.io/client-go v0.19.0 // Required by prometheus-operator
)
