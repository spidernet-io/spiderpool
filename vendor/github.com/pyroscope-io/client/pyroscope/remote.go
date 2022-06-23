package pyroscope

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
)

type uploadFormat string

const pprofFormat uploadFormat = "pprof"

type uploadJob struct {
	Name            string
	StartTime       time.Time
	EndTime         time.Time
	SpyName         string
	SampleRate      uint32
	Units           string
	AggregationType string
	Format          uploadFormat
	Profile         []byte
	PrevProfile     []byte
}

var (
	errCloudTokenRequired = errors.New("please provide an authentication token. You can find it here: https://pyroscope.io/cloud")
)

const cloudHostnameSuffix = "pyroscope.cloud"

type remote struct {
	cfg    remoteConfig
	jobs   chan *uploadJob
	client *http.Client
	Logger Logger

	done chan struct{}
	wg   sync.WaitGroup
}

type remoteConfig struct {
	authToken string
	threads   int
	address   string
	timeout   time.Duration
}

func newRemote(cfg remoteConfig, logger Logger) (*remote, error) {
	r := &remote{
		cfg:  cfg,
		jobs: make(chan *uploadJob, 20),
		client: &http.Client{
			Transport: &http.Transport{
				MaxConnsPerHost: cfg.threads,
			},
			// Don't follow redirects
			// Since the go http client strips the Authorization header when doing redirects (eg http -> https)
			// https://github.com/golang/go/blob/a41763539c7ad09a22720a517a28e6018ca4db0f/src/net/http/client_test.go#L1764
			// that makes the an authorized return a 401
			// which is confusing since the user most likely already set up an API Key
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Timeout: cfg.timeout,
		},
		Logger: logger,
		done:   make(chan struct{}),
	}

	// parse the upstream address
	u, err := url.Parse(cfg.address)
	if err != nil {
		return nil, err
	}

	// authorize the token first
	if cfg.authToken == "" && requiresAuthToken(u) {
		return nil, errCloudTokenRequired
	}

	// start goroutines for uploading profile data
	r.Start()
	return r, nil
}

func (r *remote) Start() {
	r.wg.Add(r.cfg.threads)
	for i := 0; i < r.cfg.threads; i++ {
		go r.handleJobs()
	}
}

func (r *remote) Stop() {
	if r.done != nil {
		close(r.done)
	}

	// wait for uploading goroutines exit
	r.wg.Wait()
}

func (r *remote) upload(job *uploadJob) {
	select {
	case r.jobs <- job:
	default:
		r.Logger.Errorf("remote upload queue is full, dropping a profile job")
	}
}

func (r *remote) uploadProfile(j *uploadJob) error {
	u, err := url.Parse(r.cfg.address)
	if err != nil {
		return fmt.Errorf("url parse: %v", err)
	}

	body := &bytes.Buffer{}

	writer := multipart.NewWriter(body)
	fw, err := writer.CreateFormFile("profile", "profile.pprof")
	fw.Write(j.Profile)
	if err != nil {
		return err
	}
	if j.PrevProfile != nil {
		fw, err = writer.CreateFormFile("prev_profile", "profile.pprof")
		fw.Write(j.PrevProfile)
		if err != nil {
			return err
		}
	}
	writer.Close()

	q := u.Query()
	q.Set("name", j.Name)
	// TODO: I think these should be renamed to startTime / endTime
	q.Set("from", strconv.Itoa(int(j.StartTime.Unix())))
	q.Set("until", strconv.Itoa(int(j.EndTime.Unix())))
	q.Set("spyName", j.SpyName)
	q.Set("sampleRate", strconv.Itoa(int(j.SampleRate)))
	q.Set("units", j.Units)
	q.Set("aggregationType", j.AggregationType)

	u.Path = path.Join(u.Path, "/ingest")
	u.RawQuery = q.Encode()

	r.Logger.Debugf("uploading at %s", u.String())
	// new a request for the job
	request, err := http.NewRequest("POST", u.String(), body)
	if err != nil {
		return fmt.Errorf("new http request: %v", err)
	}
	contentType := writer.FormDataContentType()
	r.Logger.Debugf("content type: %s", contentType)
	request.Header.Set("Content-Type", contentType)
	// request.Header.Set("Content-Type", "binary/octet-stream+"+string(j.Format))

	if r.cfg.authToken != "" {
		request.Header.Set("Authorization", "Bearer "+r.cfg.authToken)
	}

	// do the request and get the response
	response, err := r.client.Do(request)
	if err != nil {
		return fmt.Errorf("do http request: %v", err)
	}
	defer response.Body.Close()

	// read all the response body
	respBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read response body: %v", err)
	}

	if response.StatusCode != 200 {
		return fmt.Errorf("failed to upload. server responded with statusCode: '%d' and body: '%s'", response.StatusCode, string(respBody))
	}

	return nil
}

// handle the jobs
func (r *remote) handleJobs() {
	for {
		select {
		case <-r.done:
			r.wg.Done()
			return
		case job := <-r.jobs:
			r.safeUpload(job)
		}
	}
}

func requiresAuthToken(u *url.URL) bool {
	return strings.HasSuffix(u.Host, cloudHostnameSuffix)
}

// do safe upload
func (r *remote) safeUpload(job *uploadJob) {
	defer func() {
		if catch := recover(); catch != nil {
			r.Logger.Errorf("recover stack: %v", debug.Stack())
		}
	}()

	// update the profile data to server
	if err := r.uploadProfile(job); err != nil {
		r.Logger.Errorf("upload profile: %v", err)
	}
}
