package main

import (
	_ "embed"
	"grader/common"
	"log"
	"net/http"
	"time"
)

const (
	localhost        = "http://localhost"
	defaultNamespace = "default"
	project          = "sds-grader"
	grader           = "grader"
	topic            = "activity4"
)

//go:embed activity8.json.enc
var encryptedServiceAccountJSON []byte

func main() {
	currentTime := time.Now()
	log.Println("Current Timestamp: " + currentTime.Format(time.RFC3339))

	result := checkAllServices()

	finalResult := common.AllTrue(result)
	log.Printf("Result: %t\n", finalResult)

	if finalResult {
		common.HandleSuccess(currentTime, encryptedServiceAccountJSON, []byte(localhost+localhost), project, topic)
	}
}

func checkAllServices() []bool {
	namespace := common.CollectInfo("Kubernetes namespace", defaultNamespace)
	domain := common.EnsureHTTPPrefix(common.CollectInfo("domain", localhost))

	result := []bool{
		common.CheckResult(common.CheckNamespaceExists(namespace), "Namespace exists and can use kubectl command."),
		common.CheckResult(common.CheckKubernetesResources(namespace), "All Kubernetes resources are up and running."),
		common.CheckResult(common.CheckIngressExists(namespace), "Ingress resource exists in the namespace."),
		common.CheckResult(common.CheckHTTPStatus(domain, http.StatusOK, "Todo-service was not found via http://localhost. Please check your nginx-ingress service."), "Todo is up and running at http://localhost"),
		common.CheckResult(common.CheckHTTPStatus(domain+":8000", http.StatusNotFound, "Todo service at http://localhost:8000 should be inaccessible. Please check your nginx-ingress service."), "Todo service is inaccessible at http://localhost:8000"),
		common.CheckResult(common.CheckHTTPStatus(domain+":6379", http.StatusNotFound, "Redis service at http://localhost:6379 should be inaccessible. Please check your nginx-ingress service."), "Redis service is inaccessible at http://localhost:6379"),
		common.CheckResult(common.SendPostRequest(domain, true), "POST request to http://localhost was successful."),
		common.CheckResult(common.SendGetRequest(domain, grader), "GET request shows result from previous POST request to http://localhost."),
	}
	return result
}
