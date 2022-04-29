// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Spiderpool

package framework

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/remotecommand"
)

const SpiderLabelSelector = "app.kubernetes.io/name: spiderpool"

type Option func(f *CLusterConfig)

type Framework struct {
	BaseName        string
	SystemNameSpace string
	KubeClientSet   kubernetes.Interface
	KubeConfig      *rest.Config
	CLusterConfig   *CLusterConfig
}

// CLusterConfig the install information about cluster
// TODO: CLusterConfig  more cluster information should be included
type CLusterConfig struct {
	IpFamily string
	Multus   bool
	Spider   bool
}

// NewFramework init Framework struct
func NewFramework(baseName, kubeconfig string, clusterOption ...Option) *Framework {
	if kubeconfig == "" {
		klog.Fatal("kubeconfig must be specify")
	}

	f := &Framework{
		BaseName:      baseName,
		CLusterConfig: &CLusterConfig{},
	}

	for _, option := range clusterOption {
		option(f.CLusterConfig)
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Fatal(err)
	}
	f.KubeConfig = cfg

	cfg.QPS = 1000
	cfg.Burst = 2000
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatal(err)
	}

	f.KubeClientSet = kubeClient

	return f
}

// WithIpFamily mutates the inner state to set the
// IpFamily attribute
func WithIpFamily(ipFamily string) Option {
	return func(f *CLusterConfig) {
		f.IpFamily = ipFamily
	}
}

// WithMultus mutates the inner state to set the
// Multus attribute
func WithMultus(install bool) Option {
	return func(f *CLusterConfig) {
		f.Multus = install
	}
}

// WithSpider mutates the inner state to set the
// Spider attribute
func WithSpider(install bool) Option {
	return func(f *CLusterConfig) {
		f.Multus = install
	}
}
func (f *Framework) WaitPodReady(pod, namespace string) (*corev1.Pod, error) {
	for {
		time.Sleep(1 * time.Second)
		p, err := f.KubeClientSet.CoreV1().Pods(namespace).Get(context.Background(), pod, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if p.Status.Phase == "Running" && p.Status.Reason != "" {
			return p, nil
		}

		switch getPodStatus(*p) {
		case Completed:
			return nil, fmt.Errorf("pod already completed")
		case Running:
			return p, nil
		case Initing, Pending, PodInitializing, ContainerCreating, Terminating:
			continue
		default:
			klog.Info(p.String())
			return nil, fmt.Errorf("pod status failed")
		}
	}
}

func (f *Framework) WaitPodDeleted(pod, namespace string) error {
	for {
		time.Sleep(1 * time.Second)
		p, err := f.KubeClientSet.CoreV1().Pods(namespace).Get(context.Background(), pod, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		if status := getPodStatus(*p); status != Terminating {
			return fmt.Errorf("unexpected pod status: %s", status)
		}
	}
}

const (
	Running           = "Running"
	Pending           = "Pending"
	Completed         = "Completed"
	ContainerCreating = "ContainerCreating"
	PodInitializing   = "PodInitializing"
	Terminating       = "Terminating"
	Initing           = "Initing"
)

func getPodContainerStatus(pod corev1.Pod, reason string) string {
	for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
		container := pod.Status.ContainerStatuses[i]

		if container.State.Waiting != nil && container.State.Waiting.Reason != "" {
			reason = container.State.Waiting.Reason
		} else if container.State.Terminated != nil && container.State.Terminated.Reason != "" {
			reason = container.State.Terminated.Reason
		} else if container.State.Terminated != nil && container.State.Terminated.Reason == "" {
			if container.State.Terminated.Signal != 0 {
				reason = fmt.Sprintf("Signal:%d", container.State.Terminated.Signal)
			} else {
				reason = fmt.Sprintf("ExitCode:%d", container.State.Terminated.ExitCode)
			}
		}
	}
	return reason
}

func getPodStatus(pod corev1.Pod) string {
	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}
	initializing, reason := getPodInitStatus(pod, reason)
	if !initializing {
		reason = getPodContainerStatus(pod, reason)
	}

	if pod.DeletionTimestamp != nil && pod.Status.Reason == "NodeLost" {
		reason = "Unknown"
	} else if pod.DeletionTimestamp != nil {
		reason = "Terminating"
	}
	return reason
}

func getPodInitStatus(pod corev1.Pod, reason string) (bool, string) {
	initializing := false
	for i := range pod.Status.InitContainerStatuses {
		container := pod.Status.InitContainerStatuses[i]
		switch {
		case container.State.Terminated != nil && container.State.Terminated.ExitCode == 0:
			continue
		case container.State.Terminated != nil:
			// initialization is failed
			if len(container.State.Terminated.Reason) == 0 {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Init:Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("Init:ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else {
				reason = "Init:" + container.State.Terminated.Reason
			}
			initializing = true
		case container.State.Waiting != nil && len(container.State.Waiting.Reason) > 0 && container.State.Waiting.Reason != "PodInitializing":
			reason = "Initing:" + container.State.Waiting.Reason
			initializing = true
		default:
			reason = fmt.Sprintf("Initing:%d/%d", i, len(pod.Spec.InitContainers))
			initializing = true
		}
		break
	}
	return initializing, reason
}

func (f *Framework) ExecToPodThroughAPI(command, containerName, podName, namespace string, stdin io.Reader) (string, string, error) {
	req := f.KubeClientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return "", "", fmt.Errorf("error adding to scheme: %v", err)
	}

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   strings.Fields(command),
		Container: containerName,
		Stdin:     stdin != nil,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, parameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(f.KubeConfig, "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("error while creating Executor: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		return "", "", fmt.Errorf("error in Stream: %v", err)
	}

	return stdout.String(), stderr.String(), nil
}
