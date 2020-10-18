package zoneMinderAPI

import (
	//"encoding/json"
	"fmt"
	//"github.com/go-resty/resty/v2"
	//"log"
	"os"
)

func GetConfigs() map[string]interface {} {
	var res interface{}

	_, err := Client.R().
		SetHeader("Content-Type", "application/json").
		SetQueryParams(map[string]string{
			"token": Token,
		}).
		SetResult(&res).
		Get("http://192.168.1.99:108/zm/api/configs.json")

	if err != nil {
		fmt.Println("Cannot get configs")
		fmt.Println("  Error = ", err)
		os.Exit(1)
	}

	return res.(map[string]interface{})
}
