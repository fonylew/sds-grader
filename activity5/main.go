package main

import (
	_ "embed"
	"grader/common"
	"log"
	"net/http"
	"path/filepath"
	"time"
)

const (
	localhost = "http://localhost"
	project   = "sds-grader"
	grader    = "grader"
)

// topic is the Pub/Sub topic name, which will be set at build time.
var topic string

//go:embed activity5.json.enc
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
	containerNames := []string{"todo-service", "redis"}
	domain := localhost
	tfFilePath := common.CollectInfo("[REQUIRED] Terraform file path", "")

	result := []bool{
		common.CheckResult(common.CheckFilePath(tfFilePath, ".tf"), "Terraform file path is exist."),
		common.CheckResult(common.CheckCmdExitCode("terraform", "version"), "Terraform is installed."),
		common.CheckResult(common.CheckCmdExitCode("terraform", "-chdir="+filepath.Dir(tfFilePath), "init"), "Terraform is initialized."),
		common.CheckResult(common.CheckCmdExitCode("terraform", "-chdir="+filepath.Dir(tfFilePath), "plan"), "Terraform plan is generated."),
		common.CheckResult(common.CheckRunningContainers(containerNames), "All specified containers are running."),
		common.CheckResult(common.CheckHTTPStatus(domain+":8000", http.StatusOK, "Todo-service was not found via http://localhost:8000. Please check your nginx-ingress service."), "Todo is up and running at http://localhost:8000"),
		common.CheckResult(common.SendPostRequest(domain+":8000", true), "POST request to http://localhost:8000 was successful."),
		common.CheckResult(common.SendGetRequest(domain+":8000", grader), "GET request shows result from previous POST request to http://localhost:8000."),
	}
	return result
}
