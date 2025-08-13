package common

import (
	"bufio"
	"cloud.google.com/go/pubsub"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"google.golang.org/api/option"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/host"
)

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

func HandleSuccess(currentTime time.Time, encryptedServiceAccountJSON []byte, key []byte, project string, topic string) {
	log.Printf("🎉 Looks good! Please enter your StudentID and Full name below\n")
	id, name := CollectUserInfo()
	hostName, user, osFamily, version, up, ip, pub := CollectMachineInfo()

	acc, err := DecryptJSON(key, encryptedServiceAccountJSON)
	HandleError(err, "Failed to decrypt JSON")

	ctx := context.Background()
	pubsubClient, err := pubsub.NewClient(ctx, project, option.WithCredentialsJSON(acc))
	HandleError(err, "Failed to create Pub/Sub client")
	defer pubsubClient.Close()
	message := CreateMessage(currentTime, id, name, hostName, user, osFamily, version, up, ip, pub)

	pub_status := PublishMessage(ctx, pubsubClient, topic, message)
	HandleError(pub_status, "Failed to publish message")

	log.Println("🎉🎉🎉 Congratulations! You have completed the activity 🎉🎉🎉")
	log.Printf("⚠️ Don't forget! you still need to submit your assignment via MyCourseVille ⚠️\n")
}

func CollectUserInfo() (int, string) {
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
		log.Println(ErrorPrefix + "Invalid StudentID. Please enter a valid integer.")
	}

	fmt.Print("👉 Full Name (TH): ")
	fullName, _ := reader.ReadString('\n')
	fullName = strings.TrimSpace(fullName)
	return studentID, fullName
}

func CollectMachineInfo() (string, string, string, string, int, string, string) {
	hostInfo, _ := host.Info()
	ip, _ := GetLocalIP()
	publicIP, _ := GetPublicIP()
	return hostInfo.Hostname, os.Getenv("USER"), hostInfo.OS, hostInfo.PlatformVersion, int(hostInfo.Uptime), ip, publicIP
}

func CreateMessage(currentTime time.Time, id int, name, hostName, user, osFamily, version string, up int, ip, pub string) Message {
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

func PublishMessage(ctx context.Context, client *pubsub.Client, topicName string, message Message) error {
	messageData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	topic := client.Topic(topicName)
	result := topic.Publish(ctx, &pubsub.Message{
		Data: messageData,
	})

	id, err := result.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to publish message: %v", err)
	}

	log.Printf("💪 Successfully submitted. Your lucky number is %s\n", id)
	return nil
}

func DecryptJSON(key []byte, encryptedServiceAccountJSON []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid encryption key length: %d. Expected 32 bytes for AES-256", len(key))
	}

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

func GetLocalIP() (string, error) {
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

func GetPublicIP() (string, error) {
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
