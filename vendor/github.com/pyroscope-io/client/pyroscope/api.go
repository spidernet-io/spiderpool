package pyroscope

import (
	"context"
	"fmt"
	"os"
	"runtime/pprof"
	"time"
)

type Config struct {
	ApplicationName string // e.g backend.purchases
	Tags            map[string]string
	ServerAddress   string // e.g http://pyroscope.services.internal:4040
	AuthToken       string // specify this token when using pyroscope cloud
	SampleRate      uint32
	Logger          Logger
	ProfileTypes    []ProfileType
	DisableGCRuns   bool // this will disable automatic runtime.GC runs between getting the heap profiles
}

type Profiler struct {
	session  *session
	uploader *remote
}

// Start starts continuously profiling go code
func Start(cfg Config) (*Profiler, error) {
	if len(cfg.ProfileTypes) == 0 {
		cfg.ProfileTypes = DefaultProfileTypes
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = DefaultSampleRate
	}
	if cfg.Logger == nil {
		cfg.Logger = noopLogger
	}

	// Override the address to use when the environment variable is defined.
	// This is useful to support adhoc push ingestion.
	if address, ok := os.LookupEnv("PYROSCOPE_ADHOC_SERVER_ADDRESS"); ok {
		cfg.ServerAddress = address
	}

	rc := remoteConfig{
		authToken: cfg.AuthToken,
		address:   cfg.ServerAddress,
		threads:   4,
		timeout:   30 * time.Second,
	}
	uploader, err := newRemote(rc, cfg.Logger)
	if err != nil {
		return nil, err
	}

	sc := sessionConfig{
		upstream:       uploader,
		logger:         cfg.Logger,
		appName:        cfg.ApplicationName,
		tags:           cfg.Tags,
		profilingTypes: cfg.ProfileTypes,
		disableGCRuns:  cfg.DisableGCRuns,
		sampleRate:     cfg.SampleRate,
		uploadRate:     10 * time.Second,
	}

	cfg.Logger.Infof("starting profiling session:")
	cfg.Logger.Infof("  AppName:        %+v", sc.appName)
	cfg.Logger.Infof("  Tags:           %+v", sc.tags)
	cfg.Logger.Infof("  ProfilingTypes: %+v", sc.profilingTypes)
	cfg.Logger.Infof("  DisableGCRuns:  %+v", sc.disableGCRuns)
	cfg.Logger.Infof("  SampleRate:     %+v", sc.sampleRate)
	cfg.Logger.Infof("  UploadRate:     %+v", sc.uploadRate)
	s, err := newSession(sc)
	if err != nil {
		return nil, fmt.Errorf("new session: %w", err)
	}
	if err = s.Start(); err != nil {
		return nil, fmt.Errorf("start session: %w", err)
	}

	return &Profiler{session: s, uploader: uploader}, nil
}

// Stop stops continious profiling session and uploads the remaining profiling data
func (p *Profiler) Stop() error {
	p.session.stop()
	p.uploader.Stop()
	return nil
}

type LabelSet = pprof.LabelSet

var Labels = pprof.Labels

func TagWrapper(ctx context.Context, labels LabelSet, cb func(context.Context)) {
	pprof.Do(ctx, labels, func(c context.Context) { cb(c) })
}
