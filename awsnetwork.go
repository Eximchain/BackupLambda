package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

func getMetadata(name string) (result string) {
	url := fmt.Sprintf("http://169.254.169.254/latest/meta-data/%s", name)
	response, err := http.Get(url)
	if err != nil {
		fmt.Printf("%s", err)
	} else {
		defer response.Body.Close()
		contents, err := ioutil.ReadAll(response.Body)
		if err != nil {
			fmt.Printf("%s", err)
			os.Exit(1)
		}
		result = string(contents)
	}
	return
}

func getPrivateIP() (result string) {
	result = getMetadata("local-ipv4")
	return
}

func getPublicIP() (result string) {
	result = getMetadata("public-ipv4")
	return
}
