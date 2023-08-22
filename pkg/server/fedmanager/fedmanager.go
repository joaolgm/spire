// Package fedmanager manages the push of the trust bundle to external
// stores through the configured Federation plugins.
package fedmanager

import (
	"context"
	"errors"
	"fmt"
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
	refreshInterval = 30 * time.Second
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
		// Test hook used to indicate an attempt to publish a bundle using a
		// specific bundle publisher.
		pushResultCh chan *pushResult

		// Test hook used to indicate when the action of publishing a bundle
		// has finished.
		pushedCh chan error
	}
}

// Run runs the federation manager.
func (m *Manager) Run(ctx context.Context) error {
	fmt.Println("<-- pkg/server/fedmanager/fedmanager.go - Run(ctx)")
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

// BundleUpdated tells the bundle publishing manager that the bundle has been
// updated and forces a PublishBundle operation on all the plugins.
func (m *Manager) BundleUpdated() {
	m.drainBundleUpdated()
	m.federationUpdatedCh <- struct{}{}
}

// callPublishBundle calls the publishBundle function and logs if there was an
// error.
func (m *Manager) callPushBundle(ctx context.Context) {
	if err := m.pushBundle(ctx); err != nil && ctx.Err() == nil {
		m.log.WithError(err).Error("Failed to push bundle")
	}
}

// publishBundle iterates through the configured bundle publishers and calls
// PublishBundle with the fetched bundle. This function only returns an error
// if bundle publishers can't be called due to a failure fetching the bundle
// from the datastore.
func (m *Manager) pushBundle(ctx context.Context) (err error) {
	defer func() {
		m.pushDone(err)
	}()

	if len(m.federations) == 0 {
		return nil
	}

	// bundle, err := m.dataStore.FetchBundle(ctx, m.trustDomain.IDString())
	// if err != nil {
	// 	return fmt.Errorf("failed to fetch bundle from datastore: %w", err)
	// }

	testing := "ooooooi"
	var wg sync.WaitGroup
	wg.Add(len(m.federations))
	for _, bp := range m.federations {
		bp := bp
		go func() {
			defer wg.Done()

			log := m.log.WithField(bp.Type(), bp.Name())
			err := bp.PushBundle(ctx, testing)
			if err != nil {
				log.WithError(err).Error("Failed to publish bundle")
			}

			m.triggerPushResultHook(&pushResult{
				pluginName: bp.Name(),
				bundle:     testing,
				err:        err,
			})
		}()
	}

	wg.Wait()

	// PublishBundle was called on all the plugins. Is the responsibility of
	// each plugin to handle failure conditions and implement a retry logic if
	// needed.
	return nil
}

// triggerPublishResultHook is called to know when the publish action using a
// specific bundle publisher has happened. It informs the result of calling the
// PublishBundle method to a bundle publisher.
func (m *Manager) triggerPushResultHook(result *pushResult) {
	if m.hooks.pushResultCh != nil {
		m.hooks.pushResultCh <- result
	}
}

// publishDone is called to know when a publish action has finished and informs
// if there was an error in the overall action (not specific to a bundle
// publisher). A publish action happens periodically (every refreshInterval) and
// also when BundleUpdated() is called.
func (m *Manager) pushDone(err error) {
	if m.hooks.pushedCh != nil {
		m.hooks.pushedCh <- err
	}
}

// publishResult holds information about the result of trying to publish a
// bundle using a specific bundle publisher.
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
