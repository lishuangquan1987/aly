// +build windows

package view

import (
	"github.com/lxn/walk"

	"aly/client/aly-client/ui/winui/i18n"
)

// ChooseVersion 弹出模态列表让用户选择要回滚到的版本，**阻塞**直到关闭。
// 返回空串表示用户取消。
//
// 注意：与 AskYesNo 同理，walk 的 Synchronize 是异步的，不能在里面写外部
// 变量再直接读（计划原稿那样写会永远拿到零值）。这里用 channel 等回调跑完。
//
// current 是当前版本，会从候选里剔除——回滚到当前版本没有意义。
func (w *Window) ChooseVersion(current string, versions []string) string {
	candidates := make([]string, 0, len(versions))
	for _, v := range versions {
		if v != "" && v != current {
			candidates = append(candidates, v)
		}
	}

	ch := make(chan string, 1)
	w.mw.Synchronize(func() {
		ch <- runVersionDialog(w, current, candidates)
	})
	return <-ch
}

// runVersionDialog 在 UI 线程执行对话框，返回选中的版本（空串=取消）。
func runVersionDialog(w *Window, current string, candidates []string) string {
	dlg, err := walk.NewDialog(w.mw)
	if err != nil {
		return ""
	}
	defer dlg.Dispose()

	_ = dlg.SetTitle(i18n.T(i18n.KeyVersionList))

	vbox := walk.NewVBoxLayout()
	if err := vbox.SetMargins(walk.Margins{HNear: 12, VNear: 12, HFar: 12, VFar: 12}); err != nil {
		return ""
	}
	if err := dlg.SetLayout(vbox); err != nil {
		return ""
	}

	// 当前版本提示（次要信息，放最上面）
	if current != "" {
		lbl, lerr := walk.NewLabel(dlg)
		if lerr == nil {
			lbl.SetText(i18n.Fmt(i18n.KeyCurrentVersion, current))
		}
	}

	lb, err := walk.NewListBox(dlg)
	if err != nil {
		return ""
	}
	// ListBox.SetModel 直接接受 []string
	if err := lb.SetModel(candidates); err != nil {
		return ""
	}
	if len(candidates) > 0 {
		_ = lb.SetCurrentIndex(0)
	}

	chosen := ""
	btn, err := walk.NewPushButton(dlg)
	if err != nil {
		return ""
	}
	_ = btn.SetText(i18n.T(i18n.KeyRollback))
	btn.Clicked().Attach(func() {
		i := lb.CurrentIndex()
		if i >= 0 && i < len(candidates) {
			chosen = candidates[i]
		}
		dlg.Accept()
	})
	_ = dlg.SetDefaultButton(btn)

	cancel, cerr := walk.NewPushButton(dlg)
	if cerr == nil {
		_ = cancel.SetText(i18n.T(i18n.KeyCancel))
		cancel.Clicked().Attach(func() {
			chosen = ""
			dlg.Cancel()
		})
		_ = dlg.SetCancelButton(cancel)
	}

	// 对话框是普通带标题栏窗口，用户可直接关闭；尺寸按内容给足
	_ = dlg.SetBounds(walk.Rectangle{X: 300, Y: 200, Width: 320, Height: 280})
	dlg.Run()
	return chosen
}
