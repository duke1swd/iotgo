package zoneMinderAPI

import (
	//"encoding/json"
	"fmt"
	//"github.com/go-resty/resty/v2"
	//"log"
	"os"
)

func GetStates() map[string]interface {} {
	var res interface{}

	_, err := Client.R().
		SetHeader("Content-Type", "application/json").
		SetQueryParams(map[string]string{
			"token": Token,
		}).
		SetResult(&res).
		Get("http://192.168.1.99:108/zm/api/states.json")

	if err != nil {
		fmt.Println("Cannot get states")
		fmt.Println("  Error = ", err)
		os.Exit(1)
	}

	return res.(map[string]interface{})
}

func GetState(state string) map[string]interface{} {
	var res interface{}

	resp, err := Client.R().
		SetHeader("Content-Type", "application/json").
		SetQueryParams(map[string]string{
			"token": Token,
		}).
		SetResult(&res).
		Get("http://192.168.1.99:108/zm/api/states/view/" + state + ".json")

	if err != nil || res == nil {
		fmt.Println("Cannot view state")
		fmt.Println("  Error = ", err)
		fmt.Println("  Resp = ", resp)
		os.Exit(1)
	}

	return res.(map[string]interface{})
}
