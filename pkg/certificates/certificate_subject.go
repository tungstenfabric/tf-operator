package certificates

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
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

	var ips []net.IP
	ips = append(ips, net.ParseIP(c.ip))
	for _, ip := range c.alternativeIPs {
		ips = append(ips, net.ParseIP(ip))
	}

	fullName := c.hostname
	if !strings.HasSuffix(c.hostname, c.domain) {
		fullName = fullName + "." + c.domain
	}
	altDNSNames := []string{}
	pn := ""
	for _, i := range strings.Split(fullName, ".") {
		if pn != "" {
			pn = pn + "."
		}
		pn = pn + i
		altDNSNames = append(altDNSNames, pn)
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
