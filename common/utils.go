package common

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
)

func HandleError(err error, message string) {
	if err != nil {
		log.Fatalf("%s: %v", message, err)
	}
}

func CheckResult(err error, passMessage string) bool {
	if err != nil {
		log.Printf("%s%v\n", ErrorPrefix, err)
		return false
	}
	if passMessage != "" {
		log.Printf("%s%s\n", SuccessPrefix, passMessage)
	}
	return true
}

func AllTrue(arr []bool) bool {
	for _, v := range arr {
		if !v {
			return false
		}
	}
	return true
}

func CollectInfo(info string, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("👉 Enter %s (leave blank for '%s'): ", info, defaultValue)
	value, _ := reader.ReadString('\n')
	value = strings.TrimSpace(value)
	if value == "" {
		value = defaultValue
	}

	return value
}

func EnsureHTTPPrefix(domain string) string {
	if strings.HasPrefix(domain, "http://") || strings.HasPrefix(domain, "https://") {
		return domain
	}
	return "http://" + domain
}
