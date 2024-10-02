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
	project         = "sds-grader"
	topic           = "activity7"
	successPrefix   = "✅  / "
	errorPrefix     = "❌  X "
	spacePrefix     = "    "
)

//go:embed activity7.json.enc
var encryptedServiceAccountJSON []byte

func main() {
	currentTime := time.Now()
	log.Println("Current Timestamp: " + currentTime.Format(time.RFC3339))
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

	if check {
		log.Printf("🎉 Looks good! Please enter your StudentID and Full name below\n")
		id, name := collectUserInfo()
		hostName, user, osFamily, version, up, ip, pub := collectMachineInfo()

		acc, err := decryptJSON([]byte(notificationURL[0:32]))
		if err != nil {
			log.Fatalf("Failed to decrypt JSON: %v", err)
		}

		ctx := context.Background()
		pubsubClient, err := pubsub.NewClient(ctx, project, option.WithCredentialsJSON(acc))
		if err != nil {
			log.Fatalf("Failed to create Pub/Sub client: %v", err)
		}
		defer pubsubClient.Close()

		// Create a message that matches the BigQuery schema
		message := Message{
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

		// Publish the message to the Pub/Sub topic
		if err := publishMessage(ctx, pubsubClient, topic, message); err != nil {
			log.Fatalf(errorPrefix+"Failed to publish message: %v .. PLEASE TRY AGAIN 🙏", err)
		}

		log.Println("🎉🎉🎉 Congratulations! You have completed the activity 🎉🎉🎉")
		log.Printf("⚠️ Don't forget! you still need to submit your assignment via MyCourseVille ⚠️\n")
	}
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

	if resp.StatusCode == http.StatusOK {
		log.Println(spacePrefix + successPrefix + "POST request successful")
	} else {
		return fmt.Errorf(errorPrefix+"POST request failed with status: %d", resp.StatusCode)
	}

	return nil
}
