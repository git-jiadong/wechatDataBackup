package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"wechatDataBackup/pkg/utils"
	"wechatDataBackup/pkg/wechat"

	"github.com/spf13/viper"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	defaultConfig        = "config"
	configDefaultUserKey = "userConfig.defaultUser"
	configUsersKey       = "userConfig.users"
	configExportPathKey  = "exportPath"
	appVersion           = "v1.2.1"
)

type FileLoader struct {
	http.Handler
	FilePrefix string
}

func NewFileLoader(prefix string) *FileLoader {
	mime.AddExtensionType(".mp3", "audio/mpeg")
	return &FileLoader{FilePrefix: prefix}
}

func (h *FileLoader) SetFilePrefix(prefix string) {
	h.FilePrefix = prefix
	log.Println("SetFilePrefix", h.FilePrefix)
}

func (h *FileLoader) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	requestedFilename := h.FilePrefix + "\\" + strings.TrimPrefix(req.URL.Path, "/")

	file, err := os.Open(requestedFilename)
	if err != nil {
		http.Error(res, fmt.Sprintf("Could not load file %s", requestedFilename), http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		http.Error(res, "Could not retrieve file info", http.StatusInternalServerError)
		return
	}

	fileSize := fileInfo.Size()
	rangeHeader := req.Header.Get("Range")
	if rangeHeader == "" {
		// 无 Range 请求，直接返回整个文件
		res.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))
		http.ServeContent(res, req, requestedFilename, fileInfo.ModTime(), file)
		return
	}

	var start, end int64
	if strings.HasPrefix(rangeHeader, "bytes=") {
		ranges := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
		start, _ = strconv.ParseInt(ranges[0], 10, 64)

		if len(ranges) > 1 && ranges[1] != "" {
			end, _ = strconv.ParseInt(ranges[1], 10, 64)
		} else {
			end = fileSize - 1
		}
	} else {
		http.Error(res, "Invalid Range header", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	if start < 0 || end >= fileSize || start > end {
		http.Error(res, "Requested range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	contentType := mime.TypeByExtension(filepath.Ext(requestedFilename))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	res.Header().Set("Content-Type", contentType)
	res.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	res.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	res.WriteHeader(http.StatusPartialContent)
	buffer := make([]byte, 102400)
	file.Seek(start, 0)
	for current := start; current <= end; {
		readSize := int64(len(buffer))
		if end-current+1 < readSize {
			readSize = end - current + 1
		}

		n, err := file.Read(buffer[:readSize])
		if err != nil {
			break
		}

		res.Write(buffer[:n])
		current += int64(n)
	}
}

// App struct
type App struct {
	ctx         context.Context
	infoList    *wechat.WeChatInfoList
	provider    *wechat.WechatDataProvider
	defaultUser string
	users       []string
	firstStart  bool
	firstInit   bool
	FLoader     *FileLoader
}

type WeChatInfo struct {
	ProcessID  uint32 `json:"PID"`
	FilePath   string `json:"FilePath"`
	AcountName string `json:"AcountName"`
	Version    string `json:"Version"`
	Is64Bits   bool   `json:"Is64Bits"`
	DBKey      string `json:"DBkey"`
}

type WeChatInfoList struct {
	Info  []WeChatInfo `json:"Info"`
	Total int          `json:"Total"`
}

type WeChatAccountInfos struct {
	CurrentAccount string                     `json:"CurrentAccount"`
	Info           []wechat.WeChatAccountInfo `json:"Info"`
	Total          int                        `json:"Total"`
}

type ErrorMessage struct {
	ErrorStr string `json:"error"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	a := &App{}
	log.Println("App version:", appVersion)
	a.firstInit = true
	a.FLoader = NewFileLoader(".\\")
	viper.SetConfigName(defaultConfig)
	viper.SetConfigType("json")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err == nil {
		a.defaultUser = viper.GetString(configDefaultUserKey)
		a.users = viper.GetStringSlice(configUsersKey)
		prefix := viper.GetString(configExportPathKey)
		if prefix != "" {
			log.Println("SetFilePrefix", prefix)
			a.FLoader.SetFilePrefix(prefix)
		}
	} else {
		log.Println("not config exist")
	}
	log.Printf("default: %s users: %v\n", a.defaultUser, a.users)
	if len(a.users) == 0 {
		a.firstStart = true
	}

	return a
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	return false
}

func (a *App) shutdown(ctx context.Context) {
	if a.provider != nil {
		a.provider.WechatWechatDataProviderClose()
		a.provider = nil
	}
	log.Printf("App Version %s exit!", appVersion)
}

func (a *App) GetWeChatAllInfo() string {
	infoList := WeChatInfoList{}
	infoList.Info = make([]WeChatInfo, 0)
	infoList.Total = 0

	if a.provider != nil {
		a.provider.WechatWechatDataProviderClose()
		a.provider = nil
	}

	a.infoList = wechat.GetWeChatAllInfo()
	for i := range a.infoList.Info {
		var info WeChatInfo
		info.ProcessID = a.infoList.Info[i].ProcessID
		info.FilePath = a.infoList.Info[i].FilePath
		info.AcountName = a.infoList.Info[i].AcountName
		info.Version = a.infoList.Info[i].Version
		info.Is64Bits = a.infoList.Info[i].Is64Bits
		info.DBKey = a.infoList.Info[i].DBKey
		infoList.Info = append(infoList.Info, info)
		infoList.Total += 1
		log.Printf("ProcessID %d, FilePath %s, AcountName %s, Version %s, Is64Bits %t", info.ProcessID, info.FilePath, info.AcountName, info.Version, info.Is64Bits)
	}
	infoStr, _ := json.Marshal(infoList)
	// log.Println(string(infoStr))

	return string(infoStr)
}

func (a *App) ExportWeChatAllData(full bool, acountName string) {

	if a.provider != nil {
		a.provider.WechatWechatDataProviderClose()
		a.provider = nil
	}

	progress := make(chan string)
	go func() {
		var pInfo *wechat.WeChatInfo
		for i := range a.infoList.Info {
			if a.infoList.Info[i].AcountName == acountName {
				pInfo = &a.infoList.Info[i]
				break
			}
		}

		if pInfo == nil {
			close(progress)
			runtime.EventsEmit(a.ctx, "exportData", fmt.Sprintf("{\"status\":\"error\", \"result\":\"%s error\"}", acountName))
			return
		}

		prefixExportPath := a.FLoader.FilePrefix + "\\User\\"
		_, err := os.Stat(prefixExportPath)
		if err != nil {
			os.Mkdir(prefixExportPath, os.ModeDir)
		}

		expPath := prefixExportPath + pInfo.AcountName
		_, err = os.Stat(expPath)
		if err == nil {
			if !full {
				os.RemoveAll(expPath + "\\Msg")
			} else {
				os.RemoveAll(expPath)
			}
		}

		_, err = os.Stat(expPath)
		if err != nil {
			os.Mkdir(expPath, os.ModeDir)
		}

		go wechat.ExportWeChatAllData(*pInfo, expPath, progress)

		for p := range progress {
			log.Println(p)
			runtime.EventsEmit(a.ctx, "exportData", p)
		}

		a.defaultUser = pInfo.AcountName
		hasUser := false
		for _, user := range a.users {
			if user == pInfo.AcountName {
				hasUser = true
				break
			}
		}
		if !hasUser {
			a.users = append(a.users, pInfo.AcountName)
		}
		a.setCurrentConfig()
	}()
}

func (a *App) createWechatDataProvider(resPath string, prefix string) error {
	if a.provider != nil && a.provider.SelfInfo != nil && filepath.Base(resPath) == a.provider.SelfInfo.UserName {
		log.Println("WechatDataProvider not need create:", a.provider.SelfInfo.UserName)
		return nil
	}

	if a.provider != nil {
		a.provider.WechatWechatDataProviderClose()
		a.provider = nil
		log.Println("createWechatDataProvider WechatWechatDataProviderClose")
	}

	provider, err := wechat.CreateWechatDataProvider(resPath, prefix)
	if err != nil {
		log.Println("CreateWechatDataProvider failed:", resPath)
		return err
	}

	a.provider = provider
	// infoJson, _ := json.Marshal(a.provider.SelfInfo)
	// runtime.EventsEmit(a.ctx, "selfInfo", string(infoJson))
	return nil
}

func (a *App) WeChatInit() {

	if a.firstInit {
		a.firstInit = false
		a.scanAccountByPath(a.FLoader.FilePrefix)
		log.Println("scanAccountByPath:", a.FLoader.FilePrefix)
	}

	if len(a.defaultUser) == 0 {
		log.Println("not defaultUser")
		return
	}

	expPath := a.FLoader.FilePrefix + "\\User\\" + a.defaultUser
	prefixPath := "\\User\\" + a.defaultUser
	wechat.ExportWeChatHeadImage(expPath)
	if a.createWechatDataProvider(expPath, prefixPath) == nil {
		infoJson, _ := json.Marshal(a.provider.SelfInfo)
		runtime.EventsEmit(a.ctx, "selfInfo", string(infoJson))
	}
}

func (a *App) GetWechatSessionList(pageIndex int, pageSize int) string {
	if a.provider == nil {
		log.Println("provider not init")
		return "{\"Total\":0}"
	}
	log.Printf("pageIndex: %d\n", pageIndex)
	list, err := a.provider.WeChatGetSessionList(pageIndex, pageSize)
	if err != nil {
		return "{\"Total\":0}"
	}

	listStr, _ := json.Marshal(list)
	log.Println("GetWechatSessionList:", list.Total)
	return string(listStr)
}

func (a *App) GetWechatContactList(pageIndex int, pageSize int) string {
	if a.provider == nil {
		log.Println("provider not init")
		return "{\"Total\":0}"
	}
	log.Printf("pageIndex: %d\n", pageIndex)
	list, err := a.provider.WeChatGetContactList(pageIndex, pageSize)
	if err != nil {
		return "{\"Total\":0}"
	}

	listStr, _ := json.Marshal(list)
	log.Println("WeChatGetContactList:", list.Total)
	return string(listStr)
}

func (a *App) GetWechatMessageListByTime(userName string, time int64, pageSize int, direction string) string {
	log.Println("GetWechatMessageListByTime:", userName, pageSize, time, direction)
	if len(userName) == 0 {
		return "{\"Total\":0, \"Rows\":[]}"
	}
	dire := wechat.Message_Search_Forward
	if direction == "backward" {
		dire = wechat.Message_Search_Backward
	} else if direction == "both" {
		dire = wechat.Message_Search_Both
	}
	list, err := a.provider.WeChatGetMessageListByTime(userName, time, pageSize, dire)
	if err != nil {
		log.Println("GetWechatMessageListByTime failed:", err)
		return ""
	}
	listStr, _ := json.Marshal(list)
	log.Println("GetWechatMessageListByTime:", list.Total)

	return string(listStr)
}

func (a *App) GetWechatMessageListByType(userName string, time int64, pageSize int, msgType string, direction string) string {
	log.Println("GetWechatMessageListByType:", userName, pageSize, time, msgType, direction)
	if len(userName) == 0 {
		return "{\"Total\":0, \"Rows\":[]}"
	}
	dire := wechat.Message_Search_Forward
	if direction == "backward" {
		dire = wechat.Message_Search_Backward
	} else if direction == "both" {
		dire = wechat.Message_Search_Both
	}
	list, err := a.provider.WeChatGetMessageListByType(userName, time, pageSize, msgType, dire)
	if err != nil {
		log.Println("WeChatGetMessageListByType failed:", err)
		return ""
	}
	listStr, _ := json.Marshal(list)
	log.Println("WeChatGetMessageListByType:", list.Total)

	return string(listStr)
}

func (a *App) GetWechatMessageListByKeyWord(userName string, time int64, keyword string, msgType string, pageSize int) string {
	log.Println("GetWechatMessageListByKeyWord:", userName, pageSize, time, msgType)
	if len(userName) == 0 {
		return "{\"Total\":0, \"Rows\":[]}"
	}
	list, err := a.provider.WeChatGetMessageListByKeyWord(userName, time, keyword, msgType, pageSize)
	if err != nil {
		log.Println("WeChatGetMessageListByKeyWord failed:", err)
		return ""
	}
	listStr, _ := json.Marshal(list)
	log.Println("WeChatGetMessageListByKeyWord:", list.Total, list.KeyWord)

	return string(listStr)
}

func (a *App) GetWechatMessageDate(userName string) string {
	log.Println("GetWechatMessageDate:", userName)
	if len(userName) == 0 {
		return "{\"Total\":0, \"Date\":[]}"
	}

	messageData, err := a.provider.WeChatGetMessageDate(userName)
	if err != nil {
		log.Println("GetWechatMessageDate:", err)
		return ""
	}

	messageDataStr, _ := json.Marshal(messageData)
	log.Println("GetWechatMessageDate:", messageData.Total)

	return string(messageDataStr)
}

func (a *App) setCurrentConfig() {
	viper.Set(configDefaultUserKey, a.defaultUser)
	viper.Set(configUsersKey, a.users)
	viper.Set(configExportPathKey, a.FLoader.FilePrefix)
	err := viper.SafeWriteConfig()
	if err != nil {
		log.Println(err)
		err = viper.WriteConfig()
		if err != nil {
			log.Println(err)
		}
	}
}

type userList struct {
	Users []string `json:"Users"`
}

func (a *App) GetWeChatUserList() string {

	l := userList{}
	l.Users = a.users

	usersStr, _ := json.Marshal(l)
	str := string(usersStr)
	log.Println("users:", str)
	return str
}

func (a *App) OpenFileOrExplorer(filePath string, explorer bool) string {
	// if root, err := os.Getwd(); err == nil {
	// 	filePath = root + filePath[1:]
	// }
	// log.Println("OpenFileOrExplorer:", filePath)

	path := a.FLoader.FilePrefix + filePath
	err := utils.OpenFileOrExplorer(path, explorer)
	if err != nil {
		return "{\"result\": \"OpenFileOrExplorer failed\", \"status\":\"failed\"}"
	}

	return fmt.Sprintf("{\"result\": \"%s\", \"status\":\"OK\"}", "")
}

func (a *App) GetWeChatRoomUserList(roomId string) string {
	userlist, err := a.provider.WeChatGetChatRoomUserList(roomId)
	if err != nil {
		log.Println("WeChatGetChatRoomUserList:", err)
		return ""
	}

	userListStr, _ := json.Marshal(userlist)

	return string(userListStr)
}

func (a *App) GetAppVersion() string {
	return appVersion
}

func (a *App) GetAppIsFirstStart() bool {
	defer func() { a.firstStart = false }()
	return a.firstStart
}

func (a *App) GetWechatLocalAccountInfo() string {
	infos := WeChatAccountInfos{}
	infos.Info = make([]wechat.WeChatAccountInfo, 0)
	infos.Total = 0
	infos.CurrentAccount = a.defaultUser
	for i := range a.users {
		resPath := a.FLoader.FilePrefix + "\\User\\" + a.users[i]
		if _, err := os.Stat(resPath); err != nil {
			log.Println("GetWechatLocalAccountInfo:", resPath, err)
			continue
		}

		prefixResPath := "\\User\\" + a.users[i]
		info, err := wechat.WechatGetAccountInfo(resPath, prefixResPath, a.users[i])
		if err != nil {
			log.Println("GetWechatLocalAccountInfo", err)
			continue
		}

		infos.Info = append(infos.Info, *info)
		infos.Total += 1
	}

	infoString, _ := json.Marshal(infos)
	log.Println(string(infoString))

	return string(infoString)
}

func (a *App) WechatSwitchAccount(account string) bool {
	for i := range a.users {
		if a.users[i] == account {
			if a.provider != nil {
				a.provider.WechatWechatDataProviderClose()
				a.provider = nil
			}
			a.defaultUser = account
			a.setCurrentConfig()
			return true
		}
	}

	return false
}

func (a *App) GetExportPathStat() string {
	path := a.FLoader.FilePrefix
	log.Println("utils.GetPathStat ++")
	stat, err := utils.GetPathStat(path)
	log.Println("utils.GetPathStat --")
	if err != nil {
		log.Println("GetPathStat error:", path, err)
		var msg ErrorMessage
		msg.ErrorStr = fmt.Sprintf("%s:%v", path, err)
		msgStr, _ := json.Marshal(msg)
		return string(msgStr)
	}

	statString, _ := json.Marshal(stat)

	return string(statString)
}

func (a *App) ExportPathIsCanWrite() bool {
	path := a.FLoader.FilePrefix
	return utils.PathIsCanWriteFile(path)
}

func (a *App) OpenExportPath() {
	path := a.FLoader.FilePrefix
	runtime.BrowserOpenURL(a.ctx, path)
}

func (a *App) OpenDirectoryDialog() string {
	dialogOptions := runtime.OpenDialogOptions{
		Title: "选择导出路径",
	}
	selectedDir, err := runtime.OpenDirectoryDialog(a.ctx, dialogOptions)
	if err != nil {
		log.Println("OpenDirectoryDialog:", err)
		return ""
	}

	if selectedDir == "" {
		log.Println("Cancel selectedDir")
		return ""
	}

	if selectedDir == a.FLoader.FilePrefix {
		log.Println("same path No need SetFilePrefix")
		return ""
	}

	if !utils.PathIsCanWriteFile(selectedDir) {
		log.Println("PathIsCanWriteFile:", selectedDir, "error")
		return ""
	}

	a.FLoader.SetFilePrefix(selectedDir)
	log.Println("OpenDirectoryDialog:", selectedDir)
	a.scanAccountByPath(selectedDir)
	return selectedDir
}

func (a *App) scanAccountByPath(path string) error {
	infos := WeChatAccountInfos{}
	infos.Info = make([]wechat.WeChatAccountInfo, 0)
	infos.Total = 0
	infos.CurrentAccount = ""

	userPath := path + "\\User\\"
	if _, err := os.Stat(userPath); err != nil {
		return err
	}

	dirs, err := os.ReadDir(userPath)
	if err != nil {
		log.Println("ReadDir", err)
		return err
	}

	for i := range dirs {
		if !dirs[i].Type().IsDir() {
			continue
		}
		log.Println("dirs[i].Name():", dirs[i].Name())
		resPath := path + "\\User\\" + dirs[i].Name()
		prefixResPath := "\\User\\" + dirs[i].Name()
		info, err := wechat.WechatGetAccountInfo(resPath, prefixResPath, dirs[i].Name())
		if err != nil {
			log.Println("GetWechatLocalAccountInfo", err)
			continue
		}

		infos.Info = append(infos.Info, *info)
		infos.Total += 1
	}

	users := make([]string, 0)
	for i := 0; i < infos.Total; i++ {
		users = append(users, infos.Info[i].AccountName)
	}

	a.users = users
	found := false
	for i := range a.users {
		if a.defaultUser == a.users[i] {
			found = true
		}
	}

	if !found {
		a.defaultUser = ""
	}
	if a.defaultUser == "" && len(a.users) > 0 {
		a.defaultUser = a.users[0]
	}

	if len(a.users) > 0 {
		a.setCurrentConfig()
	}

	return nil
}

func (a *App) OepnLogFileExplorer() {
	utils.OpenFileOrExplorer(".\\app.log", true)
}

func (a *App) SaveFileDialog(file string, alisa string) string {
	filePath := a.FLoader.FilePrefix + file
	if _, err := os.Stat(filePath); err != nil {
		log.Println("SaveFileDialog:", err)
		return err.Error()
	}

	savePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: alisa,
		Title:           "选择保存路径",
	})
	if err != nil {
		log.Println("SaveFileDialog:", err)
		return err.Error()
	}

	if savePath == "" {
		return ""
	}

	dirPath := filepath.Dir(savePath)
	if !utils.PathIsCanWriteFile(dirPath) {
		errStr := "Path Is Can't Write File: " + filepath.Dir(savePath)
		log.Println(errStr)
		return errStr
	}

	_, err = utils.CopyFile(filePath, savePath)
	if err != nil {
		log.Println("Error CopyFile", filePath, savePath, err)
		return err.Error()
	}

	return ""
}

func (a *App) GetSessionLastTime(userName string) string {
	if a.provider == nil || userName == "" {
		lastTime := &wechat.WeChatLastTime{}
		lastTimeString, _ := json.Marshal(lastTime)
		return string(lastTimeString)
	}

	lastTime := a.provider.WeChatGetSessionLastTime(userName)

	lastTimeString, _ := json.Marshal(lastTime)

	return string(lastTimeString)
}

func (a *App) SetSessionLastTime(userName string, stamp int64, messageId string) string {
	if a.provider == nil {
		return ""
	}

	lastTime := &wechat.WeChatLastTime{
		UserName:  userName,
		Timestamp: stamp,
		MessageId: messageId,
	}
	err := a.provider.WeChatSetSessionLastTime(lastTime)
	if err != nil {
		log.Println("WeChatSetSessionLastTime failed:", err.Error())
		return err.Error()
	}

	return ""
}

func (a *App) SetSessionBookMask(userName, tag, info string) string {
	if a.provider == nil || userName == "" {
		return "invaild params"
	}
	err := a.provider.WeChatSetSessionBookMask(userName, tag, info)
	if err != nil {
		log.Println("WeChatSetSessionBookMask failed:", err.Error())
		return err.Error()
	}

	return ""
}

func (a *App) DelSessionBookMask(markId string) string {
	if a.provider == nil || markId == "" {
		return "invaild params"
	}

	err := a.provider.WeChatDelSessionBookMask(markId)
	if err != nil {
		log.Println("WeChatDelSessionBookMask failed:", err.Error())
		return err.Error()
	}

	return ""
}

func (a *App) GetSessionBookMaskList(userName string) string {
	if a.provider == nil || userName == "" {
		return "invaild params"
	}
	markLIst, err := a.provider.WeChatGetSessionBookMaskList(userName)
	if err != nil {
		log.Println("WeChatGetSessionBookMaskList failed:", err.Error())
		_list := &wechat.WeChatBookMarkList{}
		_listString, _ := json.Marshal(_list)
		return string(_listString)
	}

	markLIstString, _ := json.Marshal(markLIst)
	return string(markLIstString)
}

func (a *App) SelectedDirDialog(title string) string {
	dialogOptions := runtime.OpenDialogOptions{
		Title: title,
	}
	selectedDir, err := runtime.OpenDirectoryDialog(a.ctx, dialogOptions)
	if err != nil {
		log.Println("OpenDirectoryDialog:", err)
		return ""
	}

	if selectedDir == "" {
		return ""
	}

	return selectedDir
}

func (a *App) ExportWeChatDataByUserName(userName, path string) string {
	if a.provider == nil || userName == "" || path == "" {
		return "invaild params" + userName
	}

	if !utils.PathIsCanWriteFile(path) {
		log.Println("PathIsCanWriteFile: " + path)
		return "PathIsCanWriteFile: " + path
	}

	exPath := path + "\\" + "wechatDataBackup_" + userName
	if _, err := os.Stat(exPath); err != nil {
		os.MkdirAll(exPath, os.ModePerm)
	} else {
		return "path exist:" + exPath
	}

	log.Println("ExportWeChatDataByUserName:", userName, exPath)
	err := a.provider.WeChatExportDataByUserName(userName, exPath)
	if err != nil {
		log.Println("WeChatExportDataByUserName failed:", err)
		return "WeChatExportDataByUserName failed:" + err.Error()
	}

	exeSrcPath := a.FLoader.FilePrefix + "\\" + "wechatDataBackup.exe"
	exeDstPath := exPath + "\\" + "wechatDataBackup.exe"
	_, err = utils.CopyFile(exeSrcPath, exeDstPath)
	if err != nil {
		log.Println("CopyFile:", err)
		return "CopyFile:" + err.Error()
	}

	config := map[string]interface{}{
		"exportpath": ".\\",
		"userconfig": map[string]interface{}{
			"defaultuser": a.defaultUser,
			"users":       []string{a.defaultUser},
		},
	}

	configJson, err := json.MarshalIndent(config, "", "	")
	if err != nil {
		log.Println("MarshalIndent:", err)
		return "MarshalIndent:" + err.Error()
	}

	configPath := exPath + "\\" + "config.json"
	err = os.WriteFile(configPath, configJson, os.ModePerm)
	if err != nil {
		log.Println("WriteFile:", err)
		return "WriteFile:" + err.Error()
	}

	return ""
}

func (a *App) GetAppIsShareData() bool {
	if a.provider != nil {
		return a.provider.IsShareData
	}
	return false
}
