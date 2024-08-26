package wechat

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
)

const (
	keySize         = 32
	defaultIter     = 64000
	defaultPageSize = 4096
)

func DecryptDataBase(path string, password []byte, expPath string) error {
	sqliteFileHeader := []byte("SQLite format 3")
	sqliteFileHeader = append(sqliteFileHeader, byte(0))

	fp, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fp.Close()

	fpReader := bufio.NewReaderSize(fp, defaultPageSize*100)
	// fpReader := bufio.NewReader(fp)

	buffer := make([]byte, defaultPageSize)

	n, err := fpReader.Read(buffer)
	if err != nil && n != defaultPageSize {
		return fmt.Errorf("read failed")
	}

	salt := buffer[:16]
	key := pbkdf2HMAC(password, salt, defaultIter, keySize)

	page1 := buffer[16:defaultPageSize]

	macSalt := xorBytes(salt, 0x3a)
	macKey := pbkdf2HMAC(key, macSalt, 2, keySize)

	hashMac := hmac.New(sha1.New, macKey)
	hashMac.Write(page1[:len(page1)-32])
	hashMac.Write([]byte{1, 0, 0, 0})

	if !hmac.Equal(hashMac.Sum(nil), page1[len(page1)-32:len(page1)-12]) {
		return fmt.Errorf("incorrect password")
	}

	outFilePath := expPath
	outFile, err := os.Create(outFilePath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// Write SQLite file header
	_, err = outFile.Write(sqliteFileHeader)
	if err != nil {
		return err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	page1 = buffer[16:defaultPageSize]
	iv := page1[len(page1)-48 : len(page1)-32]
	stream := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(page1)-48)
	stream.CryptBlocks(decrypted, page1[:len(page1)-48])
	_, err = outFile.Write(decrypted)
	if err != nil {
		return err
	}
	_, err = outFile.Write(page1[len(page1)-48:])
	if err != nil {
		return err
	}

	for {
		n, err = fpReader.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		} else if n < defaultPageSize {
			return fmt.Errorf("read data to short %d", n)
		}

		iv := buffer[len(buffer)-48 : len(buffer)-32]
		stream := cipher.NewCBCDecrypter(block, iv)
		decrypted := make([]byte, len(buffer)-48)
		stream.CryptBlocks(decrypted, buffer[:len(buffer)-48])
		_, err = outFile.Write(decrypted)
		if err != nil {
			return err
		}
		_, err = outFile.Write(buffer[len(buffer)-48:])
		if err != nil {
			return err
		}
	}

	return nil
}

func pbkdf2HMAC(password, salt []byte, iter, keyLen int) []byte {
	dk := make([]byte, keyLen)
	loop := (keyLen + sha1.Size - 1) / sha1.Size
	key := make([]byte, 0, len(salt)+4)
	u := make([]byte, sha1.Size)
	for i := 1; i <= loop; i++ {
		key = key[:0]
		key = append(key, salt...)
		key = append(key, byte(i>>24), byte(i>>16), byte(i>>8), byte(i))
		hmac := hmac.New(sha1.New, password)
		hmac.Write(key)
		digest := hmac.Sum(nil)
		copy(u, digest)
		for j := 2; j <= iter; j++ {
			hmac.Reset()
			hmac.Write(digest)
			digest = hmac.Sum(digest[:0])
			for k, di := range digest {
				u[k] ^= di
			}
		}
		copy(dk[(i-1)*sha1.Size:], u)
	}
	return dk
}

func xorBytes(a []byte, b byte) []byte {
	result := make([]byte, len(a))
	for i := range a {
		result[i] = a[i] ^ b
	}
	return result
}

/*
func main() {

	str := "82b1a210335140a1bc8a57397391186494abe666595b4f408095538b5518f7d5"
	// 将十六进制字符串解码为字节
	password, err := hex.DecodeString(str)
	if err != nil {
		fmt.Println("解码出错:", err)
		return
	}

	fmt.Println(hex.EncodeToString(password))

	err = decryptMsg("Media.db", password)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Decryption successful!")
	}
}
*/
