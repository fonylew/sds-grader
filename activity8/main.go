package main

import (
	"bufio"
	"bytes"
	"cloud.google.com/go/pubsub"
	"context"
	"crypto/aes"
	"crypto/cipher"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"google.golang.org/api/option"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/host"
)

const (
	localhost        = "http://localhost"
	defaultNamespace = "default"
	project          = "sds-grader"
	grader           = "grader"
	topic            = "activity8"
	successPrefix    = "✅  / "
	errorPrefix      = "❌  X "
	spacePrefix      = "    "
)

//go:embed activity8.json.enc
var encryptedServiceAccountJSON []byte

func main() {
	currentTime := time.Now()
	log.Println("Current Timestamp: " + currentTime.Format(time.RFC3339))

	result := checkAllServices()

	finalResult := allTrue(result)
	log.Printf("Result: %t\n", finalResult)

	if finalResult {
		handleSuccess(currentTime)
	}
}

func checkAllServices() []bool {
	namespace := collectNamespace()

	result := []bool{
		checkResult(checkNamespaceExists(namespace), "Namespace exists and can use kubectl command."),
		checkResult(checkKubernetesResources(namespace), "All Kubernetes resources are up and running."),
		checkResult(checkIngressExists(namespace), "Ingress resource exists in the namespace."),
		checkResult(checkHTTPStatus(localhost, http.StatusOK, "Todo-service was not found via http://localhost. Please check your nginx-ingress service."), "Todo is up and running at http://localhost"),
		checkResult(sendPostRequest(localhost, true), "POST request to http://localhost was successful."),
		checkResult(sendGetRequest(localhost, grader), "GET request shows result from previous POST request to http://localhost."),
	}
	return result
}

func collectNamespace() string {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("👉 Enter Kubernetes Namespace (leave blank for '%s'): ", defaultNamespace)
	namespace, _ := reader.ReadString('\n')
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		namespace = defaultNamespace
	}

	return namespace
}

func checkNamespaceExists(namespace string) error {
	// Execute the kubectl command to get namespaces
	cmd := exec.Command("kubectl", "get", "namespace")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to execute kubectl command: %v", err)
	}

	// Convert the output to a string
	outputStr := string(output)

	// Check if the namespace is present in the output
	if strings.Contains(outputStr, namespace) {
		return nil
	}
	return fmt.Errorf("Couldn't find namespace %v.", err)
}

func checkKubernetesResources(namespace string) error {
	// Define the words to search for in the output
	wordsToFind := []string{
		"service/todo", "service/nginx",
		"deployment.apps/nginx", "deployment.apps/todo",
		"pod/redis", "pod/todo", "ingress",
		"Running", "LoadBalancer", "80",
	}

	// Execute the kubectl command
	cmd := exec.Command("kubectl", "get", "all", "-n", namespace)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Failed to execute kubectl command: %v", err)
	}

	// Convert the output to a string
	outputStr := string(output)

	// Check if all words are present in the output
	for _, word := range wordsToFind {
		if !strings.Contains(outputStr, word) {
			return fmt.Errorf("Missing word: %s\n", word)
		}
	}

	return nil
}

func checkIngressExists(namespace string) error {
	// Execute the kubectl command to get ingress resources
	cmd := exec.Command("kubectl", "get", "ingress", "-n", namespace)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to execute kubectl command: %v", err)
	}

	// Convert the output to a string
	outputStr := string(output)

	// Check if the output contains the word "ingress"
	if strings.Contains(outputStr, "ingress") {
		return nil
	}
	return fmt.Errorf("No ingress found in namespace %s", namespace)
}

func handleSuccess(currentTime time.Time) {
	log.Printf("🎉 Looks good! Please enter your StudentID and Full name below\n")
	id, name := collectUserInfo()
	hostName, user, osFamily, version, up, ip, pub := collectMachineInfo()

	acc, err := decryptJSON([]byte(localhost + localhost))
	handleError(err, "Failed to decrypt JSON")

	ctx := context.Background()
	pubsubClient, err := pubsub.NewClient(ctx, project, option.WithCredentialsJSON(acc))
	handleError(err, "Failed to create Pub/Sub client")
	defer pubsubClient.Close()
	message := createMessage(currentTime, id, name, hostName, user, osFamily, version, up, ip, pub)

	pub_status := publishMessage(ctx, pubsubClient, topic, message)
	handleError(pub_status, "Failed to publish message")

	log.Println("🎉🎉🎉 Congratulations! You have completed the activity 🎉🎉🎉")
	log.Printf("⚠️ Don't forget! you still need to submit your assignment via MyCourseVille ⚠️\n")
}

func handleError(err error, message string) {
	if err != nil {
		log.Fatalf("%s: %v", message, err)
	}
}

func checkResult(err error, passMessage string) bool {
	if err != nil {
		log.Printf("%s%v\n", errorPrefix, err)
		return false
	}
	if passMessage != "" {
		log.Printf("%s%s\n", successPrefix, passMessage)
	}
	return true
}

func allTrue(arr []bool) bool {
	for _, v := range arr {
		if !v {
			return false
		}
	}
	return true
}

func decryptJSON(key []byte) ([]byte, error) {
	// Load the encryption key from an environment variable
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid encryption key length: %d. Expected 32 bytes for AES-256", len(key))
	}

	// Decrypt the embedded file
	decoded, err := base64.StdEncoding.DecodeString(string(encryptedServiceAccountJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %v", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %v", err)
	}

	nonceSize := gcm.NonceSize()
	nonce, ciphertext := decoded[:nonceSize], decoded[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %v", err)
	}

	return plaintext, nil
}

func checkHTTPStatus(url string, expectedStatus int, errorMsg string) error {
	resp, err := http.Get(url)
	if err != nil {
		if strings.Contains(err.Error(), "refused") && expectedStatus == http.StatusNotFound {
			return nil
		}
		log.Printf(spacePrefix+errorPrefix+"Error checking URL %s: %v\n", url, err)
		return fmt.Errorf(errorMsg)
	}
	defer resp.Body.Close()

	if resp.StatusCode == expectedStatus {
		log.Printf(spacePrefix+successPrefix+"URL %s returns %d.\n", url, expectedStatus)
		return nil
	} else {
		log.Printf(spacePrefix+errorPrefix+"URL %s does not return %d. Status code: %d\n", url, expectedStatus, resp.StatusCode)
		return fmt.Errorf(errorMsg)
	}
}

func collectUserInfo() (int, string) {
	reader := bufio.NewReader(os.Stdin)
	var studentID int
	var err error
	for {
		fmt.Print("👉 StudentID: ")
		studentIDStr, _ := reader.ReadString('\n')
		studentIDStr = strings.TrimSpace(studentIDStr)

		studentID, err = strconv.Atoi(studentIDStr)
		if err == nil {
			break
		}
		log.Println(errorPrefix + "Invalid StudentID. Please enter a valid integer.")
	}

	fmt.Print("👉 Full Name (TH): ")
	fullName, _ := reader.ReadString('\n')
	fullName = strings.TrimSpace(fullName)
	return studentID, fullName
}

type Message struct {
	Field1  time.Time `json:"timestamp"`
	Field2  int       `json:"id"`
	Field3  string    `json:"name"`
	Field4  string    `json:"host"`
	Field5  string    `json:"user"`
	Field6  string    `json:"os"`
	Field7  string    `json:"version"`
	Field8  int       `json:"uptime"`
	Field9  string    `json:"ip"`
	Field10 string    `json:"pub_ip"`
}

func createMessage(currentTime time.Time, id int, name, hostName, user, osFamily, version string, up int, ip, pub string) Message {
	return Message{
		Field1:  currentTime,
		Field2:  id,
		Field3:  name,
		Field4:  hostName,
		Field5:  user,
		Field6:  osFamily,
		Field7:  version,
		Field8:  up,
		Field9:  ip,
		Field10: pub,
	}
}

func publishMessage(ctx context.Context, client *pubsub.Client, topicName string, message Message) error {
	messageData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	topic := client.Topic(topicName)
	result := topic.Publish(ctx, &pubsub.Message{
		Data: messageData,
	})

	// Get the result
	id, err := result.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to publish message: %v", err)
	}

	log.Printf("💪 Successfully submitted. Your lucky number is %s\n", id)
	return nil
}

func collectMachineInfo() (string, string, string, string, int, string, string) {
	hostInfo, _ := host.Info()
	ip, _ := getLocalIP()
	publicIP, _ := getPublicIP()
	return hostInfo.Hostname, os.Getenv("USER"), hostInfo.OS, hostInfo.PlatformVersion, int(hostInfo.Uptime), ip, publicIP
}

func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no IP address found")
}

func getPublicIP() (string, error) {
	resp, err := http.Get("https://api.ipify.org?format=text")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(ip), nil
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

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		log.Println(spacePrefix + successPrefix + "POST request successful")
	} else {
		return fmt.Errorf(errorPrefix+"POST request failed with status: %d", resp.StatusCode)
	}

	return nil
}

func sendGetRequest(url, word string) error {
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
