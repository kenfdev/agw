package compose

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadParsesServiceNetworksListSyntax(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "compose.yaml")
	writeComposeFile(t, path, "services:\n  dev:\n    build: .\n    networks:\n      - app\n      - acme_default\nnetworks:\n  app:\n    external: true\n    name: acme_default\n")

	file, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	service := file.Services["dev"]
	if !reflect.DeepEqual(service.Networks, []string{"app", "acme_default"}) {
		t.Fatalf("service networks = %#v", service.Networks)
	}
}

func TestLoadParsesServiceNetworksMappingSyntax(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "compose.yaml")
	writeComposeFile(t, path, "services:\n  dev:\n    build: .\n    networks:\n      app:\n        aliases:\n          - workspace\n      acme_default: {}\nnetworks:\n  app:\n    external: true\n    name: acme_default\n")

	file, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	service := file.Services["dev"]
	if !reflect.DeepEqual(service.Networks, []string{"app", "acme_default"}) {
		t.Fatalf("service networks = %#v", service.Networks)
	}
}

func writeComposeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
