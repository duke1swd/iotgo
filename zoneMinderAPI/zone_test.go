package zoneMinderAPI

import (
	"testing"
	"fmt"
)

func TestInit(t *testing.T) {
	s := GetToken()
	if s == "" {
		t.Errorf("token is empty")
	}
}

func TestListConfigs(t *testing.T) {
	c := GetConfigs()

	fmt.Println(c)
}
