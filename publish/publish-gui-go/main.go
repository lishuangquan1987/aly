package main

import (
	"flag"
	"log"

	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/locales/zh-Hans"
	"github.com/egoist/mygo/ui"
)

func main() {
	lang := flag.String("lang", "", "ui language: zh or en")
	flag.Parse()

	initLogging()
	a := newApp()
	switch *lang {
	case "zh", "zh-CN", "zh-Hans":
		a.lang = "zh"
	case "en", "en-US":
		a.lang = "en"
	}
	if *lang != "" {
		if a.cli.available() {
			a.status = a.tf("cli.found", "path", a.cli.exe)
		} else {
			a.status = a.t("toolbar.cliMissing")
		}
	}

	log.Printf("AlyPublish Go start; cli=%s", a.cli.exe)
	mygo.App.SetName("AlyPublish")
	mygo.App.SetVersion("0.1.0")
	mygo.App.SetLanguages("en", "zh-Hans")
	mygo.App.SetLocale(a.localeTag())
	mygo.App.OnWindowAllClosed(func() { mygo.App.Quit() })

	mygo.App.WhenReady(func() {
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:           a.t("app.title"),
			Width:           1320,
			Height:          840,
			MinWidth:        1080,
			MinHeight:       640,
			Content:         ui.View(a.view),
			BackgroundColor: "light-dark(#f7f8fa, #1e1e1e)",
		})
		a.win.OnClosed(func() { mygo.App.Quit() })
		a.start()
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
