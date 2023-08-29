package federation

import (
	"context"
	"math/rand"
	"strconv"

	federationv1 "github.com/spiffe/spire-plugin-sdk/proto/spire/plugin/server/federation/v1"
	"github.com/spiffe/spire/pkg/common/plugin"
)

type V1 struct {
	plugin.Facade
	federationv1.FederationPluginClient
}

func (v1 *V1) PushBundle(ctx context.Context, b string) error {

	randomString := strconv.Itoa(rand.Intn(2))

	_, err := v1.FederationPluginClient.PushBundle(ctx, &federationv1.PushBundleRequest{
		Request: randomString,
	})
	return v1.WrapErr(err)
}

func (v1 *V1) ApproveRelationship(ctx context.Context, b string) error {

	_, err := v1.FederationPluginClient.ApproveRelationship(ctx, &federationv1.RelationshipRequest{
		Request: "xauu",
	})
	return v1.WrapErr(err)
}
