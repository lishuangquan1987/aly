// +build windows

package view

// Rect 是工作区矩形（像素）。
type Rect struct {
	Left, Top, Right, Bottom int
}

// Size 是窗口尺寸（像素）。
type Size struct {
	W, H int
}

// CalcBottomRight 计算窗口贴工作区右下角时的左上角坐标。
//
// 工作区应取 MONITORINFO.RcWork（已扣除任务栏），而不是 RcMonitor。
// 若工作区比窗口还小，则夹到工作区左上角，避免出现负坐标或越界。
func CalcBottomRight(wa Rect, win Size, margin int) (int, int) {
	x := wa.Right - win.W - margin
	y := wa.Bottom - win.H - margin
	if x < wa.Left {
		x = wa.Left
	}
	if y < wa.Top {
		y = wa.Top
	}
	return x, y
}
