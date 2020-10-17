package zoneMinderAPI

import (
	"testing"
)

func TestInit(t *testing.T) {
	s := GetToken()
	if s == "" {
		t.Errorf("token is empty")
	}
}
