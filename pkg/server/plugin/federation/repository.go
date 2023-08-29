package federation

type Repository struct {
	Federations []Federation
}

func (repo *Repository) GetFederations() []Federation {
	return repo.Federations
}

func (repo *Repository) AddFederation(federation Federation) {
	repo.Federations = append(repo.Federations, federation)
}

func (repo *Repository) Clear() {
	repo.Federations = nil
}
