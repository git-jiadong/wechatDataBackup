package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"wechatDataBackup/pkg/wechat"
)

func main() {
	var (
		dataPath   = flag.String("data", "", "微信数据路径 (例如: C:\\export\\User\\wxid_xxx)")
		userName   = flag.String("user", "", "要导出的用户名")
		outputPath = flag.String("output", ".", "输出路径")
		listUsers  = flag.Bool("list", false, "列出所有可用的用户")
	)
	flag.Parse()

	fmt.Println("=== 微信聊天记录txt导出工具 ===")

	if *dataPath == "" {
		fmt.Println("错误: 请指定微信数据路径")
		fmt.Println("用法: txt_export_tool.exe -data=\"C:\\export\\User\\wxid_xxx\" -user=\"用户名\" -output=\"输出路径\"")
		fmt.Println("列出用户: txt_export_tool.exe -data=\"C:\\export\\User\\wxid_xxx\" -list")
		return
	}

	if _, err := os.Stat(*dataPath); err != nil {
		fmt.Printf("错误: 数据路径不存在: %s\n", *dataPath)
		return
	}

	prefixPath := "\\User\\" + filepath.Base(*dataPath)
	provider, err := wechat.CreateWechatDataProvider(*dataPath, prefixPath)
	if err != nil {
		fmt.Printf("错误: 无法创建数据提供者: %v\n", err)
		return
	}
	defer provider.WechatWechatDataProviderClose()

	if *listUsers {
		listAllUsers(provider)
		return
	}

	if *userName == "" {
		fmt.Println("错误: 请指定要导出的用户名")
		fmt.Println("使用 -list 参数查看所有可用用户")
		return
	}

	result := exportToTxt(provider, *userName, *outputPath)
	fmt.Println(result)
}

func listAllUsers(provider *wechat.WechatDataProvider) {
	fmt.Println("\n=== 可用的聊天对象 ===")
	
	sessionList, err := provider.WeChatGetSessionList(0, 100)
	if err != nil {
		fmt.Printf("获取会话列表失败: %v\n", err)
		return
	}

	fmt.Printf("找到 %d 个聊天对象:\n\n", sessionList.Total)
	for i, session := range sessionList.Rows {
		chatType := "单聊"
		if session.IsGroup {
			chatType = "群聊"
		}
		
		displayName := session.UserInfo.NickName
		if session.UserInfo.ReMark != "" {
			displayName = session.UserInfo.ReMark
		}
		
		fmt.Printf("%d. 用户名: %s\n", i+1, session.UserName)
		fmt.Printf("   显示名: %s\n", displayName)
		fmt.Printf("   类型: %s\n", chatType)
		fmt.Printf("   最后消息: %s\n", session.Content)
		fmt.Println("   " + strings.Repeat("-", 50))
	}
	
	fmt.Println("\n使用示例:")
	fmt.Printf("txt_export_tool.exe -data=\"%s\" -user=\"用户名\" -output=\"C:\\导出路径\"\n", 
		strings.ReplaceAll(provider.SelfInfo.UserName, "\\", "\\\\"))
}

func exportToTxt(provider *wechat.WechatDataProvider, userName, outputPath string) string {
	userInfo, err := provider.WechatGetUserInfoByNameOnCache(userName)
	if err != nil {
		return fmt.Sprintf("错误: 找不到用户 %s: %v", userName, err)
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("wechat_chat_%s_%s.txt", userName, timestamp)
	fullPath := filepath.Join(outputPath, filename)
	
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Sprintf("错误: 创建文件失败: %v", err)
	}
	defer file.Close()

	header := fmt.Sprintf("微信聊天记录导出\n")
	header += fmt.Sprintf("导出时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	header += fmt.Sprintf("聊天对象: %s", userInfo.NickName)
	if userInfo.ReMark != "" {
		header += fmt.Sprintf(" (%s)", userInfo.ReMark)
	}
	header += "\n"
	if userInfo.IsGroup {
		header += "聊天类型: 群聊\n"
	} else {
		header += "聊天类型: 单聊\n"
	}
	header += strings.Repeat("=", 50) + "\n\n"
	
	file.WriteString(header)

	pageSize := 500
	currentTime := time.Now().Unix()
	totalMessages := 0

	fmt.Printf("正在导出 %s 的聊天记录...\n", userInfo.NickName)

	for {
		messageList, err := provider.WeChatGetMessageListByTime(userName, currentTime, pageSize, wechat.Message_Search_Forward)
		if err != nil {
			log.Printf("获取消息失败: %v\n", err)
			break
		}

		if messageList.Total == 0 {
			break
		}

		for _, msg := range messageList.Rows {
			msgText := formatMessageToText(&msg, provider.SelfInfo.UserName)
			file.WriteString(msgText + "\n")
			totalMessages++
		}

		fmt.Printf("已处理 %d 条消息...\r", totalMessages)

		currentTime = messageList.Rows[messageList.Total-1].CreateTime - 1
	}

	footer := fmt.Sprintf("\n%s\n", strings.Repeat("=", 50))
	footer += fmt.Sprintf("导出完成，共导出 %d 条消息\n", totalMessages)
	file.WriteString(footer)

	return fmt.Sprintf("\n✅ 导出成功！\n文件保存在: %s\n共导出 %d 条消息", fullPath, totalMessages)
}

func formatMessageToText(msg *wechat.WeChatMessage, selfUserName string) string {
	msgTime := time.Unix(msg.CreateTime, 0).Format("2006-01-02 15:04:05")
	
	var sender string
	if msg.IsSender == 1 {
		sender = "我"
	} else {
		if msg.UserInfo.ReMark != "" {
			sender = msg.UserInfo.ReMark
		} else if msg.UserInfo.NickName != "" {
			sender = msg.UserInfo.NickName
		} else {
			sender = msg.UserInfo.UserName
		}
	}

	var content string
	switch msg.Type {
	case wechat.Wechat_Message_Type_Text:
		content = msg.Content
	case wechat.Wechat_Message_Type_Picture:
		content = "[图片]"
		if msg.ImagePath != "" {
			content += fmt.Sprintf(" (路径: %s)", msg.ImagePath)
		}
	case wechat.Wechat_Message_Type_Voice:
		content = "[语音]"
		if msg.VoicePath != "" {
			content += fmt.Sprintf(" (路径: %s)", msg.VoicePath)
		}
	case wechat.Wechat_Message_Type_Video:
		content = "[视频]"
		if msg.VideoPath != "" {
			content += fmt.Sprintf(" (路径: %s)", msg.VideoPath)
		}
	case wechat.Wechat_Message_Type_Emoji:
		content = "[表情]"
		if msg.EmojiPath != "" {
			content += fmt.Sprintf(" (路径: %s)", msg.EmojiPath)
		}
	case wechat.Wechat_Message_Type_Location:
		content = fmt.Sprintf("[位置] %s", msg.LocationInfo.Label)
		if msg.LocationInfo.PoiName != "" {
			content += fmt.Sprintf(" (%s)", msg.LocationInfo.PoiName)
		}
	case wechat.Wechat_Message_Type_Visit_Card:
		content = fmt.Sprintf("[名片] %s", msg.VisitInfo.NickName)
	case wechat.Wechat_Message_Type_Voip:
		content = fmt.Sprintf("[通话] %s", msg.VoipInfo.Msg)
	case wechat.Wechat_Message_Type_System:
		content = fmt.Sprintf("[系统消息] %s", msg.Content)
	case wechat.Wechat_Message_Type_Misc:
		content = formatMiscMessage(msg)
	default:
		content = fmt.Sprintf("[未知消息类型:%d] %s", msg.Type, msg.Content)
	}

	return fmt.Sprintf("[%s] %s: %s", msgTime, sender, content)
}

func formatMiscMessage(msg *wechat.WeChatMessage) string {
	switch msg.SubType {
	case wechat.Wechat_Misc_Message_File:
		return fmt.Sprintf("[文件] %s", msg.FileInfo.FileName)
	case wechat.Wechat_Misc_Message_CardLink:
		return fmt.Sprintf("[链接] %s", msg.LinkInfo.Title)
	case wechat.Wechat_Misc_Message_Applet:
		return fmt.Sprintf("[小程序] %s", msg.LinkInfo.Title)
	case wechat.Wechat_Misc_Message_Transfer:
		return fmt.Sprintf("[转账] %s", msg.PayInfo.Feedesc)
	case wechat.Wechat_Misc_Message_Music:
		return fmt.Sprintf("[音乐] %s - %s", msg.MusicInfo.Title, msg.MusicInfo.Description)
	case wechat.Wechat_Misc_Message_Channels:
		return fmt.Sprintf("[视频号] %s", msg.ChannelsInfo.Description)
	case wechat.Wechat_Misc_Message_Live:
		return fmt.Sprintf("[直播] %s", msg.ChannelsInfo.Description)
	case wechat.Wechat_Misc_Message_Refer:
		return fmt.Sprintf("[引用消息] %s", msg.Content)
	default:
		return fmt.Sprintf("[其他消息:%d] %s", msg.SubType, msg.Content)
	}

}
