package wechat

import (
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	sync "sync"
	"time"
	"wechatDataBackup/pkg/utils"

	"github.com/beevik/etree"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pierrec/lz4"
	"google.golang.org/protobuf/proto"
)

const (
	Wechat_Message_Type_Text       = 1
	Wechat_Message_Type_Picture    = 3
	Wechat_Message_Type_Voice      = 34
	Wechat_Message_Type_Visit_Card = 42
	Wechat_Message_Type_Video      = 43
	Wechat_Message_Type_Emoji      = 47
	Wechat_Message_Type_Location   = 48
	Wechat_Message_Type_Misc       = 49
	Wechat_Message_Type_Voip       = 50
	Wechat_Message_Type_System     = 10000
)

const (
	Wechat_Misc_Message_TEXT           = 1
	Wechat_Misc_Message_Music          = 3
	Wechat_Misc_Message_ThirdVideo     = 4
	Wechat_Misc_Message_CardLink       = 5
	Wechat_Misc_Message_File           = 6
	Wechat_Misc_Message_CustomEmoji    = 8
	Wechat_Misc_Message_ShareEmoji     = 15
	Wechat_Misc_Message_ForwardMessage = 19
	Wechat_Misc_Message_Applet         = 33
	Wechat_Misc_Message_Applet2        = 36
	Wechat_Misc_Message_Channels       = 51
	Wechat_Misc_Message_Refer          = 57
	Wechat_Misc_Message_Live           = 63
	Wechat_Misc_Message_Game           = 68
	Wechat_Misc_Message_Notice         = 87
	Wechat_Misc_Message_Live2          = 88
	Wechat_Misc_Message_TingListen     = 92
	Wechat_Misc_Message_Transfer       = 2000
	Wechat_Misc_Message_RedPacket      = 2003
)

const (
	Wechat_System_Message_Notice  = 1
	Wechat_System_Message_Tickle  = 4
	Wechat_System_Message_Notice2 = 8000
)

type Message_Search_Direction int

const (
	Message_Search_Forward Message_Search_Direction = iota
	Message_Search_Backward
	Message_Search_Both
)

type WeChatUserInfo struct {
	UserName        string `json:"UserName"`
	Alias           string `json:"Alias"`
	ReMark          string `json:"ReMark"`
	NickName        string `json:"NickName"`
	SmallHeadImgUrl string `json:"SmallHeadImgUrl"`
	BigHeadImgUrl   string `json:"BigHeadImgUrl"`
	LocalHeadImgUrl string `json:"LocalHeadImgUrl"`
	IsGroup         bool   `json:"IsGroup"`
}

type WeChatSession struct {
	UserName string         `json:"UserName"`
	NickName string         `json:"NickName"`
	Content  string         `json:"Content"`
	UserInfo WeChatUserInfo `json:"UserInfo"`
	Time     uint64         `json:"Time"`
	IsGroup  bool           `json:"IsGroup"`
}

type WeChatSessionList struct {
	Total int             `json:"Total"`
	Rows  []WeChatSession `json:"Rows"`
}

type FileInfo struct {
	FileName string `json:"fileName"`
	FileSize string `json:"fileSize"`
	FilePath string `json:"filePath"`
	FileExt  string `json:"fileExt"`
}

type LinkInfo struct {
	Url         string `json:"Url"`
	Title       string `json:"Title"`
	Description string `json:"Description"`
	DisPlayName string `json:"DisPlayName"`
}

type ReferInfo struct {
	Type        int    `json:"Type"`
	SubType     int    `json:"SubType"`
	Svrid       int64  `json:"Svrid"`
	Displayname string `json:"Displayname"`
	Content     string `json:"Content"`
}

type PayInfo struct {
	Type      int
	Memo      string
	BeginTime string
	Feedesc   string
}

type VoipInfo struct {
	Type int
	Msg  string
}

type ChannelsInfo struct {
	ThumbPath   string
	ThumbCache  string
	NickName    string
	Description string
}

type MusicInfo struct {
	ThumbPath   string
	Title       string
	Description string
	DisPlayName string
	DataUrl     string
}

type LocationInfo struct {
	Label     string
	PoiName   string
	X         string
	Y         string
	ThumbPath string
}

type WeChatMessage struct {
	LocalId         int            `json:"LocalId"`
	MsgSvrId        string         `json:"MsgSvrId"`
	Type            int            `json:"type"`
	SubType         int            `json:"SubType"`
	IsSender        int            `json:"IsSender"`
	CreateTime      int64          `json:"createTime"`
	Talker          string         `json:"talker"`
	Content         string         `json:"content"`
	ThumbPath       string         `json:"ThumbPath"`
	ImagePath       string         `json:"ImagePath"`
	VideoPath       string         `json:"VideoPath"`
	FileInfo        FileInfo       `json:"fileInfo"`
	EmojiPath       string         `json:"EmojiPath"`
	VoicePath       string         `json:"VoicePath"`
	IsChatRoom      bool           `json:"isChatRoom"`
	UserInfo        WeChatUserInfo `json:"userInfo"`
	LinkInfo        LinkInfo       `json:"LinkInfo"`
	ReferInfo       ReferInfo      `json:"ReferInfo"`
	PayInfo         PayInfo        `json:"PayInfo"`
	VoipInfo        VoipInfo       `json:"VoipInfo"`
	VisitInfo       WeChatUserInfo `json:"VisitInfo"`
	ChannelsInfo    ChannelsInfo   `json:"ChannelsInfo"`
	MusicInfo       MusicInfo      `json:"MusicInfo"`
	LocationInfo    LocationInfo   `json:"LocationInfo"`
	compressContent []byte
	bytesExtra      []byte
}

type WeChatMessageList struct {
	KeyWord string          `json:"KeyWord"`
	Total   int             `json:"Total"`
	Rows    []WeChatMessage `json:"Rows"`
}

type WeChatMessageDate struct {
	Date  []string `json:"Date"`
	Total int      `json:"Total"`
}

type WeChatUserList struct {
	Users []WeChatUserInfo `json:"Users"`
	Total int              `json:"Total"`
}

type WeChatContact struct {
	WeChatUserInfo
	PYInitial       string
	QuanPin         string
	RemarkPYInitial string
	RemarkQuanPin   string
}

type WeChatContactList struct {
	Users []WeChatContact `json:"Users"`
	Total int             `json:"Total"`
}

type WeChatAccountInfo struct {
	AccountName     string `json:"AccountName"`
	AliasName       string `json:"AliasName"`
	ReMarkName      string `json:"ReMarkName"`
	NickName        string `json:"NickName"`
	SmallHeadImgUrl string `json:"SmallHeadImgUrl"`
	BigHeadImgUrl   string `json:"BigHeadImgUrl"`
	LocalHeadImgUrl string `json:"LocalHeadImgUrl"`
}

type WeChatLastTime struct {
	UserName  string `json:"UserName"`
	Timestamp int64  `json:"Timestamp"`
	MessageId string `json:"MessageId"`
}

type WeChatBookMark struct {
	MarkId string `json:"MarkId"`
	Tag    string `json:"Tag"`
	Info   string `json:"Info"`
}

type WeChatBookMarkList struct {
	Marks []WeChatBookMark `json:"Marks"`
	Total int              `json:"Total"`
}

type wechatMsgDB struct {
	path      string
	db        *sql.DB
	startTime int64
	endTime   int64
}

type WechatDataProvider struct {
	resPath       string
	prefixResPath string
	microMsg      *sql.DB
	openIMContact *sql.DB
	userData      *sql.DB
	msgDBs        []*wechatMsgDB
	userInfoMap   map[string]WeChatUserInfo
	userInfoMtx   sync.Mutex

	SelfInfo    *WeChatUserInfo
	ContactList *WeChatContactList
	IsShareData bool
}

const (
	MicroMsgDB      = "MicroMsg.db"
	OpenIMContactDB = "OpenIMContact.db"
	UserDataDB      = "UserData.db"
)

type byTime []*wechatMsgDB

func (a byTime) Len() int           { return len(a) }
func (a byTime) Less(i, j int) bool { return a[i].startTime > a[j].startTime }
func (a byTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

type byName []WeChatContact

func (c byName) Len() int { return len(c) }
func (c byName) Less(i, j int) bool {
	var a, b string
	if c[i].RemarkQuanPin != "" {
		a = c[i].RemarkQuanPin
	} else {
		a = c[i].QuanPin
	}

	if c[j].RemarkQuanPin != "" {
		b = c[j].RemarkQuanPin
	} else {
		b = c[j].QuanPin
	}

	return strings.Compare(a, b) < 0
}
func (c byName) Swap(i, j int) { c[i], c[j] = c[j], c[i] }

func CreateWechatDataProvider(resPath string, prefixRes string) (*WechatDataProvider, error) {
	provider := &WechatDataProvider{}
	provider.resPath = resPath
	provider.prefixResPath = prefixRes
	provider.msgDBs = make([]*wechatMsgDB, 0)
	log.Println(resPath)

	userName := filepath.Base(resPath)
	MicroMsgDBPath := resPath + "\\Msg\\" + MicroMsgDB
	if _, err := os.Stat(MicroMsgDBPath); err != nil {
		log.Println("CreateWechatDataProvider failed", MicroMsgDBPath, err)
		return provider, err
	}
	microMsg, err := sql.Open("sqlite3", MicroMsgDBPath)
	if err != nil {
		log.Printf("open db %s error: %v", MicroMsgDBPath, err)
		return provider, err
	}

	var openIMContact *sql.DB
	OpenIMContactDBPath := resPath + "\\Msg\\" + OpenIMContactDB
	if _, err := os.Stat(OpenIMContactDBPath); err == nil {
		openIMContact, err = sql.Open("sqlite3", OpenIMContactDBPath)
		if err != nil {
			log.Printf("open db %s error: %v", OpenIMContactDBPath, err)
		}
	}

	UserDataDBPath := resPath + "\\Msg\\" + UserDataDB
	userData := openUserDataDB(UserDataDBPath)
	if userData == nil {
		log.Printf("open db %s error: %v", UserDataDBPath, err)
		return provider, err
	}

	msgDBPath := fmt.Sprintf("%s\\Msg\\Multi\\MSG.db", provider.resPath)
	if _, err := os.Stat(msgDBPath); err == nil {
		log.Println("msgDBPath", msgDBPath)
		msgDB, err := wechatOpenMsgDB(msgDBPath)
		if err != nil {
			log.Printf("open db %s error: %v", msgDBPath, err)
		} else {
			provider.msgDBs = append(provider.msgDBs, msgDB)
			log.Printf("MSG.db start %d - %d end\n", msgDB.startTime, msgDB.endTime)
			provider.IsShareData = true
		}
	}

	index := 0
	for {
		msgDBPath := fmt.Sprintf("%s\\Msg\\Multi\\MSG%d.db", provider.resPath, index)
		if _, err := os.Stat(msgDBPath); err != nil {
			log.Println("msgDBPath end", msgDBPath)
			break
		}

		msgDB, err := wechatOpenMsgDB(msgDBPath)
		if err != nil {
			log.Printf("open db %s error: %v", msgDBPath, err)
			index += 1
			continue
		}
		provider.msgDBs = append(provider.msgDBs, msgDB)
		log.Printf("MSG%d.db start %d - %d end\n", index, msgDB.startTime, msgDB.endTime)
		index += 1
	}
	sort.Sort(byTime(provider.msgDBs))
	for _, db := range provider.msgDBs {
		log.Printf("%s start %d - %d end\n", db.path, db.startTime, db.endTime)
	}
	provider.userInfoMap = make(map[string]WeChatUserInfo)
	provider.microMsg = microMsg
	provider.openIMContact = openIMContact
	provider.userData = userData
	provider.SelfInfo, err = provider.WechatGetUserInfoByNameOnCache(userName)
	if err != nil {
		log.Printf("WechatGetUserInfoByName %s failed: %v", userName, err)
		return provider, err
	}

	provider.ContactList, err = provider.wechatGetAllContact()
	if err != nil {
		log.Println("wechatGetAllContact failed", err)
		return provider, err
	}
	sort.Sort(byName(provider.ContactList.Users))
	log.Println("Contact number:", provider.ContactList.Total)
	provider.userInfoMap[userName] = *provider.SelfInfo
	log.Println("resPath:", provider.resPath)
	return provider, nil
}

func (P *WechatDataProvider) WechatWechatDataProviderClose() {
	if P.microMsg != nil {
		err := P.microMsg.Close()
		if err != nil {
			log.Println("db close:", err)
		}
	}

	if P.openIMContact != nil {
		err := P.openIMContact.Close()
		if err != nil {
			log.Println("db close:", err)
		}
	}

	if P.userData != nil {
		err := P.userData.Close()
		if err != nil {
			log.Println("db close:", err)
		}
	}

	for _, db := range P.msgDBs {
		err := db.db.Close()
		if err != nil {
			log.Println("db close:", err)
		}
	}
	log.Println("WechatWechatDataProviderClose:", P.resPath)
}

func (P *WechatDataProvider) WechatGetUserInfoByName(name string) (*WeChatUserInfo, error) {
	info := &WeChatUserInfo{}

	var UserName, Alias, ReMark, NickName string
	querySql := fmt.Sprintf("select ifnull(UserName,'') as UserName, ifnull(Alias,'') as Alias, ifnull(ReMark,'') as ReMark, ifnull(NickName,'') as NickName from Contact where UserName='%s';", name)
	// log.Println(querySql)
	err := P.microMsg.QueryRow(querySql).Scan(&UserName, &Alias, &ReMark, &NickName)
	if err != nil {
		// log.Println("not found User:", err)
		return info, err
	}

	// log.Printf("UserName %s, Alias %s, ReMark %s, NickName %s\n", UserName, Alias, ReMark, NickName)

	var smallHeadImgUrl, bigHeadImgUrl string
	querySql = fmt.Sprintf("select ifnull(smallHeadImgUrl,'') as smallHeadImgUrl, ifnull(bigHeadImgUrl,'') as bigHeadImgUrl from ContactHeadImgUrl where usrName='%s';", UserName)
	// log.Println(querySql)
	err = P.microMsg.QueryRow(querySql).Scan(&smallHeadImgUrl, &bigHeadImgUrl)
	if err != nil {
		log.Println("not find headimg", err)
	}

	info.UserName = UserName
	info.Alias = Alias
	info.ReMark = ReMark
	info.NickName = NickName
	info.SmallHeadImgUrl = smallHeadImgUrl
	info.BigHeadImgUrl = bigHeadImgUrl
	info.IsGroup = strings.HasSuffix(UserName, "@chatroom")

	localHeadImgPath := fmt.Sprintf("%s\\FileStorage\\HeadImage\\%s.headimg", P.resPath, name)
	relativePath := fmt.Sprintf("%s\\FileStorage\\HeadImage\\%s.headimg", P.prefixResPath, name)
	if _, err = os.Stat(localHeadImgPath); err == nil {
		info.LocalHeadImgUrl = relativePath
	}
	// log.Println(info)
	return info, nil
}

func (P *WechatDataProvider) WechatGetOpenIMMUserInfoByName(name string) (*WeChatUserInfo, error) {
	info := &WeChatUserInfo{}

	var UserName, ReMark, NickName string
	querySql := fmt.Sprintf("select ifnull(UserName,'') as UserName, ifnull(ReMark,'') as ReMark, ifnull(NickName,'') as NickName from OpenIMContact where UserName='%s';", name)
	// log.Println(querySql)
	if P.openIMContact != nil {
		err := P.openIMContact.QueryRow(querySql).Scan(&UserName, &ReMark, &NickName)
		if err != nil {
			log.Println("not found User:", err)
			return info, err
		}
	}

	log.Printf("UserName %s, ReMark %s, NickName %s\n", UserName, ReMark, NickName)

	var smallHeadImgUrl, bigHeadImgUrl string
	querySql = fmt.Sprintf("select ifnull(smallHeadImgUrl,'') as smallHeadImgUrl, ifnull(bigHeadImgUrl,'') as bigHeadImgUrl from ContactHeadImgUrl where usrName='%s';", UserName)
	// log.Println(querySql)
	err := P.microMsg.QueryRow(querySql).Scan(&smallHeadImgUrl, &bigHeadImgUrl)
	if err != nil {
		log.Println("not find headimg", err)
	}

	info.UserName = UserName
	info.Alias = ""
	info.ReMark = ReMark
	info.NickName = NickName
	info.SmallHeadImgUrl = smallHeadImgUrl
	info.BigHeadImgUrl = bigHeadImgUrl
	info.IsGroup = strings.HasSuffix(UserName, "@chatroom")

	localHeadImgPath := fmt.Sprintf("%s\\FileStorage\\HeadImage\\%s.headimg", P.resPath, name)
	relativePath := fmt.Sprintf("%s\\FileStorage\\HeadImage\\%s.headimg", P.prefixResPath, name)
	if _, err = os.Stat(localHeadImgPath); err == nil {
		info.LocalHeadImgUrl = relativePath
	}
	// log.Println(info)
	return info, nil
}

func (P *WechatDataProvider) WeChatGetSessionList(pageIndex int, pageSize int) (*WeChatSessionList, error) {
	List := &WeChatSessionList{}
	List.Rows = make([]WeChatSession, 0)

	querySql := fmt.Sprintf("select ifnull(strUsrName,'') as strUsrName,ifnull(strNickName,'') as strNickName,ifnull(strContent,'') as strContent, nMsgType, nTime from Session order by nOrder desc limit %d, %d;", pageIndex*pageSize, pageSize)
	dbRows, err := P.microMsg.Query(querySql)
	if err != nil {
		log.Println(err)
		return List, err
	}
	defer dbRows.Close()

	var strUsrName, strNickName, strContent string
	var nTime uint64
	var nMsgType int
	for dbRows.Next() {
		var session WeChatSession
		err = dbRows.Scan(&strUsrName, &strNickName, &strContent, &nMsgType, &nTime)
		if err != nil {
			log.Println(err)
			continue
		}
		if len(strContent) == 0 {
			// log.Printf("%s cotent nil\n", strUsrName)
			continue
		}

		session.UserName = strUsrName
		session.NickName = strNickName
		session.Content = systemMsgParse(nMsgType, strContent)
		session.Time = nTime
		session.IsGroup = strings.HasSuffix(strUsrName, "@chatroom")
		info, err := P.WechatGetUserInfoByNameOnCache(strUsrName)
		if err != nil {
			log.Printf("WechatGetUserInfoByName %s failed\n", strUsrName)
			continue
		}
		session.UserInfo = *info
		List.Rows = append(List.Rows, session)
		List.Total += 1
	}

	return List, nil
}

func (P *WechatDataProvider) WeChatGetContactList(pageIndex int, pageSize int) (*WeChatUserList, error) {
	List := &WeChatUserList{}
	List.Users = make([]WeChatUserInfo, 0)

	if P.ContactList.Total <= pageIndex*pageSize {
		return List, nil
	}
	end := (pageIndex * pageSize) + pageSize
	if end > P.ContactList.Total {
		end = P.ContactList.Total
	}

	log.Printf("P.ContactList.Total %d, start %d, end %d", P.ContactList.Total, pageIndex*pageSize, end)
	var info WeChatUserInfo
	for _, contact := range P.ContactList.Users[pageIndex*pageSize : end] {
		info = contact.WeChatUserInfo
		List.Users = append(List.Users, info)
		List.Total += 1
	}

	return List, nil
}

func (P *WechatDataProvider) WeChatGetMessageListByTime(userName string, time int64, pageSize int, direction Message_Search_Direction) (*WeChatMessageList, error) {

	List := &WeChatMessageList{}
	List.Rows = make([]WeChatMessage, 0)
	selectTime := time
	selectpageSize := pageSize

	if direction == Message_Search_Both {
		selectpageSize = pageSize / 2
	}
	for direction == Message_Search_Forward || direction == Message_Search_Both {
		selectList, err := P.weChatGetMessageListByTime(userName, selectTime, selectpageSize, Message_Search_Forward)
		if err != nil {
			return List, err
		}

		if selectList.Total == 0 {
			break
		}

		selectTime = selectList.Rows[selectList.Total-1].CreateTime - 1
		selectpageSize -= selectList.Total
		List.Total += selectList.Total
		List.Rows = append(List.Rows, selectList.Rows...)
		if selectpageSize <= 0 {
			break
		}
		log.Printf("Forward selectTime %d, selectpageSize %d\n", selectTime, selectpageSize)
	}

	selectTime = time
	if direction == Message_Search_Both {
		selectpageSize = pageSize / 2
	}
	for direction == Message_Search_Backward || direction == Message_Search_Both {
		selectList, err := P.weChatGetMessageListByTime(userName, selectTime, selectpageSize, Message_Search_Backward)
		if err != nil {
			return List, err
		}

		if selectList.Total == 0 {
			break
		}

		selectTime = selectList.Rows[0].CreateTime + 1
		selectpageSize -= selectList.Total
		List.Total += selectList.Total
		List.Rows = append(selectList.Rows, List.Rows...)
		if selectpageSize <= 0 {
			break
		}
		log.Printf("Backward selectTime %d, selectpageSize %d\n", selectTime, selectpageSize)
	}

	return List, nil
}

func (P *WechatDataProvider) weChatGetMessageListByTime(userName string, time int64, pageSize int, direction Message_Search_Direction) (*WeChatMessageList, error) {
	List := &WeChatMessageList{}
	List.Rows = make([]WeChatMessage, 0)
	index := P.wechatFindDBIndex(userName, time, direction)
	if index == -1 {
		log.Printf("Not found %s %d data\n", userName, time)
		return List, nil
	}

	sqlFormat := "select localId,MsgSvrID,Type,SubType,IsSender,CreateTime,ifnull(StrTalker,'') as StrTalker, ifnull(StrContent,'') as StrContent,ifnull(CompressContent,'') as CompressContent,ifnull(BytesExtra,'') as BytesExtra from MSG Where StrTalker='%s' And CreateTime<=%d order by Sequence desc limit %d;"
	if direction == Message_Search_Backward {
		sqlFormat = "select localId,MsgSvrID,Type,SubType,IsSender,CreateTime,ifnull(StrTalker,'') as StrTalker, ifnull(StrContent,'') as StrContent,ifnull(CompressContent,'') as CompressContent,ifnull(BytesExtra,'') as BytesExtra from ( select localId, MsgSvrID, Type, SubType, IsSender, CreateTime, Sequence, StrTalker, StrContent, CompressContent, BytesExtra FROM MSG Where StrTalker='%s' And CreateTime>%d order by Sequence asc limit %d) AS SubQuery order by Sequence desc;"
	}
	querySql := fmt.Sprintf(sqlFormat, userName, time, pageSize)
	log.Println(querySql)

	rows, err := P.msgDBs[index].db.Query(querySql)
	if err != nil {
		log.Printf("%s failed %v\n", querySql, err)
		return List, nil
	}
	defer rows.Close()
	var localId, Type, SubType, IsSender int
	var MsgSvrID, CreateTime int64
	var StrTalker, StrContent string
	var CompressContent, BytesExtra []byte

	for rows.Next() {
		message := WeChatMessage{}
		err = rows.Scan(&localId, &MsgSvrID, &Type, &SubType, &IsSender, &CreateTime,
			&StrTalker, &StrContent, &CompressContent, &BytesExtra)
		if err != nil {
			log.Println("rows.Scan failed", err)
			return List, err
		}

		message.LocalId = localId
		message.MsgSvrId = fmt.Sprintf("%d", MsgSvrID)
		message.Type = Type
		message.SubType = SubType
		message.IsSender = IsSender
		message.CreateTime = CreateTime
		message.Talker = StrTalker
		message.Content = systemMsgParse(Type, StrContent)
		message.IsChatRoom = strings.HasSuffix(StrTalker, "@chatroom")
		message.compressContent = make([]byte, len(CompressContent))
		message.bytesExtra = make([]byte, len(BytesExtra))
		copy(message.compressContent, CompressContent)
		copy(message.bytesExtra, BytesExtra)
		P.wechatMessageExtraHandle(&message)
		P.wechatMessageGetUserInfo(&message)
		P.wechatMessageEmojiHandle(&message)
		P.wechatMessageCompressContentHandle(&message)
		P.wechatMessageVoipHandle(&message)
		P.wechatMessageVisitHandke(&message)
		P.wechatMessageLocationHandke(&message)
		List.Rows = append(List.Rows, message)
		List.Total += 1
	}

	if err := rows.Err(); err != nil {
		log.Println("rows.Scan failed", err)
		return List, err
	}

	return List, nil
}

func (P *WechatDataProvider) WeChatGetMessageListByKeyWord(userName string, time int64, keyWord string, msgType string, pageSize int) (*WeChatMessageList, error) {
	List := &WeChatMessageList{}
	List.Rows = make([]WeChatMessage, 0)
	List.KeyWord = keyWord
	_time := time
	selectPagesize := pageSize
	if keyWord != "" || msgType != "" {
		selectPagesize = 600
	}
	for {
		log.Println("time:", _time, keyWord)
		rawList, err := P.weChatGetMessageListByTime(userName, _time, selectPagesize, Message_Search_Forward)
		if err != nil {
			log.Println("weChatGetMessageListByTime failed: ", err)
			return nil, err
		}
		log.Println("rawList.Total:", rawList.Total)
		if rawList.Total == 0 {
			if List.Total == 0 {
				log.Printf("user %s not find [%s]\n", userName, keyWord)
			}
			break
		}

		for i, _ := range rawList.Rows {
			if weChatMessageTypeFilter(&rawList.Rows[i], msgType) && (len(keyWord) == 0 || weChatMessageContains(&rawList.Rows[i], keyWord)) {
				List.Rows = append(List.Rows, rawList.Rows[i])
				List.Total += 1
				if List.Total >= pageSize {
					return List, nil
				}
			}
		}

		_time = rawList.Rows[rawList.Total-1].CreateTime - 1
	}

	return List, nil
}

func (P *WechatDataProvider) WeChatGetMessageListByType(userName string, time int64, pageSize int, msgType string, direction Message_Search_Direction) (*WeChatMessageList, error) {

	List := &WeChatMessageList{}
	List.Rows = make([]WeChatMessage, 0)
	selectTime := time
	selectpageSize := 30
	needSize := pageSize

	if msgType != "" {
		selectpageSize = 600
	}
	if direction == Message_Search_Both {
		needSize = pageSize / 2
	}
	for direction == Message_Search_Forward || direction == Message_Search_Both {
		selectList, err := P.weChatGetMessageListByTime(userName, selectTime, selectpageSize, Message_Search_Forward)
		if err != nil {
			return List, err
		}

		if selectList.Total == 0 {
			break
		}

		for i, _ := range selectList.Rows {
			if weChatMessageTypeFilter(&selectList.Rows[i], msgType) {
				List.Rows = append(List.Rows, selectList.Rows[i])
				List.Total += 1
				needSize -= 1
				if needSize <= 0 {
					break
				}
			}
		}
		if needSize <= 0 {
			break
		}
		selectTime = selectList.Rows[selectList.Total-1].CreateTime - 1
		log.Printf("Forward selectTime %d, selectpageSize %d needSize %d\n", selectTime, selectpageSize, needSize)
	}

	selectTime = time
	if direction == Message_Search_Both {
		needSize = pageSize / 2
	}
	for direction == Message_Search_Backward || direction == Message_Search_Both {
		selectList, err := P.weChatGetMessageListByTime(userName, selectTime, selectpageSize, Message_Search_Backward)
		if err != nil {
			return List, err
		}

		if selectList.Total == 0 {
			break
		}

		tmpTotal := 0
		tmpRows := make([]WeChatMessage, 0)
		for i := selectList.Total - 1; i >= 0; i-- {
			if weChatMessageTypeFilter(&selectList.Rows[i], msgType) {
				tmpRows = append([]WeChatMessage{selectList.Rows[i]}, tmpRows...)
				tmpTotal += 1
				needSize -= 1
				if needSize <= 0 {
					break
				}
			}
		}
		if tmpTotal > 0 {
			List.Rows = append(tmpRows, List.Rows...)
			List.Total += tmpTotal
		}
		selectTime = selectList.Rows[0].CreateTime + 1
		if needSize <= 0 {
			break
		}
		log.Printf("Backward selectTime %d, selectpageSize %d needSize %d\n", selectTime, selectpageSize, needSize)
	}

	return List, nil
}

func (P *WechatDataProvider) WeChatGetMessageDate(userName string) (*WeChatMessageDate, error) {
	messageData := &WeChatMessageDate{}
	messageData.Date = make([]string, 0)
	messageData.Total = 0

	_time := time.Now().Unix()

	for {
		index := P.wechatFindDBIndex(userName, _time, Message_Search_Forward)
		if index == -1 {
			log.Println("wechat find db end")
			return messageData, nil
		}

		sqlFormat := " SELECT DISTINCT strftime('%%Y-%%m-%%d', datetime(CreateTime+28800, 'unixepoch')) FROM MSG WHERE StrTalker='%s' order by CreateTime desc;"
		querySql := fmt.Sprintf(sqlFormat, userName)

		rows, err := P.msgDBs[index].db.Query(querySql)
		if err != nil {
			log.Printf("%s failed %v\n", querySql, err)
			return messageData, nil
		}
		defer rows.Close()

		var date string
		for rows.Next() {
			err = rows.Scan(&date)
			if err != nil {
				log.Println("rows.Scan failed", err)
				return messageData, err
			}

			messageData.Date = append(messageData.Date, date)
			messageData.Total += 1
		}

		if err := rows.Err(); err != nil {
			log.Println("rows.Scan failed", err)
			return messageData, err
		}

		_time = P.wechatGetLastMessageCreateTime(userName, index)
		if -1 == _time {
			log.Println("wechatGetLastMessageCreateTime failed")
			return messageData, errors.New("wechatGetLastMessageCreateTime failed")
		}

		_time -= 1
	}
}

func (P *WechatDataProvider) WeChatGetChatRoomUserList(chatroom string) (*WeChatUserList, error) {
	userList := &WeChatUserList{}
	userList.Users = make([]WeChatUserInfo, 0)
	userList.Total = 0

	sqlFormat := "select UserNameList from ChatRoom where ChatRoomName='%s';"
	querySql := fmt.Sprintf(sqlFormat, chatroom)

	var userNameListStr string
	err := P.microMsg.QueryRow(querySql).Scan(&userNameListStr)
	if err != nil {
		log.Println("Scan: ", err)
		return nil, err
	}

	userNameArray := strings.Split(userNameListStr, "^G")
	log.Println("userNameArray:", userNameArray)

	for _, userName := range userNameArray {
		pinfo, err := P.WechatGetUserInfoByNameOnCache(userName)
		if err == nil {
			userList.Users = append(userList.Users, *pinfo)
			userList.Total += 1
		}
	}

	return userList, nil
}

func (info WeChatUserInfo) String() string {
	return fmt.Sprintf("NickName:[%s] Alias:[%s], NickName:[%s], ReMark:[%s], SmallHeadImgUrl:[%s], BigHeadImgUrl[%s]",
		info.NickName, info.Alias, info.NickName, info.ReMark, info.SmallHeadImgUrl, info.BigHeadImgUrl)
}

func (P *WechatDataProvider) wechatMessageExtraHandle(msg *WeChatMessage) {
	var extra MessageBytesExtra
	err := proto.Unmarshal(msg.bytesExtra, &extra)
	if err != nil {
		log.Println("proto.Unmarshal failed", err)
		return
	}

	for _, ext := range extra.Message2 {
		switch ext.Field1 {
		case 1:
			if msg.IsChatRoom {
				msg.UserInfo.UserName = ext.Field2
			}
		case 3:
			if len(ext.Field2) > 0 {
				if msg.Type == Wechat_Message_Type_Picture || msg.Type == Wechat_Message_Type_Video || msg.Type == Wechat_Message_Type_Misc {
					msg.ThumbPath = P.prefixResPath + ext.Field2[len(P.SelfInfo.UserName):]
				}

				if msg.Type == Wechat_Message_Type_Misc && (msg.SubType == Wechat_Misc_Message_Music || msg.SubType == Wechat_Misc_Message_TingListen) {
					msg.MusicInfo.ThumbPath = P.prefixResPath + ext.Field2[len(P.SelfInfo.UserName):]
				} else if msg.Type == Wechat_Message_Type_Location {
					msg.LocationInfo.ThumbPath = P.prefixResPath + ext.Field2[len(P.SelfInfo.UserName):]
				}
			}
		case 4:
			if len(ext.Field2) > 0 {
				if msg.Type == Wechat_Message_Type_Misc && msg.SubType == Wechat_Misc_Message_File {
					msg.FileInfo.FilePath = P.prefixResPath + ext.Field2[len(P.SelfInfo.UserName):]
					msg.FileInfo.FileName = filepath.Base(ext.Field2)
				} else if msg.Type == Wechat_Message_Type_Picture || msg.Type == Wechat_Message_Type_Video || msg.Type == Wechat_Message_Type_Misc {
					msg.ImagePath = P.prefixResPath + ext.Field2[len(P.SelfInfo.UserName):]
					msg.VideoPath = P.prefixResPath + ext.Field2[len(P.SelfInfo.UserName):]
				}
			}
		}
	}

	if msg.Type == Wechat_Message_Type_Voice {
		msg.VoicePath = fmt.Sprintf("%s\\FileStorage\\Voice\\%s.mp3", P.prefixResPath, msg.MsgSvrId)
	}
}

type EmojiMsg struct {
	XMLName xml.Name `xml:"msg"`
	Emoji   Emoji    `xml:"emoji"`
}

type Emoji struct {
	XMLName  xml.Name `xml:"emoji"`
	CdnURL   string   `xml:"cdnurl,attr"`
	Thumburl string   `xml:"thumburl,attr"`
	Width    string   `xml:"width,attr"`
	Height   string   `xml:"height,attr"`
}

func (P *WechatDataProvider) wechatMessageEmojiHandle(msg *WeChatMessage) {
	if msg.Type != Wechat_Message_Type_Emoji {
		return
	}

	emojiMsg := EmojiMsg{}
	err := xml.Unmarshal([]byte(msg.Content), &emojiMsg)
	if err != nil {
		log.Println("xml.Unmarshal failed: ", err, msg.Content)
		return
	}

	msg.EmojiPath = emojiMsg.Emoji.CdnURL
}

type xmlDocument struct {
	*etree.Document
}

func NewxmlDocument(e *etree.Document) *xmlDocument {
	return &xmlDocument{e}
}

func (e *xmlDocument) FindElementValue(path string) string {
	item := e.FindElement(path)
	if item != nil {
		return item.Text()
	}

	return ""
}

func (P *WechatDataProvider) wechatMessageCompressContentHandle(msg *WeChatMessage) {
	if len(msg.compressContent) == 0 {
		return
	}

	unCompressContent := make([]byte, len(msg.compressContent)*10)
	ulen, err := lz4.UncompressBlock(msg.compressContent, unCompressContent)
	if err != nil {
		log.Println("UncompressBlock failed:", err, msg.MsgSvrId)
		return
	}

	compMsg := etree.NewDocument()
	if err := compMsg.ReadFromBytes(unCompressContent[:ulen-1]); err != nil {
		// os.WriteFile("D:\\tmp\\"+string(msg.LocalId)+".xml", unCompressContent[:ulen], 0600)
		log.Println("ReadFromBytes failed:", err)
		return
	}
	root := NewxmlDocument(compMsg)
	if msg.Type == Wechat_Message_Type_Misc && isLinkSubType(msg.SubType) {
		msg.LinkInfo.Title = root.FindElementValue("/msg/appmsg/title")
		msg.LinkInfo.Description = root.FindElementValue("/msg/appmsg/des")
		msg.LinkInfo.Url = root.FindElementValue("/msg/appmsg/url")
		msg.LinkInfo.DisPlayName = root.FindElementValue("/msg/appmsg/sourcedisplayname")
		appName := root.FindElementValue("/msg/appinfo/appname")
		if len(msg.LinkInfo.DisPlayName) == 0 && len(appName) > 0 {
			msg.LinkInfo.DisPlayName = appName
		}
		thumburl := root.FindElementValue("/msg/appmsg/thumburl")
		if len(msg.ThumbPath) == 0 && len(thumburl) > 0 && strings.HasPrefix(thumburl, "http") {
			msg.ThumbPath = thumburl
		}
	} else if msg.Type == Wechat_Message_Type_Misc && msg.SubType == Wechat_Misc_Message_Refer {
		msg.Content = root.FindElementValue("/msg/appmsg/title")
		msg.ReferInfo.Type, _ = strconv.Atoi(root.FindElementValue("/msg/appmsg/refermsg/type"))
		msg.ReferInfo.Svrid, _ = strconv.ParseInt(root.FindElementValue("/msg/appmsg/refermsg/svrid"), 10, 64)
		msg.ReferInfo.Displayname = root.FindElementValue("/msg/appmsg/refermsg/displayname")
		msg.ReferInfo.Content = root.FindElementValue("/msg/appmsg/refermsg/content")

		if msg.ReferInfo.Type == Wechat_Message_Type_Misc {
			contentXML := etree.NewDocument()
			if err := contentXML.ReadFromString(msg.ReferInfo.Content); err != nil {
				log.Println("ReadFromString failed:", err)
				return
			}

			root := NewxmlDocument(contentXML)
			msg.ReferInfo.Content = root.FindElementValue("/msg/appmsg/title")
			msg.ReferInfo.SubType, _ = strconv.Atoi(root.FindElementValue("/msg/appmsg/type"))
		}
	} else if msg.Type == Wechat_Message_Type_Misc && msg.SubType == Wechat_Misc_Message_Transfer {
		msg.PayInfo.Type, _ = strconv.Atoi(root.FindElementValue("/msg/appmsg/wcpayinfo/paysubtype"))
		msg.PayInfo.Feedesc = root.FindElementValue("/msg/appmsg/wcpayinfo/feedesc")
		msg.PayInfo.BeginTime = root.FindElementValue("/msg/appmsg/wcpayinfo/begintransfertime")
		msg.PayInfo.Memo = root.FindElementValue("/msg/appmsg/wcpayinfo/pay_memo")
	} else if msg.Type == Wechat_Message_Type_Misc && msg.SubType == Wechat_Misc_Message_TEXT {
		msg.Content = root.FindElementValue("/msg/appmsg/title")
	} else if msg.Type == Wechat_Message_Type_Misc && msg.SubType == Wechat_Misc_Message_Channels {
		msg.ChannelsInfo.NickName = root.FindElementValue("/msg/appmsg/finderFeed/nickname")
		msg.ChannelsInfo.ThumbPath = root.FindElementValue("/msg/appmsg/finderFeed/mediaList/media/thumbUrl")
		msg.ChannelsInfo.Description = root.FindElementValue("/msg/appmsg/finderFeed/desc")
		msg.ChannelsInfo.ThumbPath = P.urlconvertCacheName(msg.ChannelsInfo.ThumbPath, msg.CreateTime)
	} else if msg.Type == Wechat_Message_Type_Misc && msg.SubType == Wechat_Misc_Message_Live {
		msg.ChannelsInfo.NickName = root.FindElementValue("/msg/appmsg/finderLive/nickname")
		msg.ChannelsInfo.ThumbPath = root.FindElementValue("/msg/appmsg/finderLive/media/coverUrl")
		msg.ChannelsInfo.Description = root.FindElementValue("/msg/appmsg/finderLive/desc")
		msg.ChannelsInfo.ThumbPath = P.urlconvertCacheName(msg.ChannelsInfo.ThumbPath, msg.CreateTime)
	} else if msg.Type == Wechat_Message_Type_Misc && (msg.SubType == Wechat_Misc_Message_Music || msg.SubType == Wechat_Misc_Message_TingListen) {
		msg.MusicInfo.Title = root.FindElementValue("/msg/appmsg/title")
		msg.MusicInfo.Description = root.FindElementValue("/msg/appmsg/des")
		msg.MusicInfo.DataUrl = root.FindElementValue("/msg/appmsg/dataurl")
		msg.MusicInfo.DisPlayName = root.FindElementValue("/msg/appinfo/appname")
	}
}

func (P *WechatDataProvider) wechatMessageVoipHandle(msg *WeChatMessage) {
	if msg.Type != Wechat_Message_Type_Voip {
		return
	}

	xmlMsg := etree.NewDocument()
	if err := xmlMsg.ReadFromBytes([]byte(msg.Content)); err != nil {
		// os.WriteFile("D:\\tmp\\"+string(msg.LocalId)+".xml", unCompressContent[:ulen], 0600)
		log.Println("ReadFromBytes failed:", err)
		return
	}
	root := NewxmlDocument(xmlMsg)
	msg.VoipInfo.Type, _ = strconv.Atoi(root.FindElementValue("/voipmsg/VoIPBubbleMsg/room_type"))
	msg.VoipInfo.Msg = root.FindElementValue("/voipmsg/VoIPBubbleMsg/msg")
}

func (P *WechatDataProvider) wechatMessageVisitHandke(msg *WeChatMessage) {
	if msg.Type != Wechat_Message_Type_Visit_Card {
		return
	}

	attr := utils.HtmlMsgGetAttr(msg.Content, "msg")
	userName, exists := attr["username"]
	if !exists {
		return
	}

	userInfo, err := P.WechatGetUserInfoByNameOnCache(userName)
	if err == nil {
		msg.VisitInfo = *userInfo
	} else {
		msg.VisitInfo.UserName = userName
		msg.VisitInfo.Alias = attr["alias"]
		msg.VisitInfo.NickName = attr["nickname"]
		msg.VisitInfo.SmallHeadImgUrl = attr["smallheadimgurl"]
		msg.VisitInfo.BigHeadImgUrl = attr["bigheadimgurl"]
		localHeadImgPath := fmt.Sprintf("%s\\FileStorage\\HeadImage\\%s.headimg", P.resPath, userName)
		relativePath := fmt.Sprintf("%s\\FileStorage\\HeadImage\\%s.headimg", P.prefixResPath, userName)
		if _, err = os.Stat(localHeadImgPath); err == nil {
			msg.VisitInfo.LocalHeadImgUrl = relativePath
		}
	}
}

func (P *WechatDataProvider) wechatMessageLocationHandke(msg *WeChatMessage) {
	if msg.Type != Wechat_Message_Type_Location {
		return
	}

	attr := utils.HtmlMsgGetAttr(msg.Content, "location")
	msg.LocationInfo.Label = attr["label"]
	msg.LocationInfo.PoiName = attr["poiname"]
	msg.LocationInfo.X = attr["x"]
	msg.LocationInfo.Y = attr["y"]
}

func (P *WechatDataProvider) wechatMessageGetUserInfo(msg *WeChatMessage) {
	who := msg.Talker
	if msg.IsSender == 1 {
		who = P.SelfInfo.UserName
	} else if msg.IsChatRoom {
		who = msg.UserInfo.UserName
	}

	pinfo, err := P.WechatGetUserInfoByNameOnCache(who)
	if err != nil {
		// log.Println("WechatGetUserInfoByNameOnCache:", err)
		return
	}

	msg.UserInfo = *pinfo
}

func (P *WechatDataProvider) wechatFindDBIndex(userName string, time int64, direction Message_Search_Direction) int {
	if direction == Message_Search_Forward {
		index := 0
		for {
			if index >= len(P.msgDBs) {
				return -1
			}
			msgDB := P.msgDBs[index]

			if msgDB.startTime > time {
				index += 1
				continue
			}

			rowId := 0
			querySql := fmt.Sprintf("select rowid from Name2ID where UsrName='%s';", userName)
			err := msgDB.db.QueryRow(querySql).Scan(&rowId)
			if err != nil {
				log.Printf("Scan: %v\n", err)
				index += 1
				continue
			}

			querySql = fmt.Sprintf(" select rowid from MSG where StrTalker='%s' AND CreateTime<=%d limit 1;", userName, time)
			log.Printf("in %s, %s\n", msgDB.path, querySql)
			err = msgDB.db.QueryRow(querySql).Scan(&rowId)
			if err != nil {
				log.Printf("Scan: %v\n", err)
				index += 1
				continue
			}

			log.Printf("Select in %d %s\n", index, msgDB.path)
			return index
		}
	} else {
		index := len(P.msgDBs) - 1
		for {
			if index < 0 {
				return -1
			}
			msgDB := P.msgDBs[index]

			if msgDB.endTime < time {
				index -= 1
				continue
			}

			rowId := 0
			querySql := fmt.Sprintf("select rowid from Name2ID where UsrName='%s';", userName)
			err := msgDB.db.QueryRow(querySql).Scan(&rowId)
			if err != nil {
				log.Printf("Scan: %v\n", err)
				index -= 1
				continue
			}

			querySql = fmt.Sprintf(" select rowid from MSG where StrTalker='%s' AND CreateTime>%d limit 1;", userName, time)
			log.Printf("in %s, %s\n", msgDB.path, querySql)
			err = msgDB.db.QueryRow(querySql).Scan(&rowId)
			if err != nil {
				log.Printf("Scan: %v\n", err)
				index -= 1
				continue
			}

			log.Printf("Select in %d %s\n", index, msgDB.path)
			return index
		}
	}
}

func (P *WechatDataProvider) wechatGetLastMessageCreateTime(userName string, index int) int64 {
	if index >= len(P.msgDBs) {
		return -1
	}
	sqlFormat := "SELECT CreateTime FROM MSG WHERE StrTalker='%s' order by CreateTime asc limit 1;"
	querySql := fmt.Sprintf(sqlFormat, userName)
	var lastTime int64
	err := P.msgDBs[index].db.QueryRow(querySql).Scan(&lastTime)
	if err != nil {
		log.Println("select DB lastTime failed:", index, ":", err)
		return -1
	}

	return lastTime
}

func weChatMessageContains(msg *WeChatMessage, chars string) bool {

	switch msg.Type {
	case Wechat_Message_Type_Text:
		return strings.Contains(msg.Content, chars)
	case Wechat_Message_Type_Location:
		return strings.Contains(msg.LocationInfo.Label, chars) || strings.Contains(msg.LocationInfo.PoiName, chars)
	case Wechat_Message_Type_Misc:
		switch msg.SubType {
		case Wechat_Misc_Message_CardLink, Wechat_Misc_Message_ThirdVideo, Wechat_Misc_Message_Applet, Wechat_Misc_Message_Applet2:
			return strings.Contains(msg.LinkInfo.Title, chars) || strings.Contains(msg.LinkInfo.Description, chars)
		case Wechat_Misc_Message_Refer:
			return strings.Contains(msg.Content, chars)
		case Wechat_Misc_Message_File:
			return strings.Contains(msg.FileInfo.FileName, chars)
		default:
			return false
		}
	default:
		return false
	}
}

func weChatMessageTypeFilter(msg *WeChatMessage, msgType string) bool {
	switch msgType {
	case "":
		return true
	case "文件":
		return msg.Type == Wechat_Message_Type_Misc && msg.SubType == Wechat_Misc_Message_File
	case "图片与视频":
		return msg.Type == Wechat_Message_Type_Picture || msg.Type == Wechat_Message_Type_Video
	case "链接":
		return msg.Type == Wechat_Message_Type_Misc && (msg.SubType == Wechat_Misc_Message_CardLink || msg.SubType == Wechat_Misc_Message_ThirdVideo)
	case "语音":
		return msg.Type == Wechat_Message_Type_Voice
	case "通话":
		return msg.Type == Wechat_Message_Type_Voip
	default:
		if strings.HasPrefix(msgType, "群成员") {
			userName := msgType[len("群成员"):]
			return msg.UserInfo.UserName == userName
		}

		return false
	}
}

func wechatOpenMsgDB(path string) (*wechatMsgDB, error) {
	msgDB := wechatMsgDB{}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		log.Printf("open db %s error: %v", path, err)
		return nil, err
	}
	msgDB.db = db
	msgDB.path = path
	querySql := "select CreateTime from MSG order by CreateTime asc limit 1;"
	err = msgDB.db.QueryRow(querySql).Scan(&msgDB.startTime)
	if err != nil {
		log.Println("select DB startTime failed:", path, ":", err)
		msgDB.db.Close()
		return nil, err
	}

	querySql = "select CreateTime from MSG order by CreateTime desc limit 1;"
	err = msgDB.db.QueryRow(querySql).Scan(&msgDB.endTime)
	if err != nil {
		log.Println("select DB endTime failed:", path, ":", err)
		msgDB.db.Close()
		return nil, err
	}

	return &msgDB, nil
}

func (P *WechatDataProvider) WechatGetUserInfoByNameOnCache(name string) (*WeChatUserInfo, error) {

	// log.Printf("who: %s", who)

	P.userInfoMtx.Lock()
	defer P.userInfoMtx.Unlock()

	info, ok := P.userInfoMap[name]
	if ok {
		return &info, nil
	}

	var pinfo *WeChatUserInfo
	var err error
	if strings.HasSuffix(name, "@openim") {
		pinfo, err = P.WechatGetOpenIMMUserInfoByName(name)
	} else {
		pinfo, err = P.WechatGetUserInfoByName(name)
	}
	if err != nil {
		// log.Printf("WechatGetUserInfoByName %s failed: %v\n", name, err)
		return nil, err
	}

	P.userInfoMap[name] = *pinfo

	return pinfo, nil
}

func (P *WechatDataProvider) wechatGetAllContact() (*WeChatContactList, error) {
	List := &WeChatContactList{}
	List.Users = make([]WeChatContact, 0)

	querySql := fmt.Sprintf("select ifnull(UserName,'') as UserName,Reserved1,Reserved2,ifnull(PYInitial,'') as PYInitial,ifnull(QuanPin,'') as QuanPin,ifnull(RemarkPYInitial,'') as RemarkPYInitial,ifnull(RemarkQuanPin,'') as RemarkQuanPin from Contact desc;")
	dbRows, err := P.microMsg.Query(querySql)
	if err != nil {
		log.Println(err)
		return List, err
	}
	defer dbRows.Close()

	var UserName string
	var Reserved1, Reserved2 int
	for dbRows.Next() {
		var Contact WeChatContact
		err = dbRows.Scan(&UserName, &Reserved1, &Reserved2, &Contact.PYInitial, &Contact.QuanPin, &Contact.RemarkPYInitial, &Contact.RemarkQuanPin)
		if err != nil {
			log.Println(err)
			continue
		}

		if Reserved1 != 1 || Reserved2 != 1 {
			// log.Printf("%s is not your contact", UserName)
			continue
		}
		info, err := P.WechatGetUserInfoByNameOnCache(UserName)
		if err != nil {
			log.Printf("WechatGetUserInfoByName %s failed\n", UserName)
			continue
		}

		if info.NickName == "" && info.ReMark == "" {
			continue
		}
		Contact.WeChatUserInfo = *info
		List.Users = append(List.Users, Contact)
		List.Total += 1
	}

	return List, nil
}

func WechatGetAccountInfo(resPath, prefixRes, accountName string) (*WeChatAccountInfo, error) {
	MicroMsgDBPath := resPath + "\\Msg\\" + MicroMsgDB
	if _, err := os.Stat(MicroMsgDBPath); err != nil {
		log.Println("MicroMsgDBPath:", MicroMsgDBPath, err)
		return nil, err
	}

	microMsg, err := sql.Open("sqlite3", MicroMsgDBPath)
	if err != nil {
		log.Printf("open db %s error: %v", MicroMsgDBPath, err)
		return nil, err
	}
	defer microMsg.Close()

	info := &WeChatAccountInfo{}

	var UserName, Alias, ReMark, NickName string
	querySql := fmt.Sprintf("select ifnull(UserName,'') as UserName, ifnull(Alias,'') as Alias, ifnull(ReMark,'') as ReMark, ifnull(NickName,'') as NickName from Contact where UserName='%s';", accountName)
	// log.Println(querySql)
	err = microMsg.QueryRow(querySql).Scan(&UserName, &Alias, &ReMark, &NickName)
	if err != nil {
		log.Println("not found User:", err)
		return nil, err
	}

	log.Printf("UserName %s, Alias %s, ReMark %s, NickName %s\n", UserName, Alias, ReMark, NickName)

	var smallHeadImgUrl, bigHeadImgUrl string
	querySql = fmt.Sprintf("select ifnull(smallHeadImgUrl,'') as smallHeadImgUrl, ifnull(bigHeadImgUrl,'') as bigHeadImgUrl from ContactHeadImgUrl where usrName='%s';", UserName)
	// log.Println(querySql)
	err = microMsg.QueryRow(querySql).Scan(&smallHeadImgUrl, &bigHeadImgUrl)
	if err != nil {
		log.Println("not find headimg", err)
	}

	info.AccountName = UserName
	info.AliasName = Alias
	info.ReMarkName = ReMark
	info.NickName = NickName
	info.SmallHeadImgUrl = smallHeadImgUrl
	info.BigHeadImgUrl = bigHeadImgUrl

	localHeadImgPath := fmt.Sprintf("%s\\FileStorage\\HeadImage\\%s.headimg", resPath, accountName)
	relativePath := fmt.Sprintf("%s\\FileStorage\\HeadImage\\%s.headimg", prefixRes, accountName)
	if _, err = os.Stat(localHeadImgPath); err == nil {
		info.LocalHeadImgUrl = relativePath
	}
	// log.Println(info)
	return info, nil
}

func systemMsgParse(msgType int, content string) string {
	if msgType != Wechat_Message_Type_System {
		return content
	}

	return utils.Html2Text(content)
}

func (P *WechatDataProvider) urlconvertCacheName(url string, timestamp int64) string {
	t := time.Unix(timestamp, 0)
	yearMonth := t.Format("2006-01")
	md5String := utils.Hash256Sum([]byte(url))
	realPath := fmt.Sprintf("%s\\FileStorage\\Cache\\%s\\%s.jpg", P.resPath, yearMonth, md5String)
	path := fmt.Sprintf("%s\\FileStorage\\Cache\\%s\\%s.jpg", P.prefixResPath, yearMonth, md5String)

	if _, err := os.Stat(realPath); err == nil {
		return path
	}

	return url
}

func isLinkSubType(subType int) bool {
	targetSubTypes := map[int]bool{
		Wechat_Misc_Message_CardLink:   true,
		Wechat_Misc_Message_ThirdVideo: true,
		Wechat_Misc_Message_ShareEmoji: true,
		Wechat_Misc_Message_Applet:     true,
		Wechat_Misc_Message_Applet2:    true,
		Wechat_Misc_Message_Game:       true,
	}
	return targetSubTypes[subType]
}

func openUserDataDB(path string) *sql.DB {
	if _, err := os.Stat(path); err == nil {
		sql, err := sql.Open("sqlite3", path)
		if err != nil {
			log.Printf("open db %s error: %v", path, err)
			return nil
		}

		return sql
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		log.Printf("open db %s error: %v", path, err)
		return nil
	}

	createLastTimeTable := `
	CREATE TABLE IF NOT EXISTS lastTime (
		localId INTEGER PRIMARY KEY AUTOINCREMENT,
		userName TEXT,
		timestamp INT,
		messageId TEXT,
		Reserved0 INT DEFAULT 0,
		Reserved1 INT DEFAULT 0,
		Reserved2 TEXT,
		Reserved3 TEXT
	);`

	_, err = db.Exec(createLastTimeTable)
	if err != nil {
		log.Printf("create lastTime table failed: %v", err)
		db.Close()
		return nil
	}

	createBookMarkTable := `
	CREATE TABLE IF NOT EXISTS bookMark (
		localId INTEGER PRIMARY KEY AUTOINCREMENT,
		userName TEXT,
		markId TEXT,
		tag TEXT,
		info TEXT,
		Reserved0 INT DEFAULT 0,
		Reserved1 INT DEFAULT 0,
		Reserved2 TEXT,
		Reserved3 TEXT
	);`

	_, err = db.Exec(createBookMarkTable)
	if err != nil {
		log.Printf("create bookMark table failed: %v", err)
		db.Close()
		return nil
	}

	return db
}

func (P *WechatDataProvider) WeChatGetSessionLastTime(userName string) *WeChatLastTime {
	lastTime := &WeChatLastTime{}
	if P.userData == nil {
		log.Println("userData DB is nill")
		return lastTime
	}

	var timestamp int64
	var messageId string
	querySql := fmt.Sprintf("select timestamp, messageId from lastTime where userName='%s';", userName)
	err := P.userData.QueryRow(querySql).Scan(&timestamp, &messageId)
	if err != nil {
		log.Println("select DB timestamp failed:", err)
		return lastTime
	}

	lastTime.UserName = userName
	lastTime.MessageId = messageId
	lastTime.Timestamp = timestamp
	return lastTime
}

func (P *WechatDataProvider) WeChatSetSessionLastTime(lastTime *WeChatLastTime) error {
	var count int
	querySql := fmt.Sprintf("select COUNT(*) from lastTime where userName='%s';", lastTime.UserName)
	err := P.userData.QueryRow(querySql).Scan(&count)
	if err != nil {
		log.Println("select DB timestamp count failed:", err)
		return err
	}

	if count > 0 {
		_, err := P.userData.Exec("UPDATE lastTime SET timestamp = ?, messageId = ? WHERE userName = ?", lastTime.Timestamp, lastTime.MessageId, lastTime.UserName)
		if err != nil {
			return fmt.Errorf("update timestamp failed: %v", err)
		}
	} else {
		_, err := P.userData.Exec("INSERT INTO lastTime (userName, timestamp, messageId) VALUES (?, ?, ?)", lastTime.UserName, lastTime.Timestamp, lastTime.MessageId)
		if err != nil {
			return fmt.Errorf("insert failed: %v", err)
		}
	}

	log.Printf("WeChatSetSessionLastTime %s %d %s done!\n", lastTime.UserName, lastTime.Timestamp, lastTime.MessageId)
	return nil
}

func (P *WechatDataProvider) WeChatSetSessionBookMask(userName, tag, info string) error {
	markId := utils.Hash256Sum([]byte(info))
	querySql := fmt.Sprintf("select COUNT(*) from bookMark where markId='%s';", markId)
	var count int

	err := P.userData.QueryRow(querySql).Scan(&count)
	if err != nil {
		log.Println("select DB markId count failed:", err)
		return err
	}

	if count > 0 {
		log.Printf("exist userName: %s, tag: %s, info: %s, markId: %s\n", userName, tag, info, markId)
		return nil
	}

	_, err = P.userData.Exec("INSERT INTO bookMark (userName, markId, tag, info) VALUES (?, ?, ?, ?)", userName, markId, tag, info)
	if err != nil {
		return fmt.Errorf("insert failed: %v", err)
	}

	return nil
}

func (P *WechatDataProvider) WeChatDelSessionBookMask(markId string) error {
	querySql := fmt.Sprintf("select COUNT(*) from bookMark where markId='%s';", markId)
	var count int

	err := P.userData.QueryRow(querySql).Scan(&count)
	if err != nil {
		log.Println("select DB markId count failed:", err)
		return err
	}

	if count > 0 {
		_, err = P.userData.Exec("DELETE from bookMark where markId=?", markId)
		if err != nil {
			return fmt.Errorf("delete failed: %v", err)
		}
	} else {
		log.Printf("markId %s not exits\n", markId)
	}

	return nil
}

func (P *WechatDataProvider) WeChatGetSessionBookMaskList(userName string) (*WeChatBookMarkList, error) {
	markList := &WeChatBookMarkList{}
	markList.Marks = make([]WeChatBookMark, 0)
	markList.Total = 0

	querySql := fmt.Sprintf("select markId, tag, info from bookMark where userName='%s';", userName)
	log.Println("querySql:", querySql)

	rows, err := P.userData.Query(querySql)
	if err != nil {
		log.Printf("%s failed %v\n", querySql, err)
		return markList, err
	}
	defer rows.Close()

	var markId, tag, info string
	for rows.Next() {
		err = rows.Scan(&markId, &tag, &info)
		if err != nil {
			log.Println("rows.Scan failed", err)
			return markList, err
		}

		markList.Marks = append(markList.Marks, WeChatBookMark{MarkId: markId, Tag: tag, Info: info})
		markList.Total += 1
	}

	if err := rows.Err(); err != nil {
		log.Println("rows.Scan failed", err)
		return markList, err
	}

	return markList, nil
}

func (P *WechatDataProvider) WeChatExportDataByUserName(userName, exportPath string) error {

	err := P.WeChatExportDBByUserName(userName, exportPath)
	if err != nil {
		log.Println("WeChatExportDBByUserName:", err)
		return err
	}

	err = P.WeChatExportFileByUserName(userName, exportPath)
	if err != nil {
		log.Println("WeChatExportFileByUserName:", err)
		return err
	}
	log.Println("WeChatExportDataByUserName done")
	return nil
}

func (P *WechatDataProvider) WeChatExportDBByUserName(userName, exportPath string) error {
	msgPath := fmt.Sprintf("%s\\User\\%s\\Msg", exportPath, P.SelfInfo.UserName)
	multiPath := fmt.Sprintf("%s\\Multi", msgPath)
	if _, err := os.Stat(multiPath); err != nil {
		if err := os.MkdirAll(multiPath, 0644); err != nil {
			log.Printf("MkdirAll %s failed: %v\n", multiPath, err)
			return err
		}
	}

	err := P.weChatExportMicroMsgDBByUserName(userName, msgPath)
	if err != nil {
		log.Println("weChatExportMicroMsgDBByUserName failed:", err)
		return err
	}

	err = P.weChatExportMsgDBByUserName(userName, multiPath)
	if err != nil {
		log.Println("weChatExportMsgDBByUserName failed:", err)
		return err
	}

	err = P.weChatExportUserDataDBByUserName(userName, msgPath)
	if err != nil {
		log.Println("weChatExportUserDataDBByUserName failed:", err)
		return err
	}

	err = P.weChatExportOpenIMContactDBByUserName(userName, msgPath)
	if err != nil {
		log.Println("weChatExportOpenIMContactDBByUserName failed:", err)
		return err
	}

	return nil
}

func (P *WechatDataProvider) weChatExportMicroMsgDBByUserName(userName, exportPath string) error {
	exMicroMsgDBPath := exportPath + "\\" + MicroMsgDB
	if _, err := os.Stat(exMicroMsgDBPath); err == nil {
		log.Println("exist", exMicroMsgDBPath)
		return errors.New("exist " + exMicroMsgDBPath)
	}

	exMicroMsgDB, err := sql.Open("sqlite3", exMicroMsgDBPath)
	if err != nil {
		log.Println("db open", err)
		return err
	}
	defer exMicroMsgDB.Close()

	tables := []string{"Contact", "ContactHeadImgUrl", "Session"}
	IsGroup := false
	if strings.HasSuffix(userName, "@chatroom") {
		IsGroup = true
		tables = append(tables, "ChatRoom", "ChatRoomInfo")
	}

	err = wechatCopyDBTables(exMicroMsgDB, P.microMsg, tables)
	if err != nil {
		log.Println("wechatCopyDBTables:", err)
		return err
	}

	copyContactData := func(users []string) error {
		columns := "UserName, Alias, EncryptUserName, DelFlag, Type, VerifyFlag, Reserved1, Reserved2, Reserved3, Reserved4, Remark, NickName, LabelIDList, DomainList, ChatRoomType, PYInitial, QuanPin, RemarkPYInitial, RemarkQuanPin, BigHeadImgUrl, SmallHeadImgUrl, HeadImgMd5, ChatRoomNotify, Reserved5, Reserved6, Reserved7, ExtraBuf, Reserved8, Reserved9, Reserved10, Reserved11"
		err = wechatCopyTableData(exMicroMsgDB, P.microMsg, "Contact", columns, "UserName", users)
		if err != nil {
			log.Println("wechatCopyTableData Contact:", err)
			return err
		}

		columns = "usrName, smallHeadImgUrl, bigHeadImgUrl, headImgMd5, reverse0, reverse1"
		err = wechatCopyTableData(exMicroMsgDB, P.microMsg, "ContactHeadImgUrl", columns, "usrName", users)
		if err != nil {
			log.Println("wechatCopyTableData ContactHeadImgUrl:", err)
			return err
		}

		return nil
	}

	err = copyContactData([]string{userName, P.SelfInfo.UserName})
	if err != nil {
		log.Println("copyContactData:", err)
		return err
	}

	columns := "strUsrName, nOrder, nUnReadCount, parentRef, Reserved0, Reserved1, strNickName, nStatus, nIsSend, strContent, nMsgType, nMsgLocalID, nMsgStatus, nTime, editContent, othersAtMe, Reserved2, Reserved3, Reserved4, Reserved5, bytesXml"
	err = wechatCopyTableData(exMicroMsgDB, P.microMsg, "Session", columns, "strUsrName", []string{userName})
	if err != nil {
		log.Println("wechatCopyTableData Session:", err)
		return err
	}

	if !IsGroup {
		return nil
	}

	uList, err := P.WeChatGetChatRoomUserList(userName)
	if err != nil {
		log.Println("WeChatGetChatRoomUserList failed:", err)
		return err
	}

	userNames := make([]string, 0, 100)
	for i := range uList.Users {
		userNames = append(userNames, uList.Users[i].UserName)
		if len(userNames) >= 100 || i == len(uList.Users)-1 {
			err = copyContactData(userNames)
			if err != nil {
				log.Println("copyContactData:", err)
			}
			userNames = userNames[:0]
		}
	}

	columns = "ChatRoomName, UserNameList, DisplayNameList, ChatRoomFlag, Owner, IsShowName, SelfDisplayName, Reserved1, Reserved2, Reserved3, Reserved4, Reserved5, Reserved6, RoomData, Reserved7, Reserved8"
	err = wechatCopyTableData(exMicroMsgDB, P.microMsg, "ChatRoom", columns, "ChatRoomName", []string{userName})
	if err != nil {
		log.Println("wechatCopyTableData ChatRoom:", err)
		return err
	}

	columns = "ChatRoomName, Announcement, InfoVersion, AnnouncementEditor, AnnouncementPublishTime, ChatRoomStatus, Reserved1, Reserved2, Reserved3, Reserved4, Reserved5, Reserved6, Reserved7, Reserved8"
	err = wechatCopyTableData(exMicroMsgDB, P.microMsg, "ChatRoomInfo", columns, "ChatRoomName", []string{userName})
	if err != nil {
		log.Println("wechatCopyTableData ChatRoom:", err)
		return err
	}

	return nil
}

func (P *WechatDataProvider) weChatExportMsgDBByUserName(userName, exportPath string) error {
	exMsgDBPath := exportPath + "\\" + "MSG.db"
	if _, err := os.Stat(exMsgDBPath); err == nil {
		log.Println("exist", exMsgDBPath)
		return errors.New("exist " + exMsgDBPath)
	}

	exMsgDB, err := sql.Open("sqlite3", exMsgDBPath)
	if err != nil {
		log.Println("db open", err)
		return err
	}
	defer exMsgDB.Close()

	if len(P.msgDBs) == 0 {
		return fmt.Errorf("P.msgDBs len = 0")
	}

	tables := []string{"MSG", "Name2ID"}
	err = wechatCopyDBTables(exMsgDB, P.msgDBs[0].db, tables)
	if err != nil {
		log.Println("wechatCopyDBTables:", err)
		return err
	}

	columns := "TalkerId, MsgSvrID, Type, SubType, IsSender, CreateTime, Sequence, StatusEx, FlagEx, Status, MsgServerSeq, MsgSequence, StrTalker, StrContent, DisplayContent, Reserved0, Reserved1, Reserved2, Reserved3, Reserved4, Reserved5, Reserved6, CompressContent, BytesExtra, BytesTrans"
	for _, msgDB := range P.msgDBs {
		err = wechatCopyTableData(exMsgDB, msgDB.db, "MSG", columns, "StrTalker", []string{userName})
		if err != nil {
			log.Println("wechatCopyTableData MSG:", err)
			return err
		}
	}

	columns = "UsrName"
	for _, msgDB := range P.msgDBs {
		err = wechatCopyTableData(exMsgDB, msgDB.db, "Name2ID", columns, "UsrName", []string{userName})
		if err != nil {
			continue
		}
		// log.Println("Name2ID:", userName)
	}

	return nil
}

func (P *WechatDataProvider) weChatExportUserDataDBByUserName(userName, exportPath string) error {
	exUserDataDBPath := exportPath + "\\" + UserDataDB
	if _, err := os.Stat(exUserDataDBPath); err == nil {
		log.Println("exist", exUserDataDBPath)
		return errors.New("exist " + exUserDataDBPath)
	}

	exUserDataDB, err := sql.Open("sqlite3", exUserDataDBPath)
	if err != nil {
		log.Println("db open", err)
		return err
	}
	defer exUserDataDB.Close()

	tables := []string{"lastTime", "bookMark"}
	err = wechatCopyDBTables(exUserDataDB, P.userData, tables)
	if err != nil {
		log.Println("wechatCopyDBTables:", err)
		return err
	}

	columns := "localId,userName,timestamp,messageId,Reserved0,Reserved1,Reserved2,Reserved3"
	err = wechatCopyTableData(exUserDataDB, P.userData, "lastTime", columns, "userName", []string{userName})
	if err != nil {
		log.Println("wechatCopyTableData lastTime:", err)
		return err
	}

	columns = "localId, userName, markId, tag, info, Reserved0, Reserved1, Reserved2, Reserved3"
	err = wechatCopyTableData(exUserDataDB, P.userData, "bookMark", columns, "userName", []string{userName})
	if err != nil {
		log.Println("wechatCopyTableData bookMark:", err)
		return err
	}

	return nil
}

func (P *WechatDataProvider) weChatExportOpenIMContactDBByUserName(userName, exportPath string) error {
	hasOpenIM := false
	IsGroup := false
	if strings.HasSuffix(userName, "@openim") {
		hasOpenIM = true
	}

	userNames := make([]string, 0)
	if strings.HasSuffix(userName, "@chatroom") {
		IsGroup = true
		uList, err := P.WeChatGetChatRoomUserList(userName)
		if err != nil {
			log.Println("WeChatGetChatRoomUserList failed:", err)
			return err
		}
		for i := range uList.Users {
			if strings.HasSuffix(uList.Users[i].UserName, "@openim") {
				userNames = append(userNames, uList.Users[i].UserName)
				hasOpenIM = true
			}
		}
	}

	if !hasOpenIM || P.openIMContact == nil {
		log.Println("not Open Im")
		return nil
	}

	exOpenIMContactDBPath := exportPath + "\\" + OpenIMContactDB
	if _, err := os.Stat(exOpenIMContactDBPath); err == nil {
		log.Println("exist", exOpenIMContactDBPath)
		return errors.New("exist " + exOpenIMContactDBPath)
	}

	exOpenIMContactDB, err := sql.Open("sqlite3", exOpenIMContactDBPath)
	if err != nil {
		log.Println("db open", err)
		return err
	}
	defer exOpenIMContactDB.Close()

	tables := []string{"OpenIMContact"}
	err = wechatCopyDBTables(exOpenIMContactDB, P.openIMContact, tables)
	if err != nil {
		log.Println("wechatCopyDBTables:", err)
		return err
	}

	copyContactData := func(users []string) error {
		columns := "UserName, NickName, Type, Remark, BigHeadImgUrl, SmallHeadImgUrl, Source, NickNamePYInit, NickNameQuanPin, RemarkPYInit, RemarkQuanPin, CustomInfoDetail, CustomInfoDetailVisible, AntiSpamTicket, AppId, Sex, DescWordingId, Reserved1, Reserved2, Reserved3, Reserved4, Reserved5, Reserved6, Reserved7, Reserved8, ExtraBuf"
		err = wechatCopyTableData(exOpenIMContactDB, P.openIMContact, "OpenIMContact", columns, "UserName", users)
		if err != nil {
			log.Println("wechatCopyTableData OpenIMContact:", err)
			return err
		}

		return nil
	}

	if !IsGroup {
		return copyContactData([]string{userName})
	}

	chunkSize := 100
	for i := 0; i < len(userNames); i += chunkSize {
		end := i + chunkSize
		if end > len(userNames) {
			end = len(userNames)
		}
		err = copyContactData(userNames[i:end])
		if err != nil {
			return err
		}
	}
	return nil
}

func wechatCopyDBTables(dts, src *sql.DB, tables []string) error {
	for _, tab := range tables {
		querySql := fmt.Sprintf("SELECT sql FROM sqlite_master WHERE tbl_name='%s';", tab)
		// log.Println("querySql:", querySql)
		rows, err := src.Query(querySql)
		if err != nil {
			rows.Close()
			log.Println("src.Query", err)
			continue
		}

		var createStatements []string
		for rows.Next() {
			var sql string
			if err := rows.Scan(&sql); err != nil {
				log.Println(err)
				continue
			}
			if sql != "" {
				createStatements = append(createStatements, sql)
			}
		}
		rows.Close()
		// log.Println("createStatements:", createStatements)
		tx, err := dts.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %v", err)
		}

		for _, stmt := range createStatements {
			if _, err := tx.Exec(stmt); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to execute statement: %s, error: %v", stmt, err)
			}
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %v", err)
		}
	}

	return nil
}

func wechatCopyTableData(dts, src *sql.DB, tableName, columns, conditionField string, conditionValue []string) error {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = '%s'", columns, tableName, conditionField, conditionValue[0])
	if len(conditionValue) > 1 {
		query = fmt.Sprintf("SELECT %s FROM %s WHERE %s IN ('%s')", columns, tableName, conditionField, strings.Join(conditionValue, "','"))
	}
	// log.Println("query:", query)
	rows, err := src.Query(query)
	if err != nil {
		return fmt.Errorf("query src failed: %v", err)
	}
	defer rows.Close()

	tx, err := dts.Begin()
	if err != nil {
		return fmt.Errorf("dts.Begin failed: %v", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	columnList := strings.Split(columns, ",")
	placeholders := strings.Repeat("?, ", len(columnList))
	placeholders = placeholders[:len(placeholders)-2]
	insertQuery := fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) VALUES (%s)", tableName, columns, placeholders)
	// log.Println("wechatCopyTableData:", insertQuery)
	stmt, err := tx.Prepare(insertQuery)
	if err != nil {
		return fmt.Errorf("prepare insertquery: %v", err)
	}
	defer stmt.Close()

	for rows.Next() {
		values := make([]interface{}, len(columnList))
		valuePtrs := make([]interface{}, len(columnList))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("scan rows failed: %v", err)
		}

		if _, err := stmt.Exec(values...); err != nil {
			return fmt.Errorf("insert data failed: %v", err)
		}
	}

	return nil
}

func (P *WechatDataProvider) WeChatExportFileByUserName(userName, exportPath string) error {

	topDir := filepath.Dir(P.resPath)
	topDir = filepath.Dir(topDir)
	pageSize := 600
	_time := time.Now().Unix()
	taskChan := make(chan [2]string, 100)
	var wg sync.WaitGroup

	taskSend := func(topDir, path, exportPath string, taskChan chan [2]string) {
		if path == "" {
			return
		}
		srcFile := topDir + path
		if _, err := os.Stat(srcFile); err != nil {
			// log.Println("no exist:", srcFile)
			return
		}

		dstFile := exportPath + path
		dstDir := filepath.Dir(dstFile)
		if _, err := os.Stat(dstDir); err != nil {
			os.MkdirAll(dstDir, os.ModePerm)
		}

		task := [2]string{srcFile, dstFile}
		taskChan <- task
	}

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskChan {
				// log.Println("copy: ", task[0], task[1])
				utils.CopyFile(task[0], task[1])
			}
		}()
	}

	for {
		mlist, err := P.WeChatGetMessageListByTime(userName, _time, pageSize, Message_Search_Forward)
		if err != nil {
			return err
		}

		paths := make([]string, 0)
		for _, m := range mlist.Rows {
			switch m.Type {
			case Wechat_Message_Type_Picture:
				paths = append(paths, m.ThumbPath, m.ImagePath)
			case Wechat_Message_Type_Voice:
				paths = append(paths, m.VoicePath)
			case Wechat_Message_Type_Visit_Card:
				paths = append(paths, m.VisitInfo.LocalHeadImgUrl)
			case Wechat_Message_Type_Video:
				paths = append(paths, m.ThumbPath, m.VideoPath)
			case Wechat_Message_Type_Location:
				paths = append(paths, m.LocationInfo.ThumbPath)
			case Wechat_Message_Type_Misc:
				switch m.SubType {
				case Wechat_Misc_Message_Music:
					paths = append(paths, m.MusicInfo.ThumbPath)
				case Wechat_Misc_Message_ThirdVideo:
					paths = append(paths, m.ThumbPath)
				case Wechat_Misc_Message_CardLink:
					paths = append(paths, m.ThumbPath)
				case Wechat_Misc_Message_File:
					paths = append(paths, m.FileInfo.FilePath)
				case Wechat_Misc_Message_Applet:
					paths = append(paths, m.ThumbPath)
				case Wechat_Misc_Message_Applet2:
					paths = append(paths, m.ThumbPath)
				case Wechat_Misc_Message_Channels:
					paths = append(paths, m.ChannelsInfo.ThumbPath)
				case Wechat_Misc_Message_Live:
					paths = append(paths, m.ChannelsInfo.ThumbPath)
				case Wechat_Misc_Message_Game:
					paths = append(paths, m.ThumbPath)
				case Wechat_Misc_Message_TingListen:
					paths = append(paths, m.MusicInfo.ThumbPath)
				}
			}
		}

		for _, path := range paths {
			taskSend(topDir, path, exportPath, taskChan)
		}

		if mlist.Total < pageSize {
			break
		}
		_time = mlist.Rows[mlist.Total-1].CreateTime - 1
	}
	log.Println("message file done")
	//copy HeadImage
	taskSend(topDir, P.SelfInfo.LocalHeadImgUrl, exportPath, taskChan)
	info, err := P.WechatGetUserInfoByNameOnCache(userName)
	if err == nil {
		taskSend(topDir, info.LocalHeadImgUrl, exportPath, taskChan)
	}

	if strings.HasSuffix(userName, "@chatroom") {
		uList, err := P.WeChatGetChatRoomUserList(userName)
		if err == nil {
			for _, user := range uList.Users {
				taskSend(topDir, user.LocalHeadImgUrl, exportPath, taskChan)
			}
		}
	}
	log.Println("HeadImage file done")
	close(taskChan)
	wg.Wait()

	return nil
}
