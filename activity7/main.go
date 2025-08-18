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
	pageURL         = "http://localhost:3000"
	scriptURL       = "http://localhost:3000/static/js/bundle.js"
	localhost8000   = "http://localhost:8000"
	localhost9000   = "http://localhost:9000"
	todoServiceURL  = "http://localhost:8000/todo"
	notificationURL = "http://localhost:8000/notification"
	project         = "sds-grader"
	topic           = "activity7"
)

//go:embed activity7.json.enc
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
	result := []bool{common.CheckResult(common.CheckNetwork(networkName), "Network exists.")}
	containerNames := []string{"webapp", "todo-service", "notification-service", "redis", "api-gateway"}
	result = append(result, common.CheckResult(common.CheckRunningContainers(containerNames), "All specified containers are running."))
	result = append(result, common.CheckResult(common.CheckTodoWebapp(pageURL, scriptURL), ""))
	result = append(result, common.CheckResult(common.CheckHTTPStatus(localhost8000, http.StatusNotFound, "Please make sure that you set up services behind api-gateway."), ""))
	result = append(result, common.CheckResult(common.CheckHTTPStatus(localhost9000, http.StatusNotFound, "Please make sure that you expose ports only webapp and api-gateway."), ""))
	result = append(result, common.CheckResult(common.CheckHTTPStatus(todoServiceURL, http.StatusOK, "Todo-service was not found. Please check your api-gateway"), ""))
	if result[len(result)-1] {
		result = append(result, common.CheckResult(common.SendPostRequest(todoServiceURL, common.AllTrue(result)), "Todo-service found with api-gateway."))
	}
	result = append(result, common.CheckResult(common.CheckHTTPStatus(notificationURL, http.StatusOK, "Notification-service was not found. Please check your api-gateway"), "Notification-service found with api-gateway."))
	return result
}
