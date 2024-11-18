package unzip

import (
	"archive/zip"
	"bytes"
	"fmt"
	log "github.com/sirupsen/logrus"
	"golang.org/x/text/transform"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/mholt/archiver"
	"github.com/nwaples/rardecode"
	"golang.org/x/text/encoding/simplifiedchinese"
)

type ZipFile struct {
	ZipFilePath     string
	ZipType         string
	ZipOutputFolder string
	ZipFileNameList []string
}

func (unzip *ZipFile) Utf8ToGBK(text string) (string, error) {
	dst := make([]byte, len(text)*2)
	tr := simplifiedchinese.GB18030.NewEncoder()
	nDst, _, err := tr.Transform(dst, []byte(text), true)
	if err != nil {
		return text, err
	}
	return string(dst[:nDst]), nil
}
func (unzip *ZipFile) WriteNewFile(fpath string, in io.Reader, fm os.FileMode) error {
	err := os.MkdirAll(filepath.Dir(fpath), 0755)
	if err != nil {
		return fmt.Errorf("%s: making directory for file: %v", fpath, err)
	}
	out, err := os.Create(fpath)
	if err != nil {
		return fmt.Errorf("%s: creating new file: %v", fpath, err)
	}
	defer func(out *os.File) {
		err = out.Close()
		if err != nil {
			log.Error(err)
		}
	}(out)
	err = out.Chmod(fm)
	_, err = io.Copy(out, in)
	if err != nil {
		return fmt.Errorf("%s: writing file: %v", fpath, err)
	}
	return nil
}

func (unzip *ZipFile) fileExists(name string) bool {
	_, err := os.Stat(name)
	return !os.IsNotExist(err)
}

func (unzip *ZipFile) mkdir(dirPath string) error {
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return fmt.Errorf("%s: making directory: %v", dirPath, err)
	}
	return nil
}

func (unzip *ZipFile) Unzip() {

	unzip.ZipFileNameList = make([]string, 0)
	unzip.ZipType = filepath.Ext(unzip.ZipFilePath)
	err := unzip.mkdir(unzip.ZipOutputFolder)
	if err != nil {
		log.Error(err)
		return
	}
	if unzip.ZipType == ".zip" {
		// 第一步，打开 unzip 文件
		zipFile, err := zip.OpenReader(unzip.ZipFilePath)
		if err != nil {
			panic(err)
		}
		defer func(zipFile *zip.ReadCloser) {
			err = zipFile.Close()
			if err != nil {
				log.Error(err)
			}
		}(zipFile)
		// 第二步，遍历 unzip 中的文件
		for _, f := range zipFile.File {
			decodeName := ""
			if utf8.Valid([]byte(f.Name)) {
				decodeName = f.Name
			} else {
				i := bytes.NewReader([]byte(f.Name))
				decoder := transform.NewReader(i, simplifiedchinese.GB18030.NewDecoder())
				content, _ := io.ReadAll(decoder)
				decodeName = string(content)
			}
			decodeName = filepath.Base(decodeName)
			filePath := filepath.Join(unzip.ZipOutputFolder, decodeName)
			unzip.ZipFileNameList = append(unzip.ZipFileNameList, decodeName)
			if f.FileInfo().IsDir() {
				// _ = os.MkdirAll(filePath, os.ModePerm)
				continue
			}
			// 创建对应文件夹
			if err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
				panic(err)
			}
			// 解压到的目标文件
			dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				panic(err)
			}
			file, err := f.Open()
			if err != nil {
				panic(err)
			}
			// 写入到解压到的目标文件
			if _, err := io.Copy(dstFile, file); err != nil {
				panic(err)
			}
			err = dstFile.Close()
			if err != nil {
				return
			}
			err = file.Close()
			if err != nil {
				log.Error(err)
				return
			}
		}
	} else if unzip.ZipType == ".rar" {
		r := archiver.NewRar()
		rarFile := unzip.ZipFilePath
		destination := unzip.ZipOutputFolder + "\\"
		count := 0
		err := r.Walk(rarFile, func(f archiver.File) error {
			if !f.IsDir() {
				count += 1
			}
			return nil
		})
		if err != nil {
			panic(err)
		}
		log.Debug("File count: ", count)

		err = r.Walk(rarFile, func(f archiver.File) error {
			rh, ok := f.Header.(*rardecode.FileHeader)

			if !ok {
				return fmt.Errorf("expected header")
			}
			to := destination + filepath.Base(rh.Name)
			log.Debug("解压出文件:", filepath.Base(rh.Name))
			unzip.ZipFileNameList = append(unzip.ZipFileNameList, filepath.Base(to))
			if !f.IsDir() && !r.OverwriteExisting && unzip.fileExists(to) {
				return fmt.Errorf("file already exists: %s", to)
			}
			err := unzip.mkdir(filepath.Dir(to))
			if err != nil {
				return fmt.Errorf("making parent directories: %v", err)
			}
			if f.IsDir() {
				return nil
			}

			return unzip.WriteNewFile(to, f.ReadCloser, rh.Mode())
		})
	}
}
