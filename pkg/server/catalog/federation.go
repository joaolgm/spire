package catalog

import (
	"github.com/spiffe/spire/pkg/common/catalog"
	"github.com/spiffe/spire/pkg/server/plugin/federation"
	"github.com/spiffe/spire/pkg/server/plugin/federation/awss3"
)

type federationRepository struct {
	federation.Repository
}

func (repo *federationRepository) Binder() interface{} {
	return repo.AddFederation
}

func (repo *federationRepository) Constraints() catalog.Constraints {
	return catalog.ZeroOrMore()
}

func (repo *federationRepository) Versions() []catalog.Version {
	return []catalog.Version{federationV1{}}
}

func (repo *federationRepository) BuiltIns() []catalog.BuiltIn {
	return []catalog.BuiltIn{
		awss3.BuiltIn(),
	}
}

type federationV1 struct{}

func (federationV1) New() catalog.Facade { return new(federation.V1) }
func (federationV1) Deprecated() bool    { return false }
