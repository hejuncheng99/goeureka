package goeureka

//File  : client.go
//Author: Simon
//Describe: eureka client for server
//Date  : 2020/12/3

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	Vport              string
	username           string                    // login username
	password           string                    // login password
	eurekaPath         = "/eureka/apps/"         // define eureka path
	discoveryServerUrl = "http://127.0.0.1:8761" // local eureka url
)

// RegisterClient register this app at the Eureka server
// params: eurekaUrl, eureka server url
// params: appName define your app name what you want
// params: port app instance port
// params: securePort
func RegisterClient(eurekaUrl string, localip string, appName string, port string, securePort string, opt map[string]string) {
	eurekaUrl = strings.Trim(eurekaUrl, "/")
	user, _ := opt["username"]
	passd, _ := opt["password"]
	if len(user) > 1 && len(passd) > 1 {
		username = user
		password = passd
		discoveryServerUrl = eurekaUrl
	} else if len(user) == 0 && len(passd) == 0 {
		discoveryServerUrl = eurekaUrl
	} else {
		panic("username or password is valid!")
	}
	RegisterLocal(appName, localip, port, securePort)
}

// RegisterLocal :register your app at the local Eureka server
// params: port app instance port
// params: securePort
// Register new application instance
// POST /eureka/v2/apps/appID
// Input: JSON/XML payload HTTP Code: 204 on success
func RegisterLocal(appName string, localip string, port string, securePort string) {
	appName = strings.ToUpper(appName)
	Vport = port
	cfg := newConfig(appName, localip, port, securePort)

	// define Register request
	registerAction := RequestAction{
		Url:         discoveryServerUrl + eurekaPath + appName,
		Method:      "POST",
		ContentType: "application/json;charset=UTF-8",
		Body:        cfg,
	}
	var result bool
	// loop send heart beat every 5s
	for {
		result = isDoHttpRequest(registerAction)
		if result {
			log.Println("Registration OK")
			handleSigterm(appName)
			go startHeartbeat(appName, localip)
			break
		} else {
			log.Println("Registration attempt of " + appName + " failed...")
			time.Sleep(time.Second * 5)
		}
	}

}

// GetServiceInstances is a function query all instances by appName
// params: appName
// Query for all appID instances
// GET /eureka/v2/apps/appID
// HTTP Code: 200 on success Output: JSON
func GetServiceInstances(appName string) ([]Instance, error) {
	var m ServiceResponse
	appName = strings.ToUpper(appName)
	// define get instance request
	requestAction := RequestAction{
		Url:         discoveryServerUrl + eurekaPath + appName,
		Method:      "GET",
		Accept:      "application/json;charset=UTF-8",
		ContentType: "application/json;charset=UTF-8",
	}
	log.Println("Query Eureka server using URL: " + requestAction.Url)
	bytes, err := executeQuery(requestAction)
	if len(bytes) == 0 {
		log.Printf("Query Eureka Response is None")
		return nil, err
	}
	if err != nil {
		return nil, err
	} else {
		//log.Println("Response from Eureka:\n" + string(bytes))
		err := json.Unmarshal(bytes, &m)
		if err != nil {
			log.Printf("Parse JSON Error(%v) from Eureka Server Response", err.Error())
			return nil, err
		}
		return m.Application.Instance, nil
	}
}

// GetServiceInstanceIdWithappName : in this function, we can get InstanceId by appName
// Notes:
//		1. use sendheartbeat
// 		2. deregister
// return instanceId, lastDirtyTimestamp
func GetInfoWithappName(appName string) (string, string, error) {
	appName = strings.ToUpper(appName)
	instances, err := GetServiceInstances(appName)
	if err != nil {
		return "", "", err
	}
	for _, ins := range instances {
		if ins.App == appName {
			return ins.InstanceId, ins.LastDirtyTimestamp, nil
		}
	}
	return "", "", err
}

// GetServices :get all services for eureka
// Notes: /gotest/TestGetServiceInstances has a test case
// Query for all instances
// GET /eureka/v2/apps
// HTTP Code: 200 on success Output: JSON
func GetServices() ([]Application, error) {
	var m ApplicationsRootResponse
	requestAction := RequestAction{
		Url:         discoveryServerUrl + eurekaPath,
		Method:      "GET",
		Accept:      "application/json;charset=UTF-8",
		ContentType: "application/json;charset=UTF-8",
	}
	log.Println("Query all services URL:" + requestAction.Url)
	bytes, err := executeQuery(requestAction)
	if err != nil {
		return nil, err
	} else {
		//log.Println("query all services response from Eureka:\n" + string(bytes))
		err := json.Unmarshal(bytes, &m)
		if err != nil {
			log.Printf("Parse JSON Error(%v) from Eureka Server Response", err.Error())
			return nil, err
		}
		return m.Resp.Applications, nil
	}
}

// startHeartbeat function will start as goroutine, will loop indefinitely until application exits.
// params: appName
func startHeartbeat(appName string, localip string) {
	for {
		time.Sleep(time.Second * 30)
		Sendheartbeat(appName, localip)
	}
}

// heartbeat Send application instance heartbeat
// PUT /eureka/v2/apps/appID/instanceID
//HTTP Code:
//* 200 on success
//* 404 if instanceID doesn’t exist
func heartbeat(appName string, localip string) {
	appName = strings.ToUpper(appName)
	instanceId, lastDirtyTimestamp, err := GetInfoWithappName(appName)
	if instanceId == "" {
		log.Printf("instanceId is None , Please check at (%v) \n", discoveryServerUrl)
		return
	}
	if err != nil {
		log.Printf("Can't get instanceId from Eureka server by appName \n")
		return
	} else {
		if localip != "" {
			// "58.49.122.210:GOLANG-SERVER:8889"
			instanceId = localip + ":" + appName + ":" + Vport
		}
		heartbeatAction := RequestAction{
			//http://127.0.0.1:8761/eureka/apps/TORNADO-SERVER/127.0.0.1:tornado-server:3333/status?value=UP&lastDirtyTimestamp=1607321668458
			Url:         discoveryServerUrl + eurekaPath + appName + "/" + instanceId + "/status?value=UP&lastDirtyTimestamp=" + lastDirtyTimestamp,
			Method:      "PUT",
			ContentType: "application/json;charset=UTF-8",
		}
		log.Println("Sending heartbeat to " + heartbeatAction.Url)
		isDoHttpRequest(heartbeatAction)
	}
}

// Sendheartbeat is a test case for heartbeat
// you can test this function: send a heart beat to eureka server
func Sendheartbeat(appName string, localip string) {
	heartbeat(appName, localip)
}

// deregister De-register application instance
// DELETE /eureka/v2/apps/appID/instanceID
// HTTP Code: 200 on success
func deregister(appName string) {
	appName = strings.ToUpper(appName)
	log.Println("Trying to deregister application " + appName)
	instanceId, _, _ := GetInfoWithappName(appName)
	// cancel registerion
	deregisterAction := RequestAction{
		//http://127.0.0.1:8761/eureka/apps/TORNADO-SERVER/127.0.0.1:tornado-server:3333/status?value=UP&lastDirtyTimestamp=1607321668458
		Url:         discoveryServerUrl + eurekaPath + appName + "/" + instanceId,
		ContentType: "application/json;charset=UTF-8",
		Method:      "DELETE",
	}
	isDoHttpRequest(deregisterAction)
	log.Println("Deregistered App: " + appName)
}

// handleSigterm when has signal os Interrupt eureka would exit
func handleSigterm(appName string) {
	c := make(chan os.Signal, 1)
	// Ctr+C shut down
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		<-c
		deregister(appName)
		os.Exit(1)
	}()
}
