package main

import (
	_ "embed"
	"grader/common"
	"log"
	"net/http"
	"time"
)

const (
	localhost = "http://localhost"
	project   = "sds-grader"
	grader    = "grader"
)

// topic is the Pub/Sub topic name, which will be set at build time.
var topic string

//go:embed activity1.json.enc
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
	containerNames := []string{"grafana", "prometheus", "apache-exporter", "apache", "node-exporter"}
	domain := common.EnsureHTTPPrefix(common.CollectInfo("domain", localhost))

	result := []bool{
		common.CheckResult(common.CheckRunningContainers(containerNames), "All specified containers are running."),
		common.CheckResult(common.CheckContainersOnSameNetwork(containerNames), "All containers are on the same network."),
		common.CheckResult(common.CheckHTTPStatus(domain+":8080", http.StatusOK, "Apache was not found via http://localhost:8080. Please check your Apache service."), "Apache is up and running at http://localhost:8080"),
		common.CheckResult(common.CheckHTTPStatus(domain+":9117/metrics", http.StatusOK, "Apache-exporter was not found via http://localhost:9117/metrics. Please check your Apache-exporter service."), "Apache-exporter is up and running at http://localhost:9117/metrics"),
		common.CheckResult(common.CheckHTTPStatus(domain+":3000", http.StatusOK, "Grafana was not found via http://localhost:3000. Please check your Grafana service."), "Grafana is up and running at http://localhost:3000"),
		common.CheckResult(common.CheckHTTPStatus(domain+":9090", http.StatusOK, "Prometheus was not found via http://localhost:9090. Please check your Prometheus service."), "Prometheus is up and running at http://localhost:9090"),
		common.CheckResult(common.CheckHTTPStatus(domain+":9100/metrics", http.StatusOK, "Node-exporter was not found via http://localhost:9100/metrics. Please check your Node-exporter service."), "Node-exporter is up and running at http://localhost:9100/metrics"),
		common.CheckResult(common.SendGetRequest(domain+":8080/server-status/?auto", "localhost"), "GET request shows result at http://localhost:8080."),
	}
	return result
}
