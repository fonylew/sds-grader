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
	topic     = "activity2_cp"
)

//go:embed activity2.json.enc
var encryptedServiceAccountJSON []byte

func main() {
	currentTime := time.Now()
	log.Println("Current Timestamp: " + currentTime.Format(time.RFC3339))

	result := checkAllServices()

	finalResult := common.AllTrue(result)
	log.Printf("Result: %t\n", finalResult)

	if finalResult {
		common.HandleSuccess(currentTime, encryptedServiceAccountJSON, []byte(grader+localhost+project), project, topic)
	}
}

func checkAllServices() []bool {
	networkName := "monitoring"
	containerNames := []string{"grafana", "prometheus", "monitoring-node-exporter-1", "monitoring-node-exporter-2", "monitoring-node-exporter-3"}

	result := []bool{
		common.CheckResult(common.CheckRunningContainers(containerNames), "All specified containers are running."),
		common.CheckResult(common.CheckDockerComposeRunning(), "Docker compose is running."),
		common.CheckResult(common.CheckNetwork(networkName), "Network exists."),
		common.CheckResult(common.CheckHTTPStatus(localhost+":3000", http.StatusOK, "Grafana was not found via http://localhost:3000. Please check your Grafana service."), "Grafana is up and running at http://localhost:3000"),
		common.CheckResult(common.CheckHTTPStatus(localhost+":9090", http.StatusOK, "Prometheus was not found via http://localhost:9090. Please check your Prometheus service."), "Prometheus is up and running at http://localhost:9090"),
	}
	return result
}
