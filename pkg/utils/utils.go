package utils

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/browser"
	"golang.org/x/sys/windows/registry"
)

func getDefaultProgram(fileExtension string) (string, error) {
	key, err := registry.OpenKey(registry.CLASSES_ROOT, fmt.Sprintf(`.%s`, fileExtension), registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()

	// 读取默认程序关联值
	defaultProgram, _, err := key.GetStringValue("")
	if err != nil {
		return "", err
	}

	return defaultProgram, nil
}

func hasDefaultProgram(fileExtension string) bool {
	prog, err := getDefaultProgram(fileExtension)
	if err != nil {
		log.Println("getDefaultProgram Error:", err)
		return false
	}

	if prog == "" {
		return false
	}

	return true
}

func OpenFileOrExplorer(filePath string, explorer bool) error {
	if _, err := os.Stat(filePath); err != nil {
		log.Printf("%s %v\n", filePath, err)
		return err
	}

	canOpen := false
	fileExtension := ""
	index := strings.LastIndex(filePath, ".")
	if index > 0 {
		fileExtension = filePath[index+1:]
		canOpen = hasDefaultProgram(fileExtension)
	}

	if canOpen && !explorer {
		return browser.OpenFile(filePath)
	}

	commandArgs := []string{"/select,", filePath}
	fmt.Println("cmd:", "explorer", commandArgs)

	// 创建一个Cmd结构体表示要执行的命令
	cmd := exec.Command("explorer", commandArgs...)

	// 执行命令并等待它完成
	err := cmd.Run()
	if err != nil {
		log.Printf("Error executing command: %s\n", err)
		// return err
	}

	fmt.Println("Command executed successfully")
	return nil
}
