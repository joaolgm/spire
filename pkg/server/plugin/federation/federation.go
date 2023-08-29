package federation

import (
	"context"

	"github.com/spiffe/spire/pkg/common/catalog"
)

type Federation interface {
	catalog.PluginInfo

	PushBundle(ctx context.Context, req string) error
	ApproveRelationship(ctx context.Context, req string) error
}
