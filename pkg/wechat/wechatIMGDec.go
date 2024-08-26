package wechat

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

/*
	from: https://github.com/liuggchen/wechatDatDecode.git
*/

var imagePrefixBtsMap = make(map[string][]byte)

func DecryptDatByDir(inDir, outDir string) error {

	startTime := time.Now()
	f, er := os.Open(inDir)
	if er != nil {
		fmt.Println(er.Error())
		return er
	}
	readdir, er := f.Readdir(0)
	if er != nil {
		fmt.Println(er.Error())
	}

	if stat, er := os.Stat(outDir); os.IsNotExist(er) {
		er := os.MkdirAll(outDir, 0755)
		if er != nil {
			return er
		}
	} else if !stat.IsDir() {
		return errors.New(outDir + "is file")
	}

	var taskChan = make(chan os.FileInfo, 100)

	go func() {
		for _, info := range readdir {
			taskChan <- info
		}
		close(taskChan)
	}()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for info := range taskChan {
				handlerOne(info, inDir, outDir)
			}
		}()
	}

	wg.Wait()
	t := time.Since(startTime).Seconds()
	log.Printf("\nfinished time= %v s\n", t)

	return nil
}

func DecryptDat(inFile string, outFile string) error {

	sourceFile, err := os.Open(inFile)
	if err != nil {
		log.Println(err.Error())
		return err
	}

	var preTenBts = make([]byte, 10)
	_, _ = sourceFile.Read(preTenBts)
	decodeByte, _, er := findDecodeByte(preTenBts)
	if er != nil {
		log.Println(er.Error())
		return err
	}

	distFile, er := os.Create(outFile)
	if er != nil {
		log.Println(er.Error())
		return err
	}
	writer := bufio.NewWriter(distFile)
	_, _ = sourceFile.Seek(0, 0)
	var rBts = make([]byte, 1024)
	for {
		n, er := sourceFile.Read(rBts)
		if er != nil {
			if er == io.EOF {
				break
			}
			log.Println("error: ", er.Error())
			return err
		}
		for i := 0; i < n; i++ {
			_ = writer.WriteByte(rBts[i] ^ decodeByte)
		}
	}
	_ = writer.Flush()
	_ = distFile.Close()
	_ = sourceFile.Close()
	// fmt.Println("output file：", distFile.Name())

	return nil
}

func handlerOne(info os.FileInfo, dir string, outputDir string) {
	if info.IsDir() || filepath.Ext(info.Name()) != ".dat" {
		return
	}
	fmt.Println("find file: ", info.Name())
	fPath := dir + "/" + info.Name()
	sourceFile, err := os.Open(fPath)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	var preTenBts = make([]byte, 10)
	_, _ = sourceFile.Read(preTenBts)
	decodeByte, _, er := findDecodeByte(preTenBts)
	if er != nil {
		fmt.Println(er.Error())
		return
	}

	distFile, er := os.Create(outputDir + "/" + info.Name())
	if er != nil {
		fmt.Println(er.Error())
		return
	}
	writer := bufio.NewWriter(distFile)
	_, _ = sourceFile.Seek(0, 0)
	var rBts = make([]byte, 1024)
	for {
		n, er := sourceFile.Read(rBts)
		if er != nil {
			if er == io.EOF {
				break
			}
			fmt.Println("error: ", er.Error())
			return
		}
		for i := 0; i < n; i++ {
			_ = writer.WriteByte(rBts[i] ^ decodeByte)
		}
	}
	_ = writer.Flush()
	_ = distFile.Close()

	fmt.Println("output file：", distFile.Name())
}

func init() {
	//JPEG (jpg)，文件头：FFD8FF
	//PNG (png)，文件头：89504E47
	//GIF (gif)，文件头：47494638
	//TIFF (tif)，文件头：49492A00
	//Windows Bitmap (bmp)，文件头：424D
	const (
		Jpeg = "FFD8FF"
		Png  = "89504E47"
		Gif  = "47494638"
		Tif  = "49492A00"
		Bmp  = "424D"
	)
	JpegPrefixBytes, _ := hex.DecodeString(Jpeg)
	PngPrefixBytes, _ := hex.DecodeString(Png)
	GifPrefixBytes, _ := hex.DecodeString(Gif)
	TifPrefixBytes, _ := hex.DecodeString(Tif)
	BmpPrefixBytes, _ := hex.DecodeString(Bmp)

	imagePrefixBtsMap = map[string][]byte{
		".jpeg": JpegPrefixBytes,
		".png":  PngPrefixBytes,
		".gif":  GifPrefixBytes,
		".tif":  TifPrefixBytes,
		".bmp":  BmpPrefixBytes,
	}
}

func findDecodeByte(bts []byte) (byte, string, error) {
	for ext, prefixBytes := range imagePrefixBtsMap {
		deCodeByte, err := testPrefix(prefixBytes, bts)
		if err == nil {
			return deCodeByte, ext, err
		}
	}
	return 0, "", errors.New("decode fail")
}

func testPrefix(prefixBytes []byte, bts []byte) (deCodeByte byte, error error) {
	var initDecodeByte = prefixBytes[0] ^ bts[0]
	for i, prefixByte := range prefixBytes {
		if b := prefixByte ^ bts[i]; b != initDecodeByte {
			return 0, errors.New("no")
		}
	}
	return initDecodeByte, nil
}
