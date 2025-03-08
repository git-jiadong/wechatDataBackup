package utils

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/browser"
	"github.com/shirou/gopsutil/v3/disk"
	"golang.org/x/net/html"
	"golang.org/x/sys/windows/registry"
)

type PathStat struct {
	Path        string  `json:"path"`
	Total       uint64  `json:"total"`
	Free        uint64  `json:"free"`
	Used        uint64  `json:"used"`
	UsedPercent float64 `json:"usedPercent"`
}

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

func GetPathStat(path string) (PathStat, error) {
	pathStat := PathStat{}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return pathStat, err
	}

	stat, err := disk.Usage(absPath)
	if err != nil {
		return pathStat, err
	}

	pathStat.Path = stat.Path
	pathStat.Total = stat.Total
	pathStat.Used = stat.Used
	pathStat.Free = stat.Free
	pathStat.UsedPercent = stat.UsedPercent

	return pathStat, nil
}

func PathIsCanWriteFile(path string) bool {

	filepath := fmt.Sprintf("%s\\CanWrite.txt", path)
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false
	}

	file.Close()
	os.Remove(filepath)

	return true
}

func CopyFile(src, dst string) (int64, error) {
	stat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}
	if stat.IsDir() {
		return 0, errors.New(src + " is dir")
	}
	sourceFile, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destFile.Close()

	bytesWritten, err := io.Copy(destFile, sourceFile)
	if err != nil {
		return bytesWritten, err
	}

	return bytesWritten, nil
}

func extractTextFromHTML(htmlStr string) string {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		fmt.Println("Error parsing HTML:", err)
		return ""
	}

	var extractText func(*html.Node) string
	extractText = func(n *html.Node) string {
		if n.Type == html.TextNode {
			return n.Data
		}

		var text string
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			text += extractText(c)
		}
		return text
	}

	return extractText(doc)
}

func removeCustomTags(input string) string {

	re := regexp.MustCompile(`<(_wc_custom_link_)[^>]*?>`)
	return re.ReplaceAllString(input, `$2`)
}

func Html2Text(htmlStr string) string {
	// if htmlStr == "" {
	// 	return ""
	// }

	if len(htmlStr) == 0 || htmlStr[0] != '<' {
		return htmlStr
	}

	text := extractTextFromHTML(htmlStr)
	if strings.Contains(text, `<_wc_custom_link_`) {
		text = "\U0001F9E7" + removeCustomTags(text)
	}

	return text
}

func HtmlMsgGetAttr(htmlStr, tag string) map[string]string {

	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		fmt.Println("Error parsing HTML:", err)
		return nil
	}

	var attributes map[string]string
	var findAttributes func(*html.Node)
	findAttributes = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == tag {
			attributes = make(map[string]string)
			for _, attr := range n.Attr {
				attributes[attr.Key] = attr.Val
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findAttributes(c)
		}
	}

	findAttributes(doc)
	return attributes
}

func Hash256Sum(data []byte) string {
	hash := md5.New()
	hash.Write([]byte(data))
	hashSum := hash.Sum(nil)

	return hex.EncodeToString(hashSum)
}
