package certificates

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CertificateSubject certificate subject
type CertificateSubject struct {
	name           string
	domain         string
	hostname       string
	ip             string
	alternativeIPs []string
}

// NewSubject creates new certificate subject
func NewSubject(name, domain, hostname, ip string, alternativeIPs []string) CertificateSubject {
	return CertificateSubject{name: name, domain: domain, hostname: hostname, ip: ip, alternativeIPs: alternativeIPs}
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
	certKeyLength      = 2048
)

func generateSerialNumber() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, serialNumberLimit)
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

func (c CertificateSubject) generateCertificateTemplate(client client.Client) (x509.Certificate, *rsa.PrivateKey, error) {
	certPrivKey, err := rsa.GenerateKey(rand.Reader, certKeyLength)

	if err != nil {
		return x509.Certificate{}, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(certValidityPeriod)

	serialNumber, err := generateSerialNumber()
	if err != nil {
		return x509.Certificate{}, nil, fmt.Errorf("fail to generate serial number: %w", err)
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

	var ips []net.IP
	var _ips []string
	ips = append(ips, net.ParseIP(c.ip))
	for _, ip := range c.alternativeIPs {
		if !contains(_ips, ip) {
			ips = append(ips, net.ParseIP(ip))
			_ips = append(_ips, ip)
		}
	}

	for _, ip := range _ips {
		hostNames, err := net.LookupAddr(ip)
		if err != nil {
			continue
		}
		for _, h := range hostNames {
			for _, n := range splitFqdn(h) {
				if !contains(altDNSNames, n) {
					altDNSNames = append(altDNSNames, n)
				}
			}
		}
	}

	certificateTemplate := x509.Certificate{
		SerialNumber: serialNumber,
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
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	return certificateTemplate, certPrivKey, nil
}
