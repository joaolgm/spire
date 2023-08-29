// Package fedmanager manages the push of the trust bundle to external
// stores through the configured Federation plugins.
package fedmanager

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/andres-erbsen/clock"
	"github.com/sirupsen/logrus"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/spire/pkg/server/datastore"
	"github.com/spiffe/spire/pkg/server/plugin/federation"
)

const (
	// refreshInterval is the interval to check for an updated trust bundle.
	refreshInterval = 3 * time.Second
)

// NewManager creates a new federation manager.
func NewManager(c *ManagerConfig) (*Manager, error) {
	return newManager(c)
}

// ManagerConfig is the config for the federation manager.
type ManagerConfig struct {
	Federations []federation.Federation
	DataStore   datastore.DataStore
	Clock       clock.Clock
	Log         logrus.FieldLogger
	TrustDomain spiffeid.TrustDomain
}

// Manager is the manager for federations. It implements the FedManager
// interface.
type Manager struct {
	federationUpdatedCh chan struct{}
	federations         []federation.Federation
	clock               clock.Clock
	dataStore           datastore.DataStore
	log                 logrus.FieldLogger
	trustDomain         spiffeid.TrustDomain

	hooks struct {
		// Test hook used to indicate an attempt to push a string using a
		// specific plugin_base.
		pushResultCh chan *pushResult

		// Test hook used to indicate when the action of pushing a string
		// has finished.
		pushedCh chan error
	}
}

// Run runs the federation manager.
func (m *Manager) Run(ctx context.Context) error {
	ticker := m.clock.Ticker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.callPushBundle(ctx)
		case <-m.federationUpdatedCh:
			m.callPushBundle(ctx)
		case <-ctx.Done():
			return nil
		}
	}
}

// BundleUpdated tells the federation manager that the bundle has been
// updated and forces a PushBundle operation on all the plugins.
func (m *Manager) BundleUpdated() {
	m.drainBundleUpdated()
	m.federationUpdatedCh <- struct{}{}
}

// callPushBundle calls the pushBundle function and logs if there was an
// error.
func (m *Manager) callPushBundle(ctx context.Context) {
	if err := m.pushBundle(ctx); err != nil && ctx.Err() == nil {
		m.log.WithError(err).Error("Failed to push bundle")
	}
}

// pushBundle iterates through the configured plugin_base and calls
// PushBundle with the fetched string. This function only returns an error
// if plugin_base can't be called due to a failure fetching the bundle
// from the datastore.
func (m *Manager) pushBundle(ctx context.Context) (err error) {
	defer func() {
		m.pushDone(err)
	}()

	if len(m.federations) == 0 {
		return nil
	}

	testing := "testing string"
	var wg sync.WaitGroup
	wg.Add(len(m.federations))
	for _, bp := range m.federations {
		bp := bp
		go func() {
			defer wg.Done()

			log := m.log.WithField(bp.Type(), bp.Name())
			err := bp.PushBundle(ctx, testing)
			if err != nil {
				log.WithError(err).Error("Failed to push bundle")
			}

			m.triggerPushResultHook(&pushResult{
				pluginName: bp.Name(),
				bundle:     testing,
				err:        err,
			})
		}()
	}

	wg.Wait()

	// PushBundle was called on all the plugins. Is the responsibility of
	// each plugin to handle failure conditions and implement a retry logic if
	// needed.
	return nil
}

// triggerPushResultHook is called to know when the push action using a
// specific plugin_base has happened. It informs the result of calling the
// PushBundle method to a plugin_base.
func (m *Manager) triggerPushResultHook(result *pushResult) {
	if m.hooks.pushResultCh != nil {
		m.hooks.pushResultCh <- result
	}
}

// pushDone is called to know when a push action has finished and informs
// if there was an error in the overall action (not specific to a plugin_base).
// A push action happens periodically (every refreshInterval) and
// also when BundleUpdated() is called.
func (m *Manager) pushDone(err error) {
	if m.hooks.pushedCh != nil {
		m.hooks.pushedCh <- err
	}
}

// pushResult holds information about the result of trying to push a
// string using a specific plugin_base.
type pushResult struct {
	pluginName string
	bundle     string
	err        error
}

func (m *Manager) drainBundleUpdated() {
	select {
	case <-m.federationUpdatedCh:
	default:
	}
}

func newManager(c *ManagerConfig) (*Manager, error) {
	if c.DataStore == nil {
		return nil, errors.New("missing datastore")
	}

	if c.TrustDomain.IsZero() {
		return nil, errors.New("missing trust domain")
	}

	if c.Clock == nil {
		c.Clock = clock.New()
	}

	return &Manager{
		federationUpdatedCh: make(chan struct{}, 1),
		federations:         c.Federations,
		clock:               c.Clock,
		dataStore:           c.DataStore,
		log:                 c.Log,
		trustDomain:         c.TrustDomain,
	}, nil
}
