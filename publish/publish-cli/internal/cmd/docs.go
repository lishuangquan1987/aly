package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// docsInventory prints a machine-readable inventory of every visible command
// and flag in the cobra tree, one item per line:
//
//	CMD <space-joined command path>
//	FLAG <command path> --<name>
//
// website/scripts/check_docs.py consumes it to fail the docs build when a
// command or flag goes missing from the website documentation (docs drift).
var cmdDocs = &cobra.Command{
	Use:    "docs",
	Short:  "输出命令与参数清单（供文档核对脚本使用）",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		var walk func(c *cobra.Command, path string)
		walk = func(c *cobra.Command, path string) {
			if c.Hidden {
				return
			}
			fmt.Printf("CMD %s\n", path)
			c.LocalFlags().VisitAll(func(f *pflag.Flag) {
				if f.Hidden {
					return
				}
				fmt.Printf("FLAG %s --%s\n", path, f.Name)
			})
			for _, sub := range c.Commands() {
				walk(sub, strings.TrimSpace(path+" "+sub.Name()))
			}
		}
		walk(cmd.Root(), cmd.Root().Name())
	},
}

func init() {
	RootCmd.AddCommand(cmdDocs)
}
