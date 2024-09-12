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
	appVersion           = "v1.0.2"
)

// App struct
type App struct {
	ctx         context.Context
	info        wechat.WeChatInfo
	provider    *wechat.WechatDataProvider
	defaultUser string
	users       []string
}

type WeChatInfo struct {
	ProcessID  uint32 `json:"PID"`
	FilePath   string `json:"FilePath"`
	AcountName string `json:"AcountName"`
	Version    string `json:"Version"`
	Is64Bits   bool   `json:"Is64Bits"`
	DBKey      string `json:"DBkey"`
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
	dialog, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:    runtime.QuestionDialog,
		Title:   "Quit?",
		Message: "Are you sure you want to quit?",
	})

	if err != nil || dialog == "Yes" {
		a.provider.WechatWechatDataProviderClose()
		a.provider = nil
		return false
	}

	return true
}

func (a *App) GetWeChatAllInfo() string {
	a.info, _ = wechat.GetWeChatAllInfo()

	var info WeChatInfo
	info.ProcessID = a.info.ProcessID
	info.FilePath = a.info.FilePath
	info.AcountName = a.info.AcountName
	info.Version = a.info.Version
	info.Is64Bits = a.info.Is64Bits
	info.DBKey = a.info.DBKey

	infoStr, _ := json.Marshal(info)
	log.Println(string(infoStr))

	return string(infoStr)
}

func (a *App) ExportWeChatAllData(full bool) {

	if a.provider != nil {
		a.provider.WechatWechatDataProviderClose()
		a.provider = nil
	}

	progress := make(chan string)
	go func() {

		_, err := os.Stat(".\\User")
		if err != nil {
			os.Mkdir(".\\User", os.ModeDir)
		}

		expPath := ".\\User\\" + a.info.AcountName
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

		go wechat.ExportWeChatAllData(a.info, expPath, progress)

		for p := range progress {
			log.Println(p)
			runtime.EventsEmit(a.ctx, "exportData", p)
		}

		if len(a.defaultUser) == 0 {
			a.defaultUser = a.info.AcountName
		}

		hasUser := false
		for _, user := range a.users {
			if user == a.info.AcountName {
				hasUser = true
				break
			}
		}
		if !hasUser {
			a.users = append(a.users, a.info.AcountName)
		}
		a.setCurrentConfig()
	}()
}

func (a *App) createWechatDataProvider(resPath string) error {
	if a.provider != nil && a.provider.SelfInfo != nil && filepath.Base(resPath) == a.provider.SelfInfo.UserName {
		log.Println("WechatDataProvider not need create:", a.provider.SelfInfo.UserName)
		return nil
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
