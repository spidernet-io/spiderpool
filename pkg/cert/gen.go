// Copyright 2022 Authors of Multus CNI
// SPDX-License-Identifier: Apache-2.0

package cert

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
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
	Namespace   string
	ServiceName string
	SecretName  string
	WebhookName string
	// in day
	CAExpiration int
	// in day
	CertExpiration int
	KeyBitLength   int
	CrtPath        string
	KeyPath        string

	clientSet kubernetes.Interface
	logger    *zap.Logger
}

// Gen generates cert pair
// If secret is empty, it will update secret and then update webhook configuration.
// Otherwise, it will load cert pair from secret, then write to local file path.
func (c *Cert) Gen(clientSet kubernetes.Interface, log *zap.Logger) error {
	c.clientSet = clientSet
	c.logger = log.Named("certificate-generator")
	cfssllog.SetLogger(c)

	c.logger.Sugar().Infof("generate server certificate for service %v/%v and webhook %s/%s, to secret %v/%v, with ca expire %v days and keyBitLength %v , cert expire %v days ",
		c.Namespace, c.ServiceName, c.Namespace, c.WebhookName, c.Namespace, c.SecretName, c.CAExpiration, c.KeyBitLength, c.CertExpiration)

	//  day unit to second
	CAExpiration := fmt.Sprintf("%ds", c.CAExpiration*24*3600)

	cli := c.clientSet.CoreV1().Secrets(c.Namespace)
	secret, err := cli.Get(context.Background(), c.SecretName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	canSkip, err := c.skip(secret)
	if err != nil {
		return err
	}
	if canSkip {
		return nil
	}

	s, caCert, err := c.genCACert(CAExpiration)
	if err != nil {
		return fmt.Errorf("error generating ca and signer: %v", err)
	}

	c.logger.Info("generate csr and private key")
	csrRaw, key, err := c.genCSR()
	if err != nil {
		return fmt.Errorf("error generating csr and private key: %v", err)
	}

	c.logger.Info("generate server certificate")
	certificate, err := s.Sign(signer.SignRequest{
		Request:   string(csrRaw),
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Duration(c.CertExpiration) * 24 * time.Hour),
	})
	if err != nil {
		return err
	}

	c.logger.Info("apply server certificate to secret")
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

	c.logger.Sugar().Infof("update mutating webhook %v/%v", c.Namespace, c.WebhookName)
	err = c.updateMutatingWebhook(caCert)
	if err != nil {
		return fmt.Errorf("error update mutating webhook configuration: %v", err)
	}

	c.logger.Sugar().Infof("update validating webhook %v/%v", c.Namespace, c.WebhookName)
	err = c.updateValidatingWebhook(caCert)
	if err != nil {
		return fmt.Errorf("error update validating webhook configuration: %v", err)
	}

	// wait secret is updated
	err = c.waitForCertDetailsUpdate()
	if err != nil {
		return fmt.Errorf("error occurred while waiting for secret creation: %v", err)
	}

	c.logger.Info("all resources created successfully")
	return nil
}

func (c *Cert) skip(secret *corev1.Secret) (bool, error) {
	tlsKey, ok1 := secret.Data["tls.key"]
	tlsCrt, ok2 := secret.Data["tls.crt"]
	caCrt, ok3 := secret.Data["ca.crt"]
	if ok1 && ok2 && ok3 {
		if len(tlsKey) != 0 && len(tlsCrt) != 0 && len(caCrt) != 0 {
			err := c.updateIfMutatingWebhookIfDifferent(caCrt)
			if err != nil {
				return true, fmt.Errorf("update mutating webhook if different ca.crt with error: %v", err)
			}
			err = c.updateValidatingWebhookIfDifferent(caCrt)
			if err != nil {
				return true, fmt.Errorf("update validating webhook if different ca.crt with error: %v", err)
			}

			expired, err := c.checkCertHasExpired(caCrt)
			if err != nil {
				c.logger.Info("secret certificate check passed, skipping regeneration.", zap.String("secret", c.SecretName))
				return true, err
			}

			c.logger.Info("certificate is expired, regenerating")
			if !expired {
				return true, nil
			}
		}
	}
	return false, nil
}

// checkCertHasExpired Check whether the x.509 certificate has expired.
func (c *Cert) checkCertHasExpired(certRaw []byte) (bool, error) {
	c.logger.Sugar().Infof("checking certificate expiration")
	now := time.Now()

	block, _ := pem.Decode(certRaw)
	if block == nil {
		return false, fmt.Errorf("failed to decode certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, err
	}

	if now.Before(cert.NotBefore) {
		return false, fmt.Errorf("the current time is earlier than the valid time of the certificate")
	}

	if now.After(cert.NotAfter) {
		return true, nil
	}

	return false, nil
}

func (c *Cert) genCACert(CAExpiration string) (*local.Signer, []byte, error) {
	certRequest := csr.New()
	c.logger.Info("key bit length", zap.String("key-bit-length", strconv.Itoa(c.KeyBitLength)))
	certRequest.KeyRequest = &csr.KeyRequest{A: "rsa", S: c.KeyBitLength}
	certRequest.CN = "Kubernetes NRI"
	certRequest.CA = &csr.CAConfig{Expiry: CAExpiration}
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
		strings.Join([]string{c.ServiceName, c.Namespace}, "."),
		strings.Join([]string{c.ServiceName, c.Namespace, "svc"}, "."),
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
	return wait.Poll(10*time.Second, 300*time.Second, c.checkSecret)
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

func (c *Cert) updateIfMutatingWebhookIfDifferent(cert []byte) error {
	cli := c.clientSet.AdmissionregistrationV1().MutatingWebhookConfigurations()
	hook, err := cli.Get(context.Background(), c.WebhookName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	var flag bool

	for i := range hook.Webhooks {
		if bytes.Equal(hook.Webhooks[i].ClientConfig.CABundle, cert) {
			flag = true
			hook.Webhooks[i].ClientConfig.CABundle = cert
		}
	}

	if flag {
		c.logger.Sugar().Infof("CABundle of mutating webhook is different from the ca.crt in secret, update mutating webhook")
		_, err = cli.Update(context.TODO(), hook, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
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

func (c *Cert) updateValidatingWebhookIfDifferent(cert []byte) error {
	hook, err := c.clientSet.AdmissionregistrationV1().ValidatingWebhookConfigurations().
		Get(context.Background(), c.WebhookName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	var flag bool

	for i := range hook.Webhooks {
		if bytes.Equal(hook.Webhooks[i].ClientConfig.CABundle, cert) {
			flag = true
			hook.Webhooks[i].ClientConfig.CABundle = cert
		}
	}

	if flag {
		c.logger.Sugar().Infof("CABundle of validating webhook is different from the ca.crt in secret, update validating webhook")
		_, err = c.clientSet.AdmissionregistrationV1().ValidatingWebhookConfigurations().
			Update(context.TODO(), hook, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
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
