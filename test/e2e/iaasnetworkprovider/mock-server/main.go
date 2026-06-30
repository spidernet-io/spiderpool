// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strings"

	"github.com/spidernet-io/spiderpool/pkg/lock"
)

const (
	allocatePath      = "/v1/apis/network.iaas.io/ipam/allocate-ips"
	releasePath       = "/v1/apis/network.iaas.io/ipam/release-ip"
	ipCacheStatusPath = "/v1/apis/network.iaas.io/status/ips-cache/"
)

type server struct {
	mutex       lock.Mutex
	records     []record
	ipCache     map[string]ipCacheEntry
	usedVLANIDs map[int64]string
}

type record struct {
	Path string      `json:"path"`
	Body interface{} `json:"body"`
}

type allocateRequest struct {
	PodName                  string                `json:"podName,omitempty"`
	PodNamespace             string                `json:"podNamespace,omitempty"`
	PodUID                   string                `json:"podUID,omitempty"`
	NodeName                 string                `json:"nodeName"`
	IaaSIPsAllocationRequest []ipAllocationRequest `json:"iaasIPsAllocationRequest"`
}

type ipAllocationRequest struct {
	IPAddress    string `json:"ipAddress"`
	Subnet       string `json:"subnet"`
	ParentNicMac string `json:"parentNicMac"`
}

type allocateResponse struct {
	PodName                   string               `json:"podName"`
	PodNamespace              string               `json:"podNamespace"`
	NodeName                  string               `json:"nodeName"`
	IaaSIPsAllocationResponse []ipAllocationResult `json:"iaasIPsAllocationResponse"`
}

type ipAllocationResult struct {
	ParentNicMac string `json:"parentNicMac"`
	Subnet       string `json:"subnet"`
	IPAddress    string `json:"ipAddress"`
	MacAddress   string `json:"macAddress"`
	VlanID       int64  `json:"vlanId"`
}

type releaseRequest struct {
	PodName      string `json:"podName,omitempty"`
	PodNamespace string `json:"podNamespace,omitempty"`
	PodUID       string `json:"podUID,omitempty"`
	NodeName     string `json:"nodeName"`
	ParentNicMac string `json:"parentNicMac,omitempty"`
	Subnet       string `json:"subnet"`
	IPAddress    string `json:"ipAddress"`
}

type ipCacheEntry struct {
	NodeName     string `json:"nodeName"`
	IPAddress    string `json:"ipAddress"`
	Subnet       string `json:"subnet"`
	ParentNicMac string `json:"parentNicMac"`
	Mac          string `json:"mac"`
	VlanID       int64  `json:"vlanID"`
}

type ipCacheStatusResponse struct {
	Entry ipCacheEntry `json:"entry"`
}

func main() {
	s := &server{
		ipCache:     make(map[string]ipCacheEntry),
		usedVLANIDs: make(map[int64]string),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthz)
	mux.HandleFunc("/records", s.recordsHandler)
	mux.HandleFunc("/reset", s.reset)
	mux.HandleFunc(allocatePath, s.allocate)
	mux.HandleFunc(releasePath, s.release)
	mux.HandleFunc(ipCacheStatusPath, s.ipCacheStatus)

	log.Println("starting IaaS provider mock server on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func (s *server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) recordsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	writeJSON(w, http.StatusOK, map[string]interface{}{"records": s.records})
}

func (s *server) reset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.records = nil
	s.ipCache = make(map[string]ipCacheEntry)
	s.usedVLANIDs = make(map[int64]string)
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

func (s *server) allocate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req allocateRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	resp := allocateResponse{
		PodName:      req.PodName,
		PodNamespace: req.PodNamespace,
		NodeName:     req.NodeName,
	}
	for _, item := range req.IaaSIPsAllocationRequest {
		vlanID, err := s.allocateVLANID(item.IPAddress)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
			return
		}

		result := ipAllocationResult{
			ParentNicMac: item.ParentNicMac,
			Subnet:       item.Subnet,
			IPAddress:    item.IPAddress,
			MacAddress:   macForIP(item.IPAddress),
			VlanID:       vlanID,
		}
		resp.IaaSIPsAllocationResponse = append(resp.IaaSIPsAllocationResponse, result)
		s.ipCache[item.IPAddress] = ipCacheEntry{
			NodeName:     req.NodeName,
			IPAddress:    item.IPAddress,
			Subnet:       item.Subnet,
			ParentNicMac: item.ParentNicMac,
			Mac:          result.MacAddress,
			VlanID:       result.VlanID,
		}
	}

	s.records = append(s.records, record{Path: r.URL.Path, Body: req})
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) release(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req releaseRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.records = append(s.records, record{Path: r.URL.Path, Body: req})
	if entry, ok := s.ipCache[req.IPAddress]; ok {
		delete(s.usedVLANIDs, entry.VlanID)
		delete(s.ipCache, req.IPAddress)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) ipCacheStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	rawIP := strings.TrimPrefix(r.URL.Path, ipCacheStatusPath)
	ipAddress, err := url.PathUnescape(rawIP)
	if err != nil || ipAddress == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid ipAddress"})
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	entry, ok := s.ipCache[ipAddress]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ip not found"})
		return
	}
	writeJSON(w, http.StatusOK, ipCacheStatusResponse{Entry: entry})
}

func readJSON(r *http.Request, out interface{}) error {
	defer func() { _ = r.Body.Close() }()
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		return fmt.Errorf("decode request body: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.WriteHeader(status)
	if status != http.StatusNoContent {
		_, _ = w.Write(data)
	}
}

func writeJSONError(w http.ResponseWriter, status int, err error) {
	if err == nil {
		err = errors.New("unknown error")
	}
	data := []byte(fmt.Sprintf(`{"error":%q}`, err.Error()))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func macForIP(ipAddress string) string {
	sum := sha1.Sum([]byte(ipAddress))
	return fmt.Sprintf("02:%s:%s:%s:%s:%s", hex.EncodeToString(sum[0:1]), hex.EncodeToString(sum[1:2]), hex.EncodeToString(sum[2:3]), hex.EncodeToString(sum[3:4]), hex.EncodeToString(sum[4:5]))
}

func (s *server) allocateVLANID(ipAddress string) (int64, error) {
	if entry, ok := s.ipCache[ipAddress]; ok {
		return entry.VlanID, nil
	}

	const (
		minVLANID = int64(1)
		maxVLANID = int64(4094)
	)

	if len(s.usedVLANIDs) >= int(maxVLANID-minVLANID+1) {
		return 0, errors.New("no available vlan id")
	}

	for {
		vlanID, err := randomVLANID(minVLANID, maxVLANID)
		if err != nil {
			return 0, err
		}
		if _, ok := s.usedVLANIDs[vlanID]; ok {
			continue
		}
		s.usedVLANIDs[vlanID] = ipAddress
		return vlanID, nil
	}
}

func randomVLANID(minVLANID, maxVLANID int64) (int64, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(maxVLANID-minVLANID+1))
	if err != nil {
		return 0, fmt.Errorf("generate vlan id: %w", err)
	}
	return minVLANID + n.Int64(), nil
}
