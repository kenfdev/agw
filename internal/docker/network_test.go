package docker

import (
	"reflect"
	"testing"
)

func TestListNetworksParsesNameAndLabelsFromJSON(t *testing.T) {
	old := runNetworkList
	defer func() { runNetworkList = old }()

	runNetworkList = func() ([]byte, error) {
		return []byte(`{"Name":"acme_default","Labels":"com.docker.compose.project=acme,com.docker.compose.network=default"}
{"Name":"isolated","Labels":"foo=bar"}`), nil
	}

	networks, err := CLI{}.ListNetworks()
	if err != nil {
		t.Fatalf("ListNetworks() error = %v", err)
	}

	want := []Network{
		{Name: "acme_default", Labels: map[string]string{"com.docker.compose.project": "acme", "com.docker.compose.network": "default"}},
		{Name: "isolated", Labels: map[string]string{"foo": "bar"}},
	}
	if !reflect.DeepEqual(networks, want) {
		t.Fatalf("ListNetworks() got %#v want %#v", networks, want)
	}
}
