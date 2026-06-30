// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package iaasnetworkprovider_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Label("iaasnetworkprovider")

const (
	providerMockNamespace = "iaas-provider-mock"
	providerMockName      = "provider-mock-server"
	providerMockPort      = int32(8080)
)

const (
	providerMockAllocatePath = "/v1/apis/network.iaas.io/ipam/allocate-ips"
	providerMockReleasePath  = "/v1/apis/network.iaas.io/ipam/release-ip"
)

type providerMockServer struct {
	frame     *e2e.Framework
	namespace string
}

type providerMockRecords struct {
	Records []providerMockRecord `json:"records"`
}

type providerMockRecord struct {
	Path string                 `json:"path"`
	Body map[string]interface{} `json:"body"`
}

type providerMockIPCacheEntry struct {
	NodeName     string `json:"nodeName"`
	IPAddress    string `json:"ipAddress"`
	Subnet       string `json:"subnet"`
	ParentNicMac string `json:"parentNicMac"`
	Mac          string `json:"mac"`
	VlanID       int64  `json:"vlanID"`
}

type providerMockIPCacheResponse struct {
	Entry providerMockIPCacheEntry `json:"entry"`
}

func newProviderMockServer(frame *e2e.Framework, namespace string) *providerMockServer {
	return &providerMockServer{
		frame:     frame,
		namespace: namespace,
	}
}

func providerMockNamespaceForProcess() string {
	process := GinkgoParallelProcess()
	if process <= 1 {
		return providerMockNamespace
	}
	return fmt.Sprintf("%s-%d", providerMockNamespace, process)
}

func (s *providerMockServer) Deploy() (string, error) {
	if s == nil || s.frame == nil || s.namespace == "" {
		return "", e2e.ErrWrongInput
	}

	deadline := time.Now().Add(common.ResourceDeleteTimeout)
	for {
		if err := s.prepareNamespace(); err != nil {
			return "", err
		}
		if err := s.createOrReplace(providerMockService(s.namespace)); err != nil {
			if isNamespaceTerminatingError(err) && time.Now().Before(deadline) {
				continue
			}
			return "", err
		}
		if err := s.createDeploymentUntilReady(providerMockDeployment(s.namespace)); err != nil {
			if isNamespaceTerminatingError(err) && time.Now().Before(deadline) {
				continue
			}
			return "", err
		}

		return fmt.Sprintf("http://%s.%s.svc:%d", providerMockName, s.namespace, providerMockPort), nil
	}
}

func (s *providerMockServer) Reset() error {
	_, err := s.requestLocal("POST", "/reset")
	return err
}

func (s *providerMockServer) Records() (*providerMockRecords, error) {
	out, err := s.requestLocal("GET", "/records")
	if err != nil {
		return nil, err
	}

	records := &providerMockRecords{}
	if err := json.Unmarshal(out, records); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *providerMockServer) IPCache(ipAddress string) (*providerMockIPCacheEntry, error) {
	out, err := s.requestLocal("GET", "/v1/apis/network.iaas.io/status/ips-cache/"+url.PathEscape(ipAddress))
	if err != nil {
		return nil, err
	}

	resp := &providerMockIPCacheResponse{}
	if err := json.Unmarshal(out, resp); err != nil {
		return nil, err
	}
	return &resp.Entry, nil
}

func (s *providerMockServer) Cleanup() error {
	if s == nil || s.frame == nil || s.namespace == "" {
		return nil
	}

	// Delete the Deployment first so the mock-server Pod is removed and
	// spiderpool can clean up its SpiderEndpoint finalizer naturally.
	// If we delete the namespace directly, the SpiderEndpoint finalizer
	// blocks namespace deletion and the force-finalize fallback orphans
	// the SpiderEndpoint, which later breaks the helm pre-delete hook.
	s.deleteDeploymentBeforeNamespace()

	return deleteNamespaceUntilFinishWithFallback(s.frame, s.namespace)
}

func (s *providerMockServer) deleteDeploymentBeforeNamespace() {
	_, err := s.frame.GetDeployment(providerMockName, s.namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return
		}
		GinkgoWriter.Printf("failed to get provider mock deployment %s/%s for cleanup: %v\n", s.namespace, providerMockName, err)
		return
	}

	propagationPolicy := metav1.DeletePropagationBackground
	if err := s.frame.DeleteDeployment(providerMockName, s.namespace, &client.DeleteOptions{PropagationPolicy: &propagationPolicy}); err != nil && !apierrors.IsNotFound(err) {
		GinkgoWriter.Printf("failed to delete provider mock deployment %s/%s for cleanup: %v\n", s.namespace, providerMockName, err)
		return
	}

	// Wait for the Deployment's Pods to be gone so spiderpool reclaims the
	// SpiderEndpoint before the namespace is deleted.
	Eventually(func() bool {
		pods, err := s.frame.GetPodList(
			client.InNamespace(s.namespace),
			client.MatchingLabels(providerMockLabels()),
		)
		if err != nil {
			return false
		}
		return len(pods.Items) == 0
	}).WithTimeout(common.ResourceDeleteTimeout).WithPolling(2*time.Second).Should(BeTrue(),
		"provider mock deployment pods should be removed before namespace deletion")
}

func (s *providerMockServer) createDeploymentUntilReady(deployment *appsv1.Deployment) error {
	if err := s.frame.CreateDeployment(deployment); err != nil {
		return err
	}

	var lastReady int32
	Eventually(func(g Gomega) {
		current, err := s.frame.GetDeployment(deployment.Name, deployment.Namespace)
		g.Expect(err).NotTo(HaveOccurred())
		lastReady = current.Status.ReadyReplicas
		g.Expect(current.Spec.Replicas).NotTo(BeNil())
		g.Expect(current.Status.ReadyReplicas).To(Equal(*current.Spec.Replicas))
	}).WithTimeout(common.PodStartTimeout).WithPolling(time.Second).Should(Succeed(), func() string {
		return fmt.Sprintf(
			"provider mock deployment %s/%s readyReplicas=%d diagnostics:\n%s",
			deployment.Namespace,
			deployment.Name,
			lastReady,
			s.deploymentDiagnostics(deployment),
		)
	})

	return nil
}

func (s *providerMockServer) deploymentDiagnostics(deployment *appsv1.Deployment) string {
	var b strings.Builder

	pods, err := s.frame.GetPodList(
		client.InNamespace(deployment.Namespace),
		client.MatchingLabelsSelector{Selector: labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels)},
	)
	if err != nil {
		_, _ = fmt.Fprintf(&b, "list pods: %v\n", err)
		return b.String()
	}
	if len(pods.Items) == 0 {
		_, _ = fmt.Fprintf(&b, "no pods found for selector %v\n", deployment.Spec.Selector.MatchLabels)
		return b.String()
	}

	for i := range pods.Items {
		pod := &pods.Items[i]
		_, _ = fmt.Fprintf(&b, "pod %s/%s phase=%s reason=%s message=%q node=%s\n", pod.Namespace, pod.Name, pod.Status.Phase, pod.Status.Reason, pod.Status.Message, pod.Spec.NodeName)
		for _, condition := range pod.Status.Conditions {
			_, _ = fmt.Fprintf(&b, "  condition %s=%s reason=%s message=%q\n", condition.Type, condition.Status, condition.Reason, condition.Message)
		}
		for _, status := range pod.Status.ContainerStatuses {
			_, _ = fmt.Fprintf(&b, "  container %s ready=%t restartCount=%d image=%s imageID=%s\n", status.Name, status.Ready, status.RestartCount, status.Image, status.ImageID)
			if status.State.Waiting != nil {
				_, _ = fmt.Fprintf(&b, "    waiting reason=%s message=%q\n", status.State.Waiting.Reason, status.State.Waiting.Message)
			}
			if status.State.Terminated != nil {
				_, _ = fmt.Fprintf(&b, "    terminated reason=%s exitCode=%d message=%q\n", status.State.Terminated.Reason, status.State.Terminated.ExitCode, status.State.Terminated.Message)
			}
		}

		events, err := s.frame.GetEvents(context.Background(), "Pod", pod.Name, pod.Namespace)
		if err != nil {
			_, _ = fmt.Fprintf(&b, "  list pod events: %v\n", err)
			continue
		}
		for _, event := range events.Items {
			_, _ = fmt.Fprintf(&b, "  event type=%s reason=%s message=%q\n", event.Type, event.Reason, event.Message)
		}
	}

	return b.String()
}

func (s *providerMockServer) prepareNamespace() error {
	if _, err := s.frame.GetNamespace(s.namespace); err == nil {
		if err := s.Cleanup(); err != nil {
			return err
		}
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	deadline := time.Now().Add(common.ResourceDeleteTimeout)
	for {
		err := s.frame.CreateNamespaceUntilDefaultServiceAccountReady(s.namespace, common.ServiceAccountReadyTimeout)
		if err == nil {
			return nil
		}
		if !apierrors.IsAlreadyExists(err) && !isNamespaceTerminatingError(err) {
			return err
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(time.Second)
	}
}

func (s *providerMockServer) createOrReplace(obj client.Object) error {
	if err := s.frame.CreateResource(obj); err == nil {
		return nil
	} else if !apierrors.IsAlreadyExists(err) {
		return err
	}

	if err := s.frame.DeleteResource(obj); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	Eventually(func() bool {
		err := s.frame.KClient.Get(context.Background(), types.NamespacedName{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		}, obj)
		return apierrors.IsNotFound(err)
	}).WithTimeout(common.ResourceDeleteTimeout).WithPolling(time.Second).Should(BeTrue())

	return s.frame.CreateResource(obj)
}

func isNamespaceTerminatingError(err error) bool {
	return apierrors.HasStatusCause(err, metav1.CauseType("NamespaceTerminating"))
}

func (s *providerMockServer) requestLocal(method, path string) ([]byte, error) {
	if s == nil || s.frame == nil || s.namespace == "" {
		return nil, e2e.ErrWrongInput
	}

	pods, err := s.frame.GetPodList(
		client.InNamespace(s.namespace),
		client.MatchingLabels(providerMockLabels()),
	)
	if err != nil {
		return nil, err
	}
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
		defer cancel()

		command := fmt.Sprintf("curl -fsS -X %s http://127.0.0.1:%d%s", method, providerMockPort, path)
		return s.frame.ExecCommandInPod(pod.Name, pod.Namespace, command, ctx)
	}
	return nil, fmt.Errorf("no running provider mock Pod found in namespace %s", s.namespace)
}

func providerMockService(namespace string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      providerMockName,
			Namespace: namespace,
			Labels:    providerMockLabels(),
		},
		Spec: corev1.ServiceSpec{
			Selector: providerMockLabels(),
			Ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: providerMockPort,
				},
			},
		},
	}
}

func providerMockDeployment(namespace string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      providerMockName,
			Namespace: namespace,
			Labels:    providerMockLabels(),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{
				MatchLabels: providerMockLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: providerMockLabels(),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "provider-mock",
							Image:           providerMockImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: providerMockPort,
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromString("http"),
									},
								},
								InitialDelaySeconds: 1,
								PeriodSeconds:       1,
								FailureThreshold:    30,
							},
						},
					},
				},
			},
		},
	}
}

func providerMockImage() string {
	if image := os.Getenv("E2E_IAAS_PROVIDER_MOCK_IMAGE"); image != "" {
		return image
	}
	return "spiderpool-iaas-provider-mock:latest"
}

func providerMockLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":      "iaas-provider-mock",
		"app.kubernetes.io/component": providerMockName,
	}
}
