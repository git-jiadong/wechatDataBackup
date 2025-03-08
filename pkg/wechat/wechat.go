package wechat

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/git-jiadong/go-lame"
	"github.com/git-jiadong/go-silk"
	_ "github.com/mattn/go-sqlite3"
	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/windows"
)

type WeChatInfo struct {
	ProcessID   uint32
	FilePath    string
	AcountName  string
	Version     string
	Is64Bits    bool
	DllBaseAddr uintptr
	DllBaseSize uint32
	DBKey       string
}

type WeChatInfoList struct {
	Info  []WeChatInfo `json:"Info"`
	Total int          `json:"Total"`
}

type wechatMediaMSG struct {
	Key      string
	MsgSvrID int
	Buf      []byte
}

type wechatHeadImgMSG struct {
	userName string
	Buf      []byte
}

func GetWeChatAllInfo() *WeChatInfoList {
	list := GetWeChatInfo()

	for i := range list.Info {
		list.Info[i].DBKey = GetWeChatKey(&list.Info[i])
	}

	return list
}

func ExportWeChatAllData(info WeChatInfo, expPath string, progress chan<- string) {
	defer close(progress)
	fileInfo, err := os.Stat(info.FilePath)
	if err != nil || !fileInfo.IsDir() {
		progress <- fmt.Sprintf("{\"status\":\"error\", \"result\":\"%s error\"}", info.FilePath)
		return
	}
	if !exportWeChatDateBase(info, expPath, progress) {
		return
	}

	exportWeChatBat(info, expPath, progress)
	exportWeChatVideoAndFile(info, expPath, progress)
	exportWeChatVoice(info, expPath, progress)
	exportWeChatHeadImage(info, expPath, progress)
}

func exportWeChatHeadImage(info WeChatInfo, expPath string, progress chan<- string) {
	progress <- "{\"status\":\"processing\", \"result\":\"export WeChat Head Image\", \"progress\": 81}"

	headImgPath := fmt.Sprintf("%s\\FileStorage\\HeadImage", expPath)
	if _, err := os.Stat(headImgPath); err != nil {
		if err := os.MkdirAll(headImgPath, 0644); err != nil {
			log.Printf("MkdirAll %s failed: %v\n", headImgPath, err)
			progress <- fmt.Sprintf("{\"status\":\"error\", \"result\":\"%v error\"}", err)
			return
		}
	}

	handleNumber := int64(0)
	fileNumber := int64(0)

	var wg sync.WaitGroup
	var reportWg sync.WaitGroup
	quitChan := make(chan struct{})
	MSGChan := make(chan wechatHeadImgMSG, 100)
	go func() {
		for {
			miscDBPath := fmt.Sprintf("%s\\Msg\\Misc.db", expPath)
			_, err := os.Stat(miscDBPath)
			if err != nil {
				log.Println("no exist:", miscDBPath)
				break
			}

			db, err := sql.Open("sqlite3", miscDBPath)
			if err != nil {
				log.Printf("open %s failed: %v\n", miscDBPath, err)
				break
			}
			defer db.Close()

			err = db.QueryRow("select count(*) from ContactHeadImg1;").Scan(&fileNumber)
			if err != nil {
				log.Println("select count(*) failed", err)
				break
			}
			log.Println("ContactHeadImg1 fileNumber", fileNumber)
			rows, err := db.Query("select ifnull(usrName,'') as usrName, ifnull(smallHeadBuf,'') as smallHeadBuf from ContactHeadImg1;")
			if err != nil {
				log.Printf("Query failed: %v\n", err)
				break
			}

			msg := wechatHeadImgMSG{}
			for rows.Next() {
				err := rows.Scan(&msg.userName, &msg.Buf)
				if err != nil {
					log.Println("Scan failed: ", err)
					break
				}

				MSGChan <- msg
			}
			break
		}
		close(MSGChan)
	}()

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for msg := range MSGChan {
				imgPath := fmt.Sprintf("%s\\%s.headimg", headImgPath, msg.userName)
				for {
					// log.Println("imgPath:", imgPath, len(msg.Buf))
					_, err := os.Stat(imgPath)
					if err == nil {
						break
					}
					if len(msg.userName) == 0 || len(msg.Buf) == 0 {
						break
					}
					err = os.WriteFile(imgPath, msg.Buf[:], 0666)
					if err != nil {
						log.Println("WriteFile:", imgPath, err)
					}
					break
				}
				atomic.AddInt64(&handleNumber, 1)
			}
		}()
	}

	reportWg.Add(1)
	go func() {
		defer reportWg.Done()
		for {
			select {
			case <-quitChan:
				log.Println("WeChat Head Image report progress end")
				return
			default:
				if fileNumber != 0 {
					filePercent := float64(handleNumber) / float64(fileNumber)
					totalPercent := 81 + (filePercent * (100 - 81))
					totalPercentStr := fmt.Sprintf("{\"status\":\"processing\", \"result\":\"export WeChat Head Image doing\", \"progress\": %d}", int(totalPercent))
					progress <- totalPercentStr
				}
				time.Sleep(time.Second)
			}
		}
	}()

	wg.Wait()
	close(quitChan)
	reportWg.Wait()
	progress <- "{\"status\":\"processing\", \"result\":\"export WeChat Head Image end\", \"progress\": 100}"
}

func exportWeChatVoice(info WeChatInfo, expPath string, progress chan<- string) {
	progress <- "{\"status\":\"processing\", \"result\":\"export WeChat voice start\", \"progress\": 61}"

	voicePath := fmt.Sprintf("%s\\FileStorage\\Voice", expPath)
	if _, err := os.Stat(voicePath); err != nil {
		if err := os.MkdirAll(voicePath, 0644); err != nil {
			log.Printf("MkdirAll %s failed: %v\n", voicePath, err)
			progress <- fmt.Sprintf("{\"status\":\"error\", \"result\":\"%v error\"}", err)
			return
		}
	}

	handleNumber := int64(0)
	fileNumber := int64(0)
	index := 0
	for {
		mediaMSGDB := fmt.Sprintf("%s\\Msg\\Multi\\MediaMSG%d.db", expPath, index)
		_, err := os.Stat(mediaMSGDB)
		if err != nil {
			break
		}
		index += 1
		fileNumber += 1
	}

	var wg sync.WaitGroup
	var reportWg sync.WaitGroup
	quitChan := make(chan struct{})
	index = -1
	MSGChan := make(chan wechatMediaMSG, 100)
	go func() {
		for {
			index += 1
			mediaMSGDB := fmt.Sprintf("%s\\Msg\\Multi\\MediaMSG%d.db", expPath, index)
			_, err := os.Stat(mediaMSGDB)
			if err != nil {
				break
			}

			db, err := sql.Open("sqlite3", mediaMSGDB)
			if err != nil {
				log.Printf("open %s failed: %v\n", mediaMSGDB, err)
				continue
			}
			defer db.Close()

			rows, err := db.Query("select Key, Reserved0, Buf from Media;")
			if err != nil {
				log.Printf("Query failed: %v\n", err)
				continue
			}

			msg := wechatMediaMSG{}
			for rows.Next() {
				err := rows.Scan(&msg.Key, &msg.MsgSvrID, &msg.Buf)
				if err != nil {
					log.Println("Scan failed: ", err)
					break
				}

				MSGChan <- msg
			}
			atomic.AddInt64(&handleNumber, 1)
		}
		close(MSGChan)
	}()

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for msg := range MSGChan {
				mp3Path := fmt.Sprintf("%s\\%d.mp3", voicePath, msg.MsgSvrID)
				_, err := os.Stat(mp3Path)
				if err == nil {
					continue
				}

				err = silkToMp3(msg.Buf[:], mp3Path)
				if err != nil {
					log.Printf("silkToMp3 %s failed: %v\n", mp3Path, err)
				}
			}
		}()
	}

	reportWg.Add(1)
	go func() {
		defer reportWg.Done()
		for {
			select {
			case <-quitChan:
				log.Println("WeChat voice report progress end")
				return
			default:
				filePercent := float64(handleNumber) / float64(fileNumber)
				totalPercent := 61 + (filePercent * (80 - 61))
				totalPercentStr := fmt.Sprintf("{\"status\":\"processing\", \"result\":\"export WeChat voice doing\", \"progress\": %d}", int(totalPercent))
				progress <- totalPercentStr
				time.Sleep(time.Second)
			}
		}
	}()

	wg.Wait()
	close(quitChan)
	reportWg.Wait()
	progress <- "{\"status\":\"processing\", \"result\":\"export WeChat voice end\", \"progress\": 80}"
}

func exportWeChatVideoAndFile(info WeChatInfo, expPath string, progress chan<- string) {
	progress <- "{\"status\":\"processing\", \"result\":\"export WeChat Video and File start\", , \"progress\": 41}"
	videoRootPath := info.FilePath + "\\FileStorage\\Video"
	fileRootPath := info.FilePath + "\\FileStorage\\File"
	cacheRootPath := info.FilePath + "\\FileStorage\\Cache"
	rootPaths := []string{videoRootPath, fileRootPath, cacheRootPath}

	handleNumber := int64(0)
	fileNumber := int64(0)
	for _, path := range rootPaths {
		fileNumber += getPathFileNumber(path, "")
	}
	log.Println("VideoAndFile ", fileNumber)

	var wg sync.WaitGroup
	var reportWg sync.WaitGroup
	quitChan := make(chan struct{})
	taskChan := make(chan [2]string, 100)
	go func() {
		for _, rootPath := range rootPaths {
			log.Println(rootPath)
			err := filepath.Walk(rootPath, func(path string, finfo os.FileInfo, err error) error {
				if err != nil {
					log.Printf("filepath.Walk：%v\n", err)
					return err
				}

				if !finfo.IsDir() {
					expFile := expPath + path[len(info.FilePath):]
					_, err := os.Stat(filepath.Dir(expFile))
					if err != nil {
						os.MkdirAll(filepath.Dir(expFile), 0644)
					}

					task := [2]string{path, expFile}
					taskChan <- task
					return nil
				}

				return nil
			})
			if err != nil {
				log.Println("filepath.Walk:", err)
				progress <- fmt.Sprintf("{\"status\":\"error\", \"result\":\"%v\"}", err)
			}
		}
		close(taskChan)
	}()

	for i := 1; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskChan {
				_, err := os.Stat(task[1])
				if err == nil {
					atomic.AddInt64(&handleNumber, 1)
					continue
				}
				_, err = copyFile(task[0], task[1])
				if err != nil {
					log.Println("DecryptDat:", err)
					progress <- fmt.Sprintf("{\"status\":\"error\", \"result\":\"copyFile %v\"}", err)
				}
				atomic.AddInt64(&handleNumber, 1)
			}
		}()
	}
	reportWg.Add(1)
	go func() {
		defer reportWg.Done()
		for {
			select {
			case <-quitChan:
				log.Println("WeChat Video and File report progress end")
				return
			default:
				filePercent := float64(handleNumber) / float64(fileNumber)
				totalPercent := 41 + (filePercent * (60 - 41))
				totalPercentStr := fmt.Sprintf("{\"status\":\"processing\", \"result\":\"export WeChat Video and File doing\", \"progress\": %d}", int(totalPercent))
				progress <- totalPercentStr
				time.Sleep(time.Second)
			}
		}
	}()
	wg.Wait()
	close(quitChan)
	reportWg.Wait()
	progress <- "{\"status\":\"processing\", \"result\":\"export WeChat Video and File end\", \"progress\": 60}"
}

func exportWeChatBat(info WeChatInfo, expPath string, progress chan<- string) {
	progress <- "{\"status\":\"processing\", \"result\":\"export WeChat Dat start\", \"progress\": 21}"
	datRootPath := info.FilePath + "\\FileStorage\\MsgAttach"
	fileInfo, err := os.Stat(datRootPath)
	if err != nil || !fileInfo.IsDir() {
		progress <- fmt.Sprintf("{\"status\":\"error\", \"result\":\"%s error\"}", datRootPath)
		return
	}

	handleNumber := int64(0)
	fileNumber := getPathFileNumber(datRootPath, ".dat")
	var wg sync.WaitGroup
	var reportWg sync.WaitGroup
	quitChan := make(chan struct{})
	taskChan := make(chan [2]string, 100)
	go func() {
		err = filepath.Walk(datRootPath, func(path string, finfo os.FileInfo, err error) error {
			if err != nil {
				log.Printf("filepath.Walk：%v\n", err)
				return err
			}

			if !finfo.IsDir() && strings.HasSuffix(path, ".dat") {
				expFile := expPath + path[len(info.FilePath):]
				_, err := os.Stat(filepath.Dir(expFile))
				if err != nil {
					os.MkdirAll(filepath.Dir(expFile), 0644)
				}

				task := [2]string{path, expFile}
				taskChan <- task
				return nil
			}

			return nil
		})

		if err != nil {
			log.Println("filepath.Walk:", err)
			progress <- fmt.Sprintf("{\"status\":\"error\", \"result\":\"%v\"}", err)
		}
		close(taskChan)
	}()

	for i := 1; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskChan {
				_, err = os.Stat(task[1])
				if err == nil {
					atomic.AddInt64(&handleNumber, 1)
					continue
				}
				err = DecryptDat(task[0], task[1])
				if err != nil {
					log.Println("DecryptDat:", err)
					progress <- fmt.Sprintf("{\"status\":\"error\", \"result\":\"DecryptDat %v\"}", err)
				}
				atomic.AddInt64(&handleNumber, 1)
			}
		}()
	}
	reportWg.Add(1)
	go func() {
		defer reportWg.Done()
		for {
			select {
			case <-quitChan:
				log.Println("WeChat Dat report progress end")
				return
			default:
				filePercent := float64(handleNumber) / float64(fileNumber)
				totalPercent := 21 + (filePercent * (40 - 21))
				totalPercentStr := fmt.Sprintf("{\"status\":\"processing\", \"result\":\"export WeChat Dat doing\", \"progress\": %d}", int(totalPercent))
				progress <- totalPercentStr
				time.Sleep(time.Second)
			}
		}
	}()
	wg.Wait()
	close(quitChan)
	reportWg.Wait()
	progress <- "{\"status\":\"processing\", \"result\":\"export WeChat Dat end\", \"progress\": 40}"
}

func exportWeChatDateBase(info WeChatInfo, expPath string, progress chan<- string) bool {

	progress <- "{\"status\":\"processing\", \"result\":\"export WeChat DateBase start\", \"progress\": 1}"

	dbKey, err := hex.DecodeString(info.DBKey)
	if err != nil {
		log.Println("DecodeString:", err)
		progress <- fmt.Sprintf("{\"status\":\"error\", \"result\":\"%v\"}", err)
		return false
	}

	handleNumber := int64(0)
	fileNumber := getPathFileNumber(info.FilePath+"\\Msg", ".db")
	var wg sync.WaitGroup
	var reportWg sync.WaitGroup
	quitChan := make(chan struct{})
	taskChan := make(chan [2]string, 20)
	go func() {
		err = filepath.Walk(info.FilePath+"\\Msg", func(path string, finfo os.FileInfo, err error) error {
			if err != nil {
				log.Printf("filepath.Walk：%v\n", err)
				return err
			}
			if !finfo.IsDir() && strings.HasSuffix(path, ".db") {
				expFile := expPath + path[len(info.FilePath):]
				_, err := os.Stat(filepath.Dir(expFile))
				if err != nil {
					os.MkdirAll(filepath.Dir(expFile), 0644)
				}

				task := [2]string{path, expFile}
				taskChan <- task
			}

			return nil
		})
		if err != nil {
			log.Println("filepath.Walk:", err)
			progress <- fmt.Sprintf("{\"status\":\"error\", \"result\":\"%v\"}", err)
		}
		close(taskChan)
	}()

	for i := 1; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskChan {
				if filepath.Base(task[0]) == "xInfo.db" {
					copyFile(task[0], task[1])
				} else {
					err = DecryptDataBase(task[0], dbKey, task[1])
					if err != nil {
						log.Println("DecryptDataBase:", err)
						progress <- fmt.Sprintf("{\"status\":\"error\", \"result\":\"%s %v\"}", task[0], err)
					}
				}
				atomic.AddInt64(&handleNumber, 1)
			}
		}()
	}

	reportWg.Add(1)
	go func() {
		defer reportWg.Done()
		for {
			select {
			case <-quitChan:
				log.Println("WeChat DateBase report progress end")
				return
			default:
				filePercent := float64(handleNumber) / float64(fileNumber)
				totalPercent := 1 + (filePercent * (20 - 1))
				totalPercentStr := fmt.Sprintf("{\"status\":\"processing\", \"result\":\"export WeChat DateBase doing\", \"progress\": %d}", int(totalPercent))
				progress <- totalPercentStr
				time.Sleep(time.Second)
			}
		}
	}()
	wg.Wait()
	close(quitChan)
	reportWg.Wait()
	progress <- "{\"status\":\"processing\", \"result\":\"export WeChat DateBase end\", \"progress\": 20}"
	return true
}

func GetWeChatInfo() (list *WeChatInfoList) {
	list = &WeChatInfoList{}
	list.Info = make([]WeChatInfo, 0)
	list.Total = 0

	processes, err := process.Processes()
	if err != nil {
		log.Println("Error getting processes:", err)
		return
	}

	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			continue
		}
		info := WeChatInfo{}
		if name == "WeChat.exe" {
			info.ProcessID = uint32(p.Pid)
			info.Is64Bits, _ = Is64BitProcess(info.ProcessID)
			log.Println("ProcessID", info.ProcessID)
			files, err := p.OpenFiles()
			if err != nil {
				log.Println("OpenFiles failed")
				continue
			}

			for _, f := range files {
				if strings.HasSuffix(f.Path, "\\Media.db") {
					// fmt.Printf("opened %s\n", f.Path[4:])
					filePath := f.Path[4:]
					parts := strings.Split(filePath, string(filepath.Separator))
					if len(parts) < 4 {
						log.Println("Error filePath " + filePath)
						break
					}
					info.FilePath = strings.Join(parts[:len(parts)-2], string(filepath.Separator))
					info.AcountName = strings.Join(parts[len(parts)-3:len(parts)-2], string(filepath.Separator))
				}

			}

			if len(info.FilePath) == 0 {
				log.Println("wechat not log in")
				continue
			}

			hModuleSnap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPMODULE|windows.TH32CS_SNAPMODULE32, uint32(p.Pid))
			if err != nil {
				log.Println("CreateToolhelp32Snapshot failed", err)
				continue
			}
			defer windows.CloseHandle(hModuleSnap)

			var me32 windows.ModuleEntry32
			me32.Size = uint32(windows.SizeofModuleEntry32)

			err = windows.Module32First(hModuleSnap, &me32)
			if err != nil {
				log.Println("Module32First failed", err)
				continue
			}

			for ; err == nil; err = windows.Module32Next(hModuleSnap, &me32) {
				if windows.UTF16ToString(me32.Module[:]) == "WeChatWin.dll" {
					// fmt.Printf("MODULE NAME: %s\n", windows.UTF16ToString(me32.Module[:]))
					// fmt.Printf("executable NAME: %s\n", windows.UTF16ToString(me32.ExePath[:]))
					// fmt.Printf("base address: 0x%08X\n", me32.ModBaseAddr)
					// fmt.Printf("base ModBaseSize: %d\n", me32.ModBaseSize)
					info.DllBaseAddr = me32.ModBaseAddr
					info.DllBaseSize = me32.ModBaseSize

					var zero windows.Handle
					driverPath := windows.UTF16ToString(me32.ExePath[:])
					infoSize, err := windows.GetFileVersionInfoSize(driverPath, &zero)
					if err != nil {
						log.Println("GetFileVersionInfoSize failed", err)
						break
					}
					versionInfo := make([]byte, infoSize)
					if err = windows.GetFileVersionInfo(driverPath, 0, infoSize, unsafe.Pointer(&versionInfo[0])); err != nil {
						log.Println("GetFileVersionInfo failed", err)
						break
					}
					var fixedInfo *windows.VS_FIXEDFILEINFO
					fixedInfoLen := uint32(unsafe.Sizeof(*fixedInfo))
					err = windows.VerQueryValue(unsafe.Pointer(&versionInfo[0]), `\`, (unsafe.Pointer)(&fixedInfo), &fixedInfoLen)
					if err != nil {
						log.Println("VerQueryValue failed", err)
						break
					}
					// fmt.Printf("%s: v%d.%d.%d.%d\n", windows.UTF16ToString(me32.Module[:]),
					// 	(fixedInfo.FileVersionMS>>16)&0xff,
					// 	(fixedInfo.FileVersionMS>>0)&0xff,
					// 	(fixedInfo.FileVersionLS>>16)&0xff,
					// 	(fixedInfo.FileVersionLS>>0)&0xff)

					info.Version = fmt.Sprintf("%d.%d.%d.%d",
						(fixedInfo.FileVersionMS>>16)&0xff,
						(fixedInfo.FileVersionMS>>0)&0xff,
						(fixedInfo.FileVersionLS>>16)&0xff,
						(fixedInfo.FileVersionLS>>0)&0xff)
					list.Info = append(list.Info, info)
					list.Total += 1
					break
				}
			}
		}
	}
	return
}

func Is64BitProcess(pid uint32) (bool, error) {
	is64Bit := false
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION, false, pid)
	if err != nil {
		log.Println("Error opening process:", err)
		return is64Bit, errors.New("OpenProcess failed")
	}
	defer windows.CloseHandle(handle)

	err = windows.IsWow64Process(handle, &is64Bit)
	if err != nil {
		log.Println("Error IsWow64Process:", err)
	}
	return !is64Bit, err
}

func GetWeChatKey(info *WeChatInfo) string {
	mediaDB := info.FilePath + "\\Msg\\Media.db"
	if _, err := os.Stat(mediaDB); err != nil {
		log.Printf("open db %s error: %v", mediaDB, err)
		return ""
	}

	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, uint32(info.ProcessID))
	if err != nil {
		log.Println("Error opening process:", err)
		return ""
	}
	defer windows.CloseHandle(handle)

	buffer := make([]byte, info.DllBaseSize)
	err = windows.ReadProcessMemory(handle, uintptr(info.DllBaseAddr), &buffer[0], uintptr(len(buffer)), nil)
	if err != nil {
		log.Println("Error ReadProcessMemory:", err)
		return ""
	}

	offset := 0
	// searchStr := []byte(info.AcountName)
	for {
		index := hasDeviceSybmol(buffer[offset:])
		if index == -1 {
			log.Println("has not DeviceSybmol")
			break
		}
		fmt.Printf("hasDeviceSybmol: 0x%X\n", index)
		keys := findDBKeyPtr(buffer[offset:index], info.Is64Bits)
		// fmt.Println("keys:", keys)

		key, err := findDBkey(handle, info.FilePath+"\\Msg\\Media.db", keys)
		if err == nil {
			// fmt.Println("key:", key)
			return key
		}

		offset += (index + 20)
	}

	return ""
}

func hasDeviceSybmol(buffer []byte) int {
	sybmols := [...][]byte{
		{'a', 'n', 'd', 'r', 'o', 'i', 'd', 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x07, 0x00, 0x00, 0x00},
		{'p', 'a', 'd', '-', 'a', 'n', 'd', 'r', 'o', 'i', 'd', 0x00, 0x00, 0x00, 0x00, 0x00, 0x0B, 0x00, 0x00, 0x00},
		{'i', 'p', 'h', 'o', 'n', 'e', 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00},
		{'i', 'p', 'a', 'd', 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00},
		{'O', 'H', 'O', 'S', 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00},
	}
	for _, syb := range sybmols {
		if index := bytes.Index(buffer, syb); index != -1 {
			return index
		}
	}

	return -1
}

func findDBKeyPtr(buffer []byte, is64Bits bool) [][]byte {
	keys := make([][]byte, 0)
	step := 8
	keyLen := []byte{0x20, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if !is64Bits {
		keyLen = keyLen[:4]
		step = 4
	}

	offset := len(buffer) - step
	for {
		if bytes.Contains(buffer[offset:offset+step], keyLen) {
			keys = append(keys, buffer[offset-step:offset])
		}

		offset -= step
		if offset <= 0 {
			break
		}
	}

	return keys
}

func findDBkey(handle windows.Handle, path string, keys [][]byte) (string, error) {
	var keyAddrPtr uint64
	addrBuffer := make([]byte, 0x08)
	for _, key := range keys {
		copy(addrBuffer, key)
		err := binary.Read(bytes.NewReader(addrBuffer), binary.LittleEndian, &keyAddrPtr)
		if err != nil {
			log.Println("binary.Read:", err)
			continue
		}
		if keyAddrPtr == 0x00 {
			continue
		}
		log.Printf("keyAddrPtr: 0x%X\n", keyAddrPtr)
		keyBuffer := make([]byte, 0x20)
		err = windows.ReadProcessMemory(handle, uintptr(keyAddrPtr), &keyBuffer[0], uintptr(len(keyBuffer)), nil)
		if err != nil {
			// fmt.Println("Error ReadProcessMemory:", err)
			continue
		}
		if checkDataBaseKey(path, keyBuffer) {
			return hex.EncodeToString(keyBuffer), nil
		}
	}

	return "", errors.New("not found key")
}

func checkDataBaseKey(path string, password []byte) bool {
	fp, err := os.Open(path)
	if err != nil {
		return false
	}
	defer fp.Close()

	fpReader := bufio.NewReaderSize(fp, defaultPageSize*100)

	buffer := make([]byte, defaultPageSize)

	n, err := fpReader.Read(buffer)
	if err != nil && n != defaultPageSize {
		log.Println("read failed:", err, n)
		return false
	}

	salt := buffer[:16]
	key := pbkdf2HMAC(password, salt, defaultIter, keySize)

	page1 := buffer[16:defaultPageSize]

	macSalt := xorBytes(salt, 0x3a)
	macKey := pbkdf2HMAC(key, macSalt, 2, keySize)

	hashMac := hmac.New(sha1.New, macKey)
	hashMac.Write(page1[:len(page1)-32])
	hashMac.Write([]byte{1, 0, 0, 0})

	return hmac.Equal(hashMac.Sum(nil), page1[len(page1)-32:len(page1)-12])
}

func (info WeChatInfo) String() string {
	return fmt.Sprintf("PID: %d\nVersion: v%s\nBaseAddr: 0x%08X\nDllSize: %d\nIs 64Bits: %v\nFilePath %s\nAcountName: %s",
		info.ProcessID, info.Version, info.DllBaseAddr, info.DllBaseSize, info.Is64Bits, info.FilePath, info.AcountName)
}

func copyFile(src, dst string) (int64, error) {
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

func silkToMp3(amrBuf []byte, mp3Path string) error {
	amrReader := bytes.NewReader(amrBuf)

	var pcmBuffer bytes.Buffer
	sr := silk.NewWriter(&pcmBuffer)
	sr.Decoder.SetSampleRate(24000)
	amrReader.WriteTo(sr)
	sr.Close()

	if pcmBuffer.Len() == 0 {
		return errors.New("silk to mp3 failed " + mp3Path)
	}

	of, err := os.Create(mp3Path)
	if err != nil {
		return nil
	}
	defer of.Close()

	wr := lame.NewWriter(of)
	wr.Encoder.SetInSamplerate(24000)
	wr.Encoder.SetOutSamplerate(24000)
	wr.Encoder.SetNumChannels(1)
	wr.Encoder.SetQuality(5)
	// IMPORTANT!
	wr.Encoder.InitParams()

	pcmBuffer.WriteTo(wr)
	wr.Close()

	return nil
}

func getPathFileNumber(targetPath string, fileSuffix string) int64 {

	number := int64(0)
	err := filepath.Walk(targetPath, func(path string, finfo os.FileInfo, err error) error {
		if err != nil {
			log.Printf("filepath.Walk：%v\n", err)
			return err
		}
		if !finfo.IsDir() && strings.HasSuffix(path, fileSuffix) {
			number += 1
		}

		return nil
	})
	if err != nil {
		log.Println("filepath.Walk:", err)
	}

	return number
}

func ExportWeChatHeadImage(exportPath string) {
	progress := make(chan string)
	info := WeChatInfo{}

	miscDBPath := fmt.Sprintf("%s\\Msg\\Misc.db", exportPath)
	_, err := os.Stat(miscDBPath)
	if err != nil {
		log.Println("no exist:", miscDBPath)
		return
	}

	headImgPath := fmt.Sprintf("%s\\FileStorage\\HeadImage", exportPath)
	if _, err := os.Stat(headImgPath); err == nil {
		log.Println("has HeadImage")
		return
	}

	go func() {
		exportWeChatHeadImage(info, exportPath, progress)
		close(progress)
	}()

	for p := range progress {
		log.Println(p)
	}
	log.Println("ExportWeChatHeadImage done")
}
