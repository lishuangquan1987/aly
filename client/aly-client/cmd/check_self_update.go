package cmd

import (
	"flag"
	"os"
	"path/filepath"

	"aly/client/aly-client/model"
	"aly/client/aly-client/util"
)

// CheckSelfUpdate checks if updator itself needs to be updated
func CheckSelfUpdate() {
	fs := flag.NewFlagSet("check_self_update", flag.ContinueOnError)
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	// A6：ContinueOnError + JSON 信封，参数解析失败 exit 2；-h 打印 usage 后 exit 0
	parseCommandFlags(fs, os.Args[2:])

	fc, err := loadFullConfig("", "", *mainExePathFlag)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	checkPath, err := fc.ExeCfg.CheckUpdaterPath()
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	// If the platform updater (aly-client.exe / aly-client) doesn't exist
	// (first deploy), no update needed
	if _, err := os.Stat(checkPath); os.IsNotExist(err) {
		printOutput(true, "", &model.CheckSelfUpdateData{NeedUpdate: false})
		return
	}

	// Compare SHA256 of self vs the packaged updater (aly-client.exe / aly-client)
	selfPath, err := os.Executable()
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}
	// 与 self_update.go 同款：先解析符号链接再取真实路径，避免对比对象不一致
	if resolved, rErr := filepath.EvalSymlinks(selfPath); rErr == nil {
		selfPath = resolved
	}

	selfSHA256, err := util.FileSHA256(selfPath)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	checkSHA256, err := util.FileSHA256(checkPath)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	printOutput(true, "", &model.CheckSelfUpdateData{
		NeedUpdate: selfSHA256 != checkSHA256,
	})
}
