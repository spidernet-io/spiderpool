// Copyright 2022 Authors of Multus CNI
// SPDX-License-Identifier: Apache-2.0

package cert

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/initca"
	cfssllog "github.com/cloudflare/cfssl/log"
	"github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// Cert define generate instance for spiderpool ca auto generated mode
type Cert struct {
	Namespace    string
	ServiceName  string
	SecretName   string
	WebhookName  string
	CAExpiration string
	KeyBitLength int
	CrtPath      string
	KeyPath      string

	clientSet kubernetes.Interface
	logger    *zap.Logger
}

func (c *Cert) autoFill() {
	if c.CAExpiration == "" {
		c.CAExpiration = "630720000s"
	}
	if c.KeyBitLength == 0 {
		c.KeyBitLength = 3072
	}
}

// Gen generates cert pair
// If secret is empty, it will update secret and then update webhook configuration.
// Otherwise, it will load cert pair from secret, then write to local file path.
func (c *Cert) Gen(clientSet kubernetes.Interface, log *zap.Logger) error {
	c.autoFill()
	c.clientSet = clientSet
	c.logger = log
	cfssllog.SetLogger(c)
	c.logger.Info("cert gen config",
		zap.String("secret", c.SecretName),
		zap.String("CAExpiration", c.CAExpiration),
		zap.String("keyBitLength", c.KeyPath),
	)

	cli := c.clientSet.CoreV1().Secrets(c.Namespace)
	secret, err := cli.Get(context.Background(), c.SecretName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	tlsKey, ok1 := secret.Data["tls.key"]
	tlsCrt, ok2 := secret.Data["tls.crt"]

	if ok1 && ok2 {
		if len(tlsKey) != 0 && len(tlsCrt) != 0 {
			c.logger.Info(
				"CA secret.Data len not equal 0, skip gen ca again.",
				zap.String("secret", c.SecretName),
			)
			return nil
		}
	}

	s, caCert, err := c.genCACert()
	if err != nil {
		return fmt.Errorf("error generating ca and signer: %v", err)
	}

	c.logger.Info("generate csr and private key")
	csrRaw, key, err := c.genCSR()
	if err != nil {
		return fmt.Errorf("error generating csr and private key: %v", err)
	}

	c.logger.Info("raw CSR and private key successfully created")
	certificate, err := s.Sign(signer.SignRequest{
		Request: string(csrRaw),
	})
	if err != nil {
		return err
	}

	err = c.updateSecret(certificate, key, caCert, secret)
	if err != nil {
		// As expected only one initContainer will succeed in creating secret.
		c.logger.Error("failed update secret", zap.Error(err))
		// wait secret is updated
		err = c.waitForCertDetailsUpdate()
		if err != nil {
			return fmt.Errorf("error occurred while waiting for secret creation: %v", err)
		}
		return nil
	}

	// wait secret is updated
	err = c.waitForCertDetailsUpdate()
	if err != nil {
		return fmt.Errorf("error occurred while waiting for secret creation: %v", err)
	}

	c.logger.Info("update webhook configurations")
	err = c.updateMutatingWebhook(caCert)
	if err != nil {
		return fmt.Errorf("error update mutating webhook configuration: %v", err)
	}

	err = c.updateValidatingWebhook(caCert)
	if err != nil {
		return fmt.Errorf("error update validating webhook configuration: %v", err)
	}

	c.logger.Info("mutating webhook configuration successfully created")
	c.logger.Info("all resources created successfully")
	return nil
}

func (c *Cert) genCACert() (*local.Signer, []byte, error) {
	certRequest := csr.New()
	c.logger.Info("key bit length", zap.String("key-bit-length", strconv.Itoa(c.KeyBitLength)))
	certRequest.KeyRequest = &csr.KeyRequest{A: "rsa", S: c.KeyBitLength}
	certRequest.CN = "Kubernetes NRI"
	certRequest.CA = &csr.CAConfig{Expiry: c.CAExpiration}
	cert, _, key, err := initca.New(certRequest)
	if err != nil {
		return nil, nil, fmt.Errorf("creating CA certificate failed: %v", err)
	}
	parsedKey, err := helpers.ParsePrivateKeyPEM(key)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing private key pem failed: %v", err)
	}
	parsedCert, err := helpers.ParseCertificatePEM(cert)
	if err != nil {
		return nil, nil, fmt.Errorf("parse certificate failed: %v", err)
	}
	s, err := local.NewSigner(parsedKey, parsedCert, signer.DefaultSigAlgo(parsedKey), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create signer: %v", err)
	}
	return s, cert, nil
}

func (c *Cert) genCSR() ([]byte, []byte, error) {
	c.logger.Info("generating Certificate Signing Request")
	certRequest := csr.New()
	certRequest.KeyRequest = &csr.KeyRequest{A: "rsa", S: c.KeyBitLength}
	certRequest.CN = strings.Join([]string{c.ServiceName, c.Namespace, "svc"}, ".")
	certRequest.Hosts = []string{
		c.ServiceName,
		strings.Join([]string{c.SecretName, c.Namespace}, "."),
		strings.Join([]string{c.SecretName, c.Namespace, "svc"}, "."),
	}
	return csr.ParseRequest(certRequest)
}

func (c *Cert) updateSecret(certificate, key, ca []byte, secret *corev1.Secret) error {
	secret.Data = map[string][]byte{
		"tls.key": key,
		"tls.crt": certificate,
		"ca.crt":  ca,
	}

	cli := c.clientSet.CoreV1().Secrets(c.Namespace)
	_, err := cli.Update(context.Background(), secret, metav1.UpdateOptions{})
	return err
}

func (c *Cert) waitForCertDetailsUpdate() error {
	return wait.Poll(5*time.Second, 300*time.Second, c.checkSecret)
}

func (c *Cert) checkSecret() (bool, error) {
	c.logger.Info("check secret")
	ctx := context.Background()
	cli := c.clientSet.CoreV1().Secrets(c.Namespace)
	secret, err := cli.Get(ctx, c.SecretName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	var key, crt []byte
	for k, v := range secret.Data {
		if k == "tls.key" {
			key = v
		} else if k == "tls.crt" {
			crt = v
		}
	}

	if len(key) == 0 || len(crt) == 0 {
		c.logger.Warn("crt length", zap.Int("len", len(key)))
		c.logger.Warn("key length", zap.Int("len", len(crt)))
		return false, fmt.Errorf("load crt and key from secret, get empty contents")
	}

	raw, err := os.ReadFile(c.CrtPath)
	if err != nil {
		return false, err
	}
	if !bytes.Equal(raw, crt) {
		return false, nil
	}
	raw, err = os.ReadFile(c.KeyPath)
	if err != nil {
		return false, err
	}
	if !bytes.Equal(raw, key) {
		return false, nil
	}

	c.logger.Info("cert is ready")
	return true, nil
}

func (c *Cert) updateMutatingWebhook(certificate []byte) error {
	cli := c.clientSet.AdmissionregistrationV1().MutatingWebhookConfigurations()

	hook, err := cli.Get(context.Background(), c.WebhookName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	for i := range hook.Webhooks {
		hook.Webhooks[i].ClientConfig.CABundle = certificate
	}

	_, err = cli.Update(context.TODO(), hook, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (c *Cert) updateValidatingWebhook(certificate []byte) error {
	cli := c.clientSet.AdmissionregistrationV1().ValidatingWebhookConfigurations()
	hook, err := cli.Get(context.Background(), c.WebhookName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	for i := range hook.Webhooks {
		hook.Webhooks[i].ClientConfig.CABundle = certificate
	}
	_, err = cli.Update(context.TODO(), hook, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

// Debug implements cfssl log driver
func (c *Cert) Debug(msg string) {
	c.logger.Debug(msg)
}

// Info implements cfssl log driver
func (c *Cert) Info(msg string) {
	c.logger.Info(msg)
}

// Warning implements cfssl log driver
func (c *Cert) Warning(msg string) {
	c.logger.Warn(msg)
}

// Err implements cfssl log driver
func (c *Cert) Err(msg string) {
	c.logger.Error(msg)
}

// Crit implements cfssl log driver
func (c *Cert) Crit(msg string) {
	c.logger.Fatal(msg)
}

// Emerg implements cfssl log driver
func (c *Cert) Emerg(msg string) {
	c.logger.Error(msg)
}
