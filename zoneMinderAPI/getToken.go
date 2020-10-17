package zoneMinderAPI

import (
	"encoding/json"
	"fmt"
	"github.com/go-resty/resty/v2"
	"io/ioutil"
	"log"
	"os"
)

var (
	Token string
)

const defaultCredentialsFileName = "/usr/local/credentials/zoneminderapi.json"

func init() {
	var (
		authRes         interface{}
		credentialsJson interface{}
	)

	credentialsFileName := os.Getenv("CREDENTIALS")
	if len(credentialsFileName) < 1 {
		credentialsFileName = defaultCredentialsFileName
	}

	// Read the credentials
	credentialsBytes, err := ioutil.ReadFile(credentialsFileName)
	if err != nil {
		log.Fatalf("Cannot read ZoneMinder API credentials file %s\n", credentialsFileName)
	}

	// Parse them
	err = json.Unmarshal(credentialsBytes, &credentialsJson)
	if err != nil {
		log.Fatalf("Cannot parse ZoneMinder API credentials in file %s\n",
			credentialsFileName)
	}
	m, ok := credentialsJson.(map[string]interface{})
	if !ok {
		log.Fatalf("json credentials in ZoneMinder API credentials file %s not a map\n",
			credentialsFileName)
	}

	user, ok := m["user"].(string)
	if !ok {
		log.Fatalf("json credentials ZoneMinder API credentials file %s lacks user information\n",
			credentialsFileName)
	}

	pass, ok := m["pass"].(string)
	if !ok {
		log.Fatalf("json credentials ZoneMinder API credentials file %s lacks password information\n",
			credentialsFileName)
	}

	client := resty.New()

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetQueryParams(map[string]string{
			"user": user,
			"pass": pass,
		}).
		SetResult(&authRes).
		Get("http://192.168.1.99:108/zm/api/host/login.json")

	if err != nil {
		fmt.Println("Cannot log in to ZoneMinder")
		fmt.Println("  Error = ", err)
		fmt.Println("  Resp  = ", resp)
		os.Exit(1)
	}

	Token = authRes.(map[string]interface{})["access_token"].(string)
}

func GetToken() string {
	return Token
}
