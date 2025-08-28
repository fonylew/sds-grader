package main

import (
	_ "embed"
	"grader/common"
	"log"
	"net/http"
	"time"
)

const (
	networkName     = "todo-net"
	localhost       = "http://localhost"
	pageURL         = "http://localhost:3000"
	scriptURL       = "http://localhost:3000/static/js/bundle.js"
	localhost8000   = "http://localhost:8000"
	localhost9000   = "http://localhost:9000"
	todoServiceURL  = "http://localhost:8000/todo"
	notificationURL = "http://localhost:8000/notification"
	project         = "sds-grader"
)

// topic is the Pub/Sub topic name, which will be set at build time.
var topic string

//go:embed activity3.json.enc
var encryptedServiceAccountJSON []byte

func main() {
	currentTime := time.Now()
	log.Println("Current Timestamp: " + currentTime.Format(time.RFC3339))

	result := checkAllServices()

	finalResult := common.AllTrue(result)
	log.Printf("Result: %t\n", finalResult)

	if finalResult {
		common.HandleSuccess(currentTime, encryptedServiceAccountJSON, []byte(notificationURL[0:32]), project, topic)
	}
}

func checkAllServices() []bool {

	containerNames := []string{"webapp", "todo-service", "notification-service", "redis", "api-gateway"}

	result := []bool{
		common.CheckResult(common.CheckNetwork(networkName), "Network exists."),
		common.CheckResult(common.CheckRunningContainers(containerNames), "All specified containers are running."),
		common.CheckResult(common.CheckDockerComposeRunning(), "Docker compose is running."),
		common.CheckResult(common.CheckTodoWebapp(pageURL, scriptURL), "Todo app is working."),
		common.CheckResult(common.CheckHTTPStatus(localhost8000, http.StatusNotFound, "Please make sure that you set up services behind api-gateway."), ""),
		common.CheckResult(common.CheckHTTPStatus(localhost9000, http.StatusNotFound, "Please make sure that you expose ports only webapp and api-gateway."), ""),
		common.CheckResult(common.CheckHTTPStatus(todoServiceURL, http.StatusOK, "Todo-service was not found. Please check your api-gateway"), ""),
		common.CheckResult(common.CheckHTTPStatus(notificationURL, http.StatusOK, "Notification-service was not found. Please check your api-gateway"), "Notification-service found with api-gateway."),
	}
	return result
}
