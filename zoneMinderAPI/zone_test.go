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
	_ = GetConfigs()
}

func TestGetStates(t *testing.T) {
	_ = GetStates()
}

func TestGetState(t *testing.T) {
	s := GetState("default")

	fmt.Println(s)
}
