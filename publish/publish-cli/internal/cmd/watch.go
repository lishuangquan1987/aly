package cmd

import (
	"os"
	"os/signal"
	"time"

	"zap/publish-cli/internal/diff"
	"zap/publish-cli/internal/staging"

	"github.com/spf13/cobra"
)

var (
	watchInterval int
	watchAutoAdd bool
)

func init() {
	cmdWatch.Flags().IntVar(&watchInterval, "interval", 2, "轮询间隔（秒）")
	cmdWatch.Flags().BoolVar(&watchAutoAdd, "auto-add", false, "将变更文件加入暂存区")
	RootCmd.AddCommand(cmdWatch)
}

var cmdWatch = &cobra.Command{
	Use:   "watch",
	Short: "实时监控文件变更",
	Long:  "轮询监控本地目录的文件新增/修改/删除，检测到变更时实时打印差异摘要。",
	Run:   runWatch,
}

func runWatch(cmd *cobra.Command, args []string) {
	cfg, err := resolveConfig()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if err := requireProject(&cfg); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}

	printHumanLn("Watching %s (interval: %ds, auto-add: %v)", cfg.Path, watchInterval, watchAutoAdd)
	printHumanLn("Press Ctrl+C to stop.\n")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	ticker := time.NewTicker(time.Duration(watchInterval) * time.Second)
	defer ticker.Stop()

	prevFiles := scanLocal(cfg.Path, cfg.Shared.IgnoreFolders, cfg.Shared.IgnoreFiles)

	for {
		select {
		case <-sigCh:
			printHumanLn("\nStopped.")
			return
		case <-ticker.C:
			currFiles := scanLocal(cfg.Path, cfg.Shared.IgnoreFolders, cfg.Shared.IgnoreFiles)
			changes := detectChanges(prevFiles, currFiles)
			if len(changes) > 0 {
				timestamp := time.Now().Format("15:04:05")
				for _, ch := range changes {
					printHumanLn("[%s] %s: %s", timestamp, ch.status, ch.path)
				}
				if watchAutoAdd {
					var paths []string
					for _, ch := range changes {
						if ch.status == "new" || ch.status == "modified" {
							paths = append(paths, ch.path)
						}
					}
					if len(paths) > 0 {
						if addErr := staging.Add(cfg.Path, paths); addErr != nil {
							printHumanLn("[WARN] auto-add failed: %v\n", addErr)
						}
						printHumanLn("[%s] Auto-added %d files to staging", timestamp, len(paths))
					}
				}
			}
			prevFiles = currFiles
		}
	}
}

type fileSnap struct {
	path string
	md5  string
}

type fileChg struct {
	path   string
	status string
}

func scanLocal(projectPath string, ignoreFolders []string, ignoreFiles []string) []fileSnap {
	files, scanErr := diff.ScanDirectory(projectPath, ignoreFolders, ignoreFiles)
	if scanErr != nil {
		printHumanLn("[WARN] scan error: %v\n", scanErr)
	}
	var result []fileSnap
	for _, f := range files {
		result = append(result, fileSnap{path: f.RelativePath, md5: f.MD5})
	}
	return result
}

func detectChanges(prev, curr []fileSnap) []fileChg {
	prevMap := make(map[string]string)
	for _, f := range prev {
		prevMap[f.path] = f.md5
	}
	var changes []fileChg
	for _, f := range curr {
		prevMD5, existed := prevMap[f.path]
		if !existed {
			changes = append(changes, fileChg{path: f.path, status: "new"})
		} else if prevMD5 != f.md5 {
			changes = append(changes, fileChg{path: f.path, status: "modified"})
		}
		delete(prevMap, f.path)
	}
	for path := range prevMap {
		changes = append(changes, fileChg{path: path, status: "deleted"})
	}
	return changes
}
