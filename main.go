package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/upload", resoveUpLoad)
	r.Handle("/", http.FileServer(http.Dir("./")))
	http.Handle("/", r)
	log.Fatal(http.ListenAndServe(":5390", r))
}

func createBuildIDRootPath(buildID string) (string, error) {
	dSYMFileRootPath := os.ExpandEnv("$HOME/Downloads/" + buildID + "/")
	err := os.MkdirAll(dSYMFileRootPath, os.ModePerm)
	if err != nil {
		return "", err
	}
	return dSYMFileRootPath, nil
}

func resoveUpLoad(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	if r.MultipartForm != nil {
		buildID := r.MultipartForm.Value["buildID"][0]
		iOS14 := ""
		if len(r.MultipartForm.Value["iOS14"]) > 0 {
			iOS14 = r.MultipartForm.Value["iOS14"][0]
		}
		buildIDRootPath, err := createBuildIDRootPath(buildID)
		if err != nil {
			fmt.Fprintf(w, "create buildIDRootPath failed.")
		}

		formFile, header, err := r.FormFile("logFile")
		if err != nil {
			log.Printf("Get form file failed: %s\n", err)
			fmt.Fprintf(w, "read log file failed.")
		}
		defer formFile.Close()
		logFilePath := buildIDRootPath + header.Filename
		// 创建保存文件
		logFile, err := os.Create(logFilePath)
		if err != nil {
			log.Printf("Create failed: %s\n", err)
			fmt.Fprintf(w, "upload log file failed.")

		}
		defer logFile.Close()

		// 读取表单文件，写入保存文件
		_, err = io.Copy(logFile, formFile)
		if err != nil {
			log.Printf("Write file failed: %s\n", err)
			fmt.Fprintf(w, "save log file failed.")
			return
		}
		dSYMPath, err := fetchSymbolFile(buildID, buildIDRootPath)
		if err != nil {
			return
		}
		outPutLogFilePath := logFilePath + "-crash.log"
		err = generateLogFile(dSYMPath, logFilePath, iOS14 == "iOS14", outPutLogFilePath)
		if err != nil {
			log.Printf("generateLogFile failed: %s\n", err)
			fmt.Fprintf(w, "shell command failed.")
		}
		fmt.Fprintf(w, "successed.")
	}

}

// 从 CI 平台上下载 并解析
func fetchSymbolFile(buildID string, dSYMFileRootPath string) (string, error) {
	dSYMFileUrl := "yourCIPlatform" + buildID + "your.app.dSYM.zip"
	//存放解析好的日志文件目录
	zipFilePath := dSYMFileRootPath + "yourapp.dSYM.zip"
	dSYMFilePath := dSYMFileRootPath + "yourapp.app.dSYM"
	if pathExists(dSYMFilePath) == true {
		return dSYMFilePath, nil
	}
	err := downloadFile(zipFilePath, dSYMFileUrl)
	if err != nil {
		return "", err
	}
	err = unZipFile(zipFilePath, dSYMFileRootPath)
	if err != nil {
		return "", err
	}
	return zipFilePath, nil
}

func generateLogFile(dSYMFilePath string, crashLogFilePath string, iOS14 bool, outputPath string) (err error) {
	developerPath := "export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer;"
	symbolicatecrashPath := "/Applications/Xcode.app/Contents/SharedFrameworks/DVTFoundation.framework/Versions/A/Resources/symbolicatecrash"
	if iOS14 == true {
		developerPath = "export DEVELOPER_DIR=/Applications/Xcode-beta.app/Contents/Developer;"
		symbolicatecrashPath = "/Applications/Xcode-beta.app/Contents/SharedFrameworks/DVTFoundation.framework/Versions/A/Resources/symbolicatecrash"
	}
	c := exec.Command("/bin/bash", "-c", developerPath+symbolicatecrashPath+" "+crashLogFilePath+" "+symbolicatecrashPath+" > "+outputPath)
	out, err := c.CombinedOutput()
	if err != nil {
		fmt.Println("error:", err)
	} else {
		fmt.Println(string(out))
	}
	return err
}

func downloadFile(filepath string, url string) (err error) {

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Writer the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func unZipFile(zipFile string, destDir string) (err error) {
	zipReader, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer zipReader.Close()

	for _, f := range zipReader.File {
		fpath := filepath.Join(destDir, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
		} else {
			if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
				return err
			}

			inFile, err := f.Open()
			if err != nil {
				return err
			}
			defer inFile.Close()

			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer outFile.Close()

			_, err = io.Copy(outFile, inFile)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

//check file is
func pathExists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}
