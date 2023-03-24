package stubs

import (
	"context"
	"strconv"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/RHEnVision/provisioning-backend/internal/clients"
	"github.com/RHEnVision/provisioning-backend/internal/models"
	"github.com/RHEnVision/provisioning-backend/internal/ptr"
)

type AzureClientStub struct {
	createdVms []*armcompute.VirtualMachine
	createdRgs []*armresources.ResourceGroup
}

func DidCreateAzureResourceGroup(ctx context.Context, name string) bool {
	client, err := getAzureClientStub(ctx)
	if err != nil {
		return false
	}
	for _, rg := range client.createdRgs {
		if *rg.Name == name {
			return true
		}
	}
	return false
}

func CountStubAzureVMs(ctx context.Context) int {
	client, err := getAzureClientStub(ctx)
	if err != nil {
		return 0
	}
	return len(client.createdVms)
}

func (stub *AzureClientStub) Status(ctx context.Context) error {
	return nil
}

func (stub *AzureClientStub) CreateVM(ctx context.Context, location string, resourceGroupName string, imageID string, pubkey *models.Pubkey, instanceType clients.InstanceTypeName, vmName string, userData []byte) (*string, error) {
	id := strconv.Itoa(len(stub.createdVms) + 1)

	vm := armcompute.VirtualMachine{
		ID:       &id,
		Name:     &vmName,
		Location: &location,
		Properties: &armcompute.VirtualMachineProperties{
			UserData: ptr.To(string(userData)),
		},
	}
	stub.createdVms = append(stub.createdVms, &vm)
	return &id, nil
}

func (stub *AzureClientStub) EnsureResourceGroup(ctx context.Context, name string, location string) (*string, error) {
	id := strconv.Itoa(len(stub.createdRgs) + 1)

	rg := armresources.ResourceGroup{
		ID:       &id,
		Name:     &name,
		Location: &location,
	}
	stub.createdRgs = append(stub.createdRgs, &rg)
	return &id, nil
}

func (stub *AzureClientStub) TenantId(ctx context.Context) (string, error) {
	return "4645f0cb-43f5-4586-b2c9-8d5c58577e3e", nil
}

func (stub *AzureClientStub) ListResourceGroups(ctx context.Context) ([]string, error) {
	return []string{"firstGroup", "secondGroup", "test"}, nil
}
