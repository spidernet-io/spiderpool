package pyroscope

import (
	"bytes"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/pyroscope-io/client/internal/flameql"
)

type session struct {
	// configuration, doesn't change
	upstream      *remote
	sampleRate    uint32
	profileTypes  []ProfileType
	uploadRate    time.Duration
	disableGCRuns bool
	pid           int

	logger    Logger
	stopOnce  sync.Once
	stopCh    chan struct{}
	trieMutex sync.Mutex

	// these things do change:
	cpuBuf       *bytes.Buffer
	memBuf       *bytes.Buffer
	memPrevBytes []byte

	lastGCGeneration uint32
	appName          string
	startTime        time.Time
}

type sessionConfig struct {
	upstream       *remote
	logger         Logger
	appName        string
	tags           map[string]string
	profilingTypes []ProfileType
	disableGCRuns  bool
	sampleRate     uint32
	uploadRate     time.Duration
}

func newSession(c sessionConfig) (*session, error) {
	appName, err := mergeTagsWithAppName(c.appName, c.tags)
	if err != nil {
		return nil, err
	}

	ps := &session{
		upstream:      c.upstream,
		appName:       appName,
		profileTypes:  c.profilingTypes,
		disableGCRuns: c.disableGCRuns,
		sampleRate:    c.sampleRate,
		uploadRate:    c.uploadRate,
		stopCh:        make(chan struct{}),
		logger:        c.logger,
		cpuBuf:        &bytes.Buffer{},
		memBuf:        &bytes.Buffer{},
	}

	return ps, nil
}

// mergeTagsWithAppName validates user input and merges explicitly specified
// tags with tags from app name.
//
// App name may be in the full form including tags (app.name{foo=bar,baz=qux}).
// Returned application name is always short, any tags that were included are
// moved to tags map. When merged with explicitly provided tags (config/CLI),
// last take precedence.
//
// App name may be an empty string. Tags must not contain reserved keys,
// the map is modified in place.
func mergeTagsWithAppName(appName string, tags map[string]string) (string, error) {
	k, err := flameql.ParseKey(appName)
	if err != nil {
		return "", err
	}
	for tagKey, tagValue := range tags {
		if flameql.IsTagKeyReserved(tagKey) {
			continue
		}
		if err = flameql.ValidateTagKey(tagKey); err != nil {
			return "", err
		}
		k.Add(tagKey, tagValue)
	}
	return k.Normalized(), nil
}

// revive:disable-next-line:cognitive-complexity complexity is fine
func (ps *session) takeSnapshots() {
	ticker := time.NewTicker(time.Second / time.Duration(ps.sampleRate))
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if ps.isDueForReset() {
				ps.reset()
			}
		case <-ps.stopCh:
			return
		}
	}
}

func copyBuf(b []byte) []byte {
	r := make([]byte, len(b))
	copy(r, b)
	return r
}

func (ps *session) Start() error {
	ps.reset()

	go ps.takeSnapshots()
	return nil
}

func (ps *session) isDueForReset() bool {
	// TODO: duration should be either taken from config or ideally passed from server
	now := time.Now().Truncate(ps.uploadRate)
	start := ps.startTime.Truncate(ps.uploadRate)

	return !start.Equal(now)
}

func (ps *session) isCPUEnabled() bool {
	for _, t := range ps.profileTypes {
		if t == ProfileCPU {
			return true
		}
	}
	return false
}

func (ps *session) isMemEnabled() bool {
	for _, t := range ps.profileTypes {
		if t == ProfileInuseObjects || t == ProfileAllocObjects || t == ProfileInuseSpace || t == ProfileAllocSpace {
			return true
		}
	}
	return false
}

func (ps *session) reset() {
	now := time.Now()
	endTime := now.Truncate(ps.uploadRate)
	startTime := endTime.Add(-ps.uploadRate)
	ps.logger.Debugf("profiling session reset %s", startTime.String())

	// first reset should not result in an upload
	if !ps.startTime.IsZero() {
		ps.uploadData(startTime, endTime)
	} else {
		pprof.StartCPUProfile(ps.cpuBuf)
	}

	ps.startTime = endTime
}

func (ps *session) uploadData(startTime, endTime time.Time) {
	if ps.isCPUEnabled() {
		pprof.StopCPUProfile()
		defer func() {
			pprof.StartCPUProfile(ps.cpuBuf)
		}()
		ps.upstream.upload(&uploadJob{
			Name:            ps.appName,
			StartTime:       startTime,
			EndTime:         endTime,
			SpyName:         "gospy",
			SampleRate:      100,
			Units:           "samples",
			AggregationType: "sum",
			Format:          pprofFormat,
			Profile:         copyBuf(ps.cpuBuf.Bytes()),
		})
		ps.cpuBuf.Reset()
	}

	if ps.isMemEnabled() {
		currentGCGeneration := numGC()
		// sometimes GC doesn't run within 10 seconds
		//   in such cases we force a GC run
		//   users can disable it with disableGCRuns option
		if currentGCGeneration == ps.lastGCGeneration && !ps.disableGCRuns {
			runtime.GC()
			currentGCGeneration = numGC()
		}
		if currentGCGeneration != ps.lastGCGeneration {
			pprof.WriteHeapProfile(ps.memBuf)
			curMemBytes := copyBuf(ps.memBuf.Bytes())
			ps.memBuf.Reset()
			if ps.memPrevBytes != nil {
				ps.upstream.upload(&uploadJob{
					Name:        ps.appName,
					StartTime:   startTime,
					EndTime:     endTime,
					SpyName:     "gospy",
					SampleRate:  100,
					Format:      pprofFormat,
					Profile:     curMemBytes,
					PrevProfile: ps.memPrevBytes,
				})
			}
			ps.memPrevBytes = curMemBytes
			ps.lastGCGeneration = currentGCGeneration
		}
	}
}

func (ps *session) stop() {
	ps.trieMutex.Lock()
	defer ps.trieMutex.Unlock()

	ps.stopOnce.Do(func() {
		// TODO: wait for stopCh consumer to finish!
		close(ps.stopCh)
		// before stopping, upload the tries
		ps.uploadLastBitOfData(time.Now())
	})
}

func (ps *session) uploadLastBitOfData(now time.Time) {
	if ps.isCPUEnabled() {
		pprof.StopCPUProfile()
		ps.upstream.upload(&uploadJob{
			Name:            ps.appName,
			StartTime:       ps.startTime,
			EndTime:         now,
			SpyName:         "gospy",
			SampleRate:      100,
			Units:           "samples",
			AggregationType: "sum",
			Format:          pprofFormat,
			Profile:         copyBuf(ps.cpuBuf.Bytes()),
		})
	}
}

func numGC() uint32 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return memStats.NumGC
}
