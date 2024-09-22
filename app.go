package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"wechatDataBackup/pkg/utils"
	"wechatDataBackup/pkg/wechat"

	"github.com/spf13/viper"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	defaultConfig        = "config"
	configDefaultUserKey = "userConfig.defaultUser"
	configUsersKey       = "userConfig.users"
	appVersion           = "v1.0.3"
)

// App struct
type App struct {
	ctx         context.Context
	infoList    *wechat.WeChatInfoList
	provider    *wechat.WechatDataProvider
	defaultUser string
	users       []string
	firstStart  bool
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

// NewApp creates a new App application struct
func NewApp() *App {
	a := &App{}

	viper.SetConfigName(defaultConfig)
	viper.SetConfigType("json")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err == nil {
		a.defaultUser = viper.GetString(configDefaultUserKey)
		a.users = viper.GetStringSlice(configUsersKey)
		// log.Println(a.defaultUser)
		// log.Println(a.users)
	} else {
		a.firstStart = true
		log.Println("not config exist")
	}

	return a
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) beforeClose(ctx context.Context) (prevent bool) {

	if a.provider != nil {
		a.provider.WechatWechatDataProviderClose()
		a.provider = nil
	}

	return false

}

func (a *App) GetWeChatAllInfo() string {
	infoList := WeChatInfoList{}
	infoList.Info = make([]WeChatInfo, 0)
	infoList.Total = 0

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

		_, err := os.Stat(".\\User")
		if err != nil {
			os.Mkdir(".\\User", os.ModeDir)
		}

		expPath := ".\\User\\" + pInfo.AcountName
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

func (a *App) createWechatDataProvider(resPath string) error {
	if a.provider != nil && a.provider.SelfInfo != nil && filepath.Base(resPath) == a.provider.SelfInfo.UserName {
		log.Println("WechatDataProvider not need create:", a.provider.SelfInfo.UserName)
		return nil
	}

	if a.provider != nil {
		a.provider.WechatWechatDataProviderClose()
		a.provider = nil
		log.Println("createWechatDataProvider WechatWechatDataProviderClose")
	}

	provider, err := wechat.CreateWechatDataProvider(resPath)
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
	expPath := ".\\User\\" + a.defaultUser
	if a.createWechatDataProvider(expPath) == nil {
		infoJson, _ := json.Marshal(a.provider.SelfInfo)
		runtime.EventsEmit(a.ctx, "selfInfo", string(infoJson))
	}
}

func (a *App) GetWechatSessionList(pageIndex int, pageSize int) string {
	expPath := ".\\User\\" + a.defaultUser
	if a.createWechatDataProvider(expPath) != nil {
		return ""
	}
	log.Printf("pageIndex: %d\n", pageIndex)
	list, err := a.provider.WeChatGetSessionList(pageIndex, pageSize)
	if err != nil {
		return ""
	}

	listStr, _ := json.Marshal(list)
	log.Println("GetWechatSessionList:", list.Total)
	return string(listStr)
}

func (a *App) GetWechatMessageListByTime(userName string, time int64, pageSize int, direction string) string {
	log.Println("GetWechatMessageList:", userName, pageSize, time, direction)
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
		log.Println("WeChatGetMessageList failed:", err)
		return ""
	}
	listStr, _ := json.Marshal(list)
	log.Println("GetWechatMessageList:", list.Total)

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
	err := utils.OpenFileOrExplorer(filePath, explorer)
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
		resPath := ".\\User\\" + a.users[i]
		if _, err := os.Stat(resPath); err != nil {
			log.Println("GetWechatLocalAccountInfo:", resPath, err)
			continue
		}

		info, err := wechat.WechatGetAccountInfo(resPath, a.users[i])
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
