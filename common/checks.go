package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	SuccessPrefix = "✅  / "
	ErrorPrefix   = "❌  X "
	SpacePrefix   = "    "
)

func CheckNetwork(networkName string) error {
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

func GetNetworkName(containerName string) (string, error) {
	cmd := exec.Command("docker", "inspect", "-f", "'{{range $key, $value := .NetworkSettings.Networks}}{{$key}}{{end}}'", containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get network name: %v", err)
	}
	return string(output), nil
}

func CheckContainersOnSameNetwork(containerNames []string) error {
	if len(containerNames) < 2 {
		return nil
	}

	firstNetwork, err := GetNetworkName(containerNames[0])
	if err != nil {
		return fmt.Errorf("error checking container network: %v", err)
	}

	for _, containerName := range containerNames[1:] {
		currentNetwork, err := GetNetworkName(containerName)
		if err != nil {
			return fmt.Errorf("containers are on different network: %v", err)
		}
		if currentNetwork != firstNetwork {
			return fmt.Errorf("Container %s is on network '%s', but should be on '%s'.", containerName, currentNetwork, firstNetwork)
		}
	}
	log.Printf(SuccessPrefix+"All containers are on the same network: %s", firstNetwork)
	return nil
}

func CheckDockerComposeRunning() error {
	cmd := exec.Command("docker", "compose", "ls")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run 'docker compose ls': %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return fmt.Errorf("no docker compose projects found")
	}

	for _, line := range lines[1:] {
		if strings.Contains(line, "running") {
			return nil // Found at least one running project
		}
	}
	return fmt.Errorf("no running docker compose projects found")
}

func CheckRunningContainers(containerNames []string) error {
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
		log.Printf(SpacePrefix+SuccessPrefix+"Container %s exists.\n", name)
	}
	return nil
}

func CheckTodoWebapp(pageURL, scriptURL string) error {
	titleFound, scriptFound, err := CheckPageContent(pageURL)
	if err != nil {
		return fmt.Errorf("error checking todo webapp content: %v", err)
	}

	if err = CheckHTTPStatus(pageURL, http.StatusOK, ""); err != nil {
		return fmt.Errorf("todo webapp was not found")
	}

	if !titleFound {
		return fmt.Errorf("todo webapp title was not found")
	}

	if !scriptFound {
		return fmt.Errorf("script not found in todo webapp")
	}

	if scriptExists, err := CheckScriptExists(scriptURL); err != nil || !scriptExists {
		return fmt.Errorf("error checking script URL: %v", err)
	}

	return nil
}

func CheckPageContent(url string) (bool, bool, error) {
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

func CheckScriptExists(url string) (bool, error) {
	resp, err := http.Get(url)
	if err != nil {
		return false, fmt.Errorf(ErrorPrefix+"failed to fetch URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf(ErrorPrefix+"received non-200 response code: %d", resp.StatusCode)
	}

	return true, nil
}

func CheckHTTPStatus(url string, expectedStatus int, errorMsg string) error {
	resp, err := http.Get(url)
	if err != nil {
		if strings.Contains(err.Error(), "refused") && expectedStatus == http.StatusNotFound {
			return nil
		}
		log.Printf(SpacePrefix+ErrorPrefix+"Error checking URL %s: %v\n", url, err)
		return fmt.Errorf(errorMsg)
	}
	defer resp.Body.Close()

	if resp.StatusCode == expectedStatus {
		log.Printf(SpacePrefix+SuccessPrefix+"URL %s returns %d.\n", url, expectedStatus)
		return nil
	} else {
		log.Printf(SpacePrefix+ErrorPrefix+"URL %s does not return %d. Status code: %d\n", url, expectedStatus, resp.StatusCode)
		return fmt.Errorf(errorMsg)
	}
}

func SendPostRequest(url string, check bool) error {
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
		return fmt.Errorf(ErrorPrefix+"error marshalling JSON: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf(ErrorPrefix+"error sending POST request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		log.Println(SpacePrefix + SuccessPrefix + "POST request successful")
	} else {
		return fmt.Errorf(ErrorPrefix+"POST request failed with status: %d", resp.StatusCode)
	}

	return nil
}

func SendGetRequest(url, word string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error sending GET request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}

	if strings.Contains(string(body), word) {
		return nil
	} else {
		return fmt.Errorf("word '%s' not found in the response from %s", word, url)
	}
}

func CheckNamespaceExists(namespace string) error {
	cmd := exec.Command("kubectl", "get", "namespace")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to execute kubectl command: %v", err)
	}

	outputStr := string(output)

	if strings.Contains(outputStr, namespace) {
		return nil
	}
	return fmt.Errorf("Couldn't find namespace %v.", err)
}

func CheckKubernetesResources(namespace string) error {
	wordsToFind := []string{"service/todo", "deployment.apps/todo", "pod/todo", "Running", "80"}

	cmd := exec.Command("kubectl", "get", "all", "-n", namespace)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Failed to execute kubectl command: %v", err)
	}

	outputStr := string(output)

	for _, word := range wordsToFind {
		if !strings.Contains(outputStr, word) {
			return fmt.Errorf("Missing word: %s\n", word)
		}
	}

	return nil
}

func CheckIngressExists(namespace string) error {
	cmd := exec.Command("kubectl", "get", "ingress", "-n", namespace)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to execute kubectl command: %v", err)
	}

	outputStr := string(output)

	if strings.Contains(outputStr, "ingress") {
		return nil
	}
	return fmt.Errorf("No ingress found in namespace %s", namespace)
}

func CheckFilePath(filePath string, suffix string) error {
	if filePath == "" {
		return fmt.Errorf("The file path cannot be empty.")
	}

	if !strings.HasSuffix(filePath, suffix) {
		return fmt.Errorf("The file path '%s' does not have the required suffix '%s'.", filePath, suffix)
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("The file path '%s' does not exist.", filePath)
	}

	return nil
}

func CheckCmdExitCode(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	if err := cmd.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return fmt.Errorf("command exited with code %d", exitError.ExitCode())
		}
		return fmt.Errorf("failed to run command: %v", err)
	}
	return nil
}
