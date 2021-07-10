package certificates

import (
	"bytes"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"reflect"
)

const (
	CertificatePemType = "CERTIFICATE"
	PrivateKeyPemType  = "RSA PRIVATE KEY"
)

func GetAndDecodePem(data map[string][]byte, key string) (*pem.Block, error) {
	pemData, ok := data[key]
	if !ok {
		return nil, fmt.Errorf("pem block %s not found data map %+v", key, reflect.ValueOf(data).MapKeys())
	}
	pemBlock, _ := pem.Decode(pemData)
	return pemBlock, nil
}

func EncodeInPemFormat(buff []byte, pemType string) ([]byte, error) {
	pemFormatBuffer := new(bytes.Buffer)
	_ = pem.Encode(pemFormatBuffer, &pem.Block{
		Type:  pemType,
		Bytes: buff,
	})
	return ioutil.ReadAll(pemFormatBuffer)
}
