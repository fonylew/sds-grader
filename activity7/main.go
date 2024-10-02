package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	networkName     = "todo-net"
	pageURL         = "http://localhost:3000"
	scriptURL       = "http://localhost:3000/static/js/bundle.js"
	localhost8000   = "http://localhost:8000"
	localhost9000   = "http://localhost:9000"
	todoServiceURL  = "http://localhost:8000/todo"
	notificationURL = "http://localhost:8000/notification"
	successPrefix   = "✅  / "
	errorPrefix     = "❌  X "
	spacePrefix     = "    "
)

func main() {
	currentTime := time.Now()
	log.Println(successPrefix + "Current Timestamp: " + currentTime.Format(time.RFC3339))
	check := true

	// 1. check Docker Network
	if err := checkNetwork(networkName); err != nil {
		log.Printf(errorPrefix+"%v\n", err)
		check = false
	} else {
		log.Printf(successPrefix+"Network %s exists.\n", networkName)
	}

	// 2. check Docker Container
	containerNames := []string{"webapp", "todo-service", "notification-service", "redis", "api-gateway"}
	if err := checkRunningContainers(containerNames); err != nil {
		log.Printf(errorPrefix+"%v\n", err)
		check = false
	} else {
		log.Println(successPrefix + "All specified containers are running.")
	}

	// 3. check Todo Webapp
	if err := checkTodoWebapp(pageURL, scriptURL); err != nil {
		log.Printf(errorPrefix+"%v\n", err)
		check = false
	}

	// 4. check API Gateway
	if !checkHTTPStatus(localhost8000, http.StatusNotFound) {
		check = false
		log.Println(errorPrefix + "Please make sure that you set up services behind api-gateway.")
	}
	if !checkHTTPStatus(localhost9000, http.StatusNotFound) {
		check = false
		log.Println(errorPrefix + "Please make sure that you expose ports only webapp and api-gateway.")
	}

	// 5. check Todo Service
	if !checkHTTPStatus(todoServiceURL, http.StatusOK) {
		log.Println(errorPrefix + "Todo-service was not found. Please check your api-gateway")
		check = false
	} else {
		if err := sendPostRequest(todoServiceURL, check); err != nil {
			log.Printf(errorPrefix+"Could not send a POST request to todo-service; %v\n", err)
		}
		log.Println(successPrefix + "Todo-service found with api-gateway.")
	}

	// 6. check Notification Service
	if !checkHTTPStatus(notificationURL, http.StatusOK) {
		log.Println(errorPrefix + "Notification-service was not found. Please check your api-gateway")
		check = false
	} else {
		log.Println(successPrefix + "Notification-service found with api-gateway.")
	}

	log.Printf("Result: %t\n", check)
}

func checkNetwork(networkName string) error {
	cmd := exec.Command("docker", "network", "ls")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to list docker networks: %v", err)
	}
	if !strings.Contains(string(output), networkName) {
		return fmt.Errorf("network %s does not exist", networkName)
	}
	return nil
}

func checkRunningContainers(containerNames []string) error {
	cmd := exec.Command("docker", "ps", "--format", "{{.Names}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to list running containers: %v", err)
	}

	runningContainers := string(output)
	for _, name := range containerNames {
		if !strings.Contains(runningContainers, name) {
			return fmt.Errorf("container %s does not exist", name)
		}
		log.Printf(spacePrefix+successPrefix+"Container %s exists.\n", name)
	}
	return nil
}

func checkTodoWebapp(pageURL, scriptURL string) error {
	titleFound, scriptFound, err := checkPageContent(pageURL)
	if err != nil {
		return fmt.Errorf("error checking todo webapp content: %v", err)
	}

	if !titleFound || !checkHTTPStatus(pageURL, http.StatusOK) {
		return fmt.Errorf("todo webapp was not found")
	}

	if !scriptFound {
		return fmt.Errorf("script not found in todo webapp")
	}

	if scriptExists, err := checkScriptExists(scriptURL); err != nil || !scriptExists {
		return fmt.Errorf("error checking script URL: %v", err)
	}

	return nil
}

func checkPageContent(url string) (bool, bool, error) {
	resp, err := http.Get(url)
	if err != nil {
		return false, false, fmt.Errorf("failed to fetch URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, false, fmt.Errorf("received non-200 response code: %d", resp.StatusCode)
	}

	var titleFound, scriptFound bool
	z := html.NewTokenizer(resp.Body)
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return titleFound, scriptFound, nil
		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			if t.Data == "title" {
				z.Next()
				if strings.TrimSpace(z.Token().Data) == "Uber To Do" {
					titleFound = true
				}
			}
			if t.Data == "script" {
				for _, attr := range t.Attr {
					if attr.Key == "src" && attr.Val == "/static/js/bundle.js" {
						scriptFound = true
					}
				}
			}
		}
	}
}

func checkScriptExists(url string) (bool, error) {
	resp, err := http.Get(url)
	if err != nil {
		return false, fmt.Errorf(errorPrefix+"failed to fetch URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf(errorPrefix+"received non-200 response code: %d", resp.StatusCode)
	}

	return true, nil
}

func checkHTTPStatus(url string, expectedStatus int) bool {
	resp, err := http.Get(url)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") && expectedStatus == http.StatusNotFound {
			return true
		}
		log.Printf(spacePrefix+errorPrefix+"Error checking URL %s: %v\n", url, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == expectedStatus {
		log.Printf(spacePrefix+successPrefix+"URL %s returns %d.\n", url, expectedStatus)
		return true
	} else {
		log.Printf(spacePrefix+errorPrefix+"URL %s does not return %d. Status code: %d\n", url, expectedStatus, resp.StatusCode)
		return false
	}
}

func sendPostRequest(url string, check bool) error {
	currentTime := time.Now()

	payload := map[string]interface{}{
		"title":     "grader",
		"detail":    "check time " + currentTime.String(),
		"completed": check,
		"duedate":   currentTime,
		"tags":      []string{},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf(errorPrefix+"error marshalling JSON: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf(errorPrefix+"error sending POST request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Println(spacePrefix + successPrefix + "POST request successful")
	} else {
		return fmt.Errorf(errorPrefix+"POST request failed with status: %d", resp.StatusCode)
	}

	return nil
}
