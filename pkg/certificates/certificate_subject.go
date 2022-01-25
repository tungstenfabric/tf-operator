package certificates

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// CertificateSubject certificate subject
type CertificateSubject struct {
	name             string
	domain           string
	hostname         string
	ip               string
	alternativeIPs   []string
	alternativeNames []string
	clientAuth       bool
}

// NewSubject creates new certificate subject
func NewSubject(name, domain, hostname, ip string, alternativeIPs, alternativeNames []string, clientAuth bool) CertificateSubject {
	return CertificateSubject{
		name: name, domain: domain, hostname: hostname, ip: ip,
		alternativeIPs: alternativeIPs, alternativeNames: alternativeNames,
		clientAuth: clientAuth}
}

func contains(list []string, val string) bool {
	for _, v := range list {
		if val == v {
			return true
		}
	}
	return false
}

const (
	certValidityPeriod = 10 * 365 * 24 * time.Hour // 10 years
)

func GenerateSerialNumber() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, serialNumberLimit)
}

func HashPublicKey(key crypto.PublicKey) ([]byte, error) {
	b, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil, fmt.Errorf("Unable to hash key: %s", err)
	}
	h := sha1.New()
	if _, err = h.Write(b); err != nil {
		return nil, fmt.Errorf("Unable to hash key: %s", err)
	}
	return h.Sum(nil), nil
}

func splitFqdn(n string) []string {
	result := []string{}
	pn := ""
	for _, i := range strings.Split(n, ".") {
		if i != "" {
			if pn != "" {
				pn = pn + "."
			}
			pn = pn + i
			result = append(result, pn)
		}
	}
	return result
}

func (c CertificateSubject) getPrivKeyFromSecret(secret *corev1.Secret) (*rsa.PrivateKey, error) {
	caCertBlock, err := GetAndDecodePem(secret.Data, serverPrivateKeyFileName(c))
	if err != nil {
		return nil, err
	}
	var privKey *rsa.PrivateKey
	if privKey, err = x509.ParsePKCS1PrivateKey(caCertBlock.Bytes); err != nil {
		return nil, fmt.Errorf("Failed to parse private key: %w", err)
	}
	return privKey, nil
}

func (c CertificateSubject) generateCertificateTemplate(certPrivKey *rsa.PrivateKey) (x509.Certificate, error) {
	notBefore := time.Now()
	notAfter := notBefore.Add(certValidityPeriod)

	serialNumber, err := GenerateSerialNumber()
	if err != nil {
		return x509.Certificate{}, fmt.Errorf("fail to generate serial number: %w", err)
	}
	keyId, err := HashPublicKey(certPrivKey.Public())
	if err != nil {
		return x509.Certificate{}, fmt.Errorf("failed to get SubjectKeyId: %w", err)
	}

	fullName := c.hostname
	if !strings.HasSuffix(c.hostname, c.domain) {
		fullName = fullName + "." + c.domain
	}
	altDNSNames := splitFqdn(fullName)
	// WA for kubespray case: it sets hostname into /etc/hosts as "short+domain" into 1st position
	shortAndDomainName := splitFqdn(c.hostname)[0] + "." + c.domain
	if !contains(altDNSNames, shortAndDomainName) {
		altDNSNames = append(altDNSNames, shortAndDomainName)
	}

	ips := []net.IP{
		net.ParseIP(c.ip),
	}
	_ips := []string{
		c.ip,
	}
	for _, ip := range c.alternativeIPs {
		if !contains(_ips, ip) {
			ips = append(ips, net.ParseIP(ip))
			_ips = append(_ips, ip)
		}
	}

	additionalNames := c.alternativeNames
	for _, ip := range _ips {
		if names, err := net.LookupAddr(ip); err == nil {
			additionalNames = append(additionalNames, names...)
		}
	}
	for _, h := range additionalNames {
		for _, n := range splitFqdn(h) {
			if !contains(altDNSNames, n) {
				altDNSNames = append(altDNSNames, n)
			}
		}
	}

	certificateTemplate := x509.Certificate{
		SerialNumber:   serialNumber,
		SubjectKeyId:   keyId[:],
		AuthorityKeyId: keyId[:],
		Subject: pkix.Name{
			CommonName:         c.ip,
			Country:            []string{"US"},
			Province:           []string{"CA"},
			Locality:           []string{"San Francisco"},
			Organization:       []string{"Linux Foundation"},
			OrganizationalUnit: []string{"Tungsten Fabric"},
		},
		DNSNames:    altDNSNames,
		IPAddresses: ips,
		NotBefore:   notBefore,
		NotAfter:    notAfter,
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	}
	if c.clientAuth {
		certificateTemplate.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	} else {
		certificateTemplate.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	}

	return certificateTemplate, nil
}
