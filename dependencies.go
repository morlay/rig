package rig

import (
	"errors"
	"fmt"
	"sync"

	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/rig/initsystem"
	"github.com/k0sproject/rig/log"
	"github.com/k0sproject/rig/os"
	"github.com/k0sproject/rig/packagemanager"
	"github.com/k0sproject/rig/remotefs"
	"github.com/k0sproject/rig/sudo"
)

// ConnectionConfigurer is an interface that can be used to configure a client.
// to get a client to use for connecting.
type ConnectionConfigurer interface {
	fmt.Stringer
	Connection() (Connection, error)
}

// DefaultConnectionConfigurer returns a new CompositeConfig to use as a default client configurator.
func DefaultConnectionConfigurer() ConnectionConfigurer {
	return &CompositeConfig{}
}

// LoggerFactory is a function that creates a logger
type LoggerFactory func(client Connection) log.Logger

var nullLogger = &log.NullLog{}

// defaultLoggerFactory returns a logger factory that returns a null logger
func defaultLoggerFactory(_ Connection) log.Logger {
	return nullLogger
}

// dependencies is a collection of injectable dependencies for a connection
type dependencies struct {
	connectionConfigurer ConnectionConfigurer
	exec.Runner          `yaml:"-"`
	log.LoggerInjectable `yaml:"-"`

	client     Connection
	clientOnce sync.Once

	fs     remotefs.FS
	fsOnce sync.Once

	os     *os.Release
	osOnce sync.Once

	initSys     initsystem.ServiceManager
	initSysOnce sync.Once

	packageMan     packagemanager.PackageManager
	packageManOnce sync.Once

	providers SubsystemProviders
}

type initsystemProvider interface {
	Get(runner exec.ContextRunner) (manager initsystem.ServiceManager, err error)
}

type packagemanagerProvider interface {
	Get(runner exec.ContextRunner) (manager packagemanager.PackageManager, err error)
}

type sudoProvider interface {
	Get(runner exec.SimpleRunner) (decorator exec.DecorateFunc, err error)
}

type fsProvider interface {
	Get(runner exec.Runner) (fs remotefs.FS, err error)
}

type osreleaseProvider interface {
	Get(runner exec.SimpleRunner) (os *os.Release, err error)
}

// SubsystemProviders is a collection of repositories for connection injectables
type SubsystemProviders struct {
	initsys        initsystemProvider
	packagemanager packagemanagerProvider
	sudo           sudoProvider
	fs             fsProvider
	os             osreleaseProvider
	loggerFactory  LoggerFactory
}

// DefaultProviders returns a set of default repositories for connection injectables
func DefaultProviders() SubsystemProviders {
	return SubsystemProviders{
		initsys:        initsystem.DefaultProvider,
		packagemanager: packagemanager.DefaultProvider,
		sudo:           sudo.DefaultProvider,
		fs:             remotefs.DefaultProvider,
		os:             os.DefaultProvider,
		loggerFactory:  defaultLoggerFactory,
	}
}

// DefaultDependencies returns a set of default injectables for a connection
func DefaultDependencies() *dependencies {
	return &dependencies{
		connectionConfigurer: DefaultConnectionConfigurer(),
		providers:            DefaultProviders(),
	}
}

// Clone returns a copy of the ConnectionInjectables with the given options applied
func (c *dependencies) Clone(opts ...Option) *dependencies {
	options := Options{connectionDependencies: &dependencies{
		connectionConfigurer: c.connectionConfigurer,
		client:               c.client,
		providers:            c.providers,
	}}
	options.Apply(opts...)
	return options.ConnectionDependencies()
}

var (
	// ErrConfiguratorNotSet is returned when a client configurator is not set when trying to connect
	ErrConfiguratorNotSet = errors.New("client configurator not set")

	// ErrClientNotSet is returned when a client is not set when trying to connect
	ErrClientNotSet = errors.New("client not set")
)

func (c *dependencies) initClient() error {
	var err error
	c.clientOnce.Do(func() {
		if c.client != nil {
			c.connectionConfigurer = nil
		}
		if c.client == nil && c.connectionConfigurer == nil {
			err = errors.Join(ErrClientNotSet, ErrConfiguratorNotSet)
			return
		}
		if c.connectionConfigurer != nil {
			c.injectLogger(c.connectionConfigurer)
			c.client, err = c.connectionConfigurer.Connection()
			if err != nil {
				err = fmt.Errorf("configure client (%v): %w", c.connectionConfigurer, err)
				return
			}
		}
		if c.client == nil {
			err = ErrClientNotSet
			return
		}
		if !c.HasLogger() {
			c.SetLogger(c.providers.loggerFactory(c.client))
		}
		c.injectLogger(c.client)
		if c.Runner == nil {
			c.Runner = exec.NewHostRunner(c.client)
		}
	})
	return err
}

func (c *dependencies) injectLogger(obj any) {
	log.InjectLogger(c.Log(), obj)
}

func (c *dependencies) sudoRunner() exec.Runner {
	decorator, err := c.providers.sudo.Get(c)
	if err != nil {
		return exec.NewErrorRunner(err)
	}
	runner := exec.NewHostRunner(c.client, decorator)
	c.injectLogger(runner)
	return runner
}

// InitSystem returns a ServiceManager for the host's init system
func (c *dependencies) getInitSystem() (initsystem.ServiceManager, error) {
	var err error
	c.initSysOnce.Do(func() {
		c.initSys, err = c.providers.initsys.Get(c)
		if err != nil {
			err = fmt.Errorf("get init system: %w", err)
		}
		c.injectLogger(c.initSys)
	})
	return c.initSys, err
}

// PackageManager returns a PackageManager for the host's package manager
func (c *dependencies) getPackageManager() (packagemanager.PackageManager, error) {
	var err error
	c.packageManOnce.Do(func() {
		c.packageMan, err = c.providers.packagemanager.Get(c)
		if err != nil {
			err = fmt.Errorf("get package manager: %w", err)
		}
		c.injectLogger(c.packageMan)
	})
	return c.packageMan, err
}

func (c *dependencies) getFS() (remotefs.FS, error) { //nolint:unparam
	var err error
	c.fsOnce.Do(func() {
		c.fs, err = c.providers.fs.Get(c)
		if err != nil {
			err = fmt.Errorf("get remote fs: %w", err)
		}
		c.injectLogger(c.fs)
	})
	return c.fs, nil
}

func (c *dependencies) getOS() (*os.Release, error) {
	var err error
	c.osOnce.Do(func() {
		c.os, err = c.providers.os.Get(c)
		if err != nil {
			err = fmt.Errorf("get os release: %w", err)
		}
		c.injectLogger(c.os)
	})
	return c.os, err
}
