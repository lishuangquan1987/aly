package cmd

import (
	"flag"
	"os"

	"zap/client/zap-client/model"
	"zap/client/zap-client/util"
)

// CheckSelfUpdate checks if updator itself needs to be updated
func CheckSelfUpdate() {
	fs := flag.NewFlagSet("check_self_update", flag.ExitOnError)
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	fs.Parse(os.Args[2:])

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

	// If zap-client.exe doesn't exist (first deploy), no update needed
	if _, err := os.Stat(checkPath); os.IsNotExist(err) {
		printOutput(true, "", &model.CheckSelfUpdateData{NeedUpdate: false})
		return
	}

	// Compare SHA256 of self vs zap-client.exe
	selfPath, err := os.Executable()
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
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
