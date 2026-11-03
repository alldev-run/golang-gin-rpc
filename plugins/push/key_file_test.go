package push

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestAPNsAuthKeyPath(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "AuthKey.p8")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	client := &APNsClient{config: Config{APNs: APNsConfig{AuthKeyPath: path}}}
	if err := client.setupTokenAuth(); err != nil {
		t.Fatalf("setupTokenAuth() error = %v", err)
	}
	if client.privateKey == nil {
		t.Fatal("setupTokenAuth() did not load a private key")
	}
}

func TestFCMServiceAccountKeyPath(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error = %v", err)
	}
	credentials, err := json.Marshal(ServiceAccountCredentials{
		Type:        "service_account",
		ProjectID:   "test-project",
		PrivateKey:  string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		ClientEmail: "push@test-project.iam.gserviceaccount.com",
		TokenURI:    "https://oauth2.googleapis.com/token",
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "service-account.json")
	if err := os.WriteFile(path, credentials, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	client := &FCMClient{config: Config{FCM: FCMConfig{
		ProjectID:             "test-project",
		ServiceAccountKeyPath: path,
	}}}
	if err := client.setupTokenSource(); err != nil {
		t.Fatalf("setupTokenSource() error = %v", err)
	}
	if client.tokenSource == nil {
		t.Fatal("setupTokenSource() did not create a token source")
	}
}
