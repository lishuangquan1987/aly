// +build windows

package view

import "testing"

func TestCalcBottomRight(t *testing.T) {
	// 工作区 1920x1080（任务栏已扣除），窗口 380x160，边距 16
	wa := Rect{Left: 0, Top: 0, Right: 1920, Bottom: 1080}
	x, y := CalcBottomRight(wa, Size{W: 380, H: 160}, 16)
	if x != 1524 || y != 904 {
		t.Fatalf("want (1524,904), got (%d,%d)", x, y)
	}
}

func TestCalcBottomRightNonZeroOrigin(t *testing.T) {
	// 副屏工作区原点不在 (0,0)
	wa := Rect{Left: 1920, Top: -200, Right: 3840, Bottom: 880}
	x, y := CalcBottomRight(wa, Size{W: 380, H: 160}, 16)
	if x != 3444 || y != 704 {
		t.Fatalf("want (3444,704), got (%d,%d)", x, y)
	}
}

func TestCalcBottomRightClampsSmallWorkArea(t *testing.T) {
	// 工作区比窗口还小：夹到左上角，不能出负坐标
	wa := Rect{Left: 10, Top: 20, Right: 210, Bottom: 120}
	x, y := CalcBottomRight(wa, Size{W: 380, H: 160}, 16)
	if x != 10 || y != 20 {
		t.Fatalf("want clamp to (10,20), got (%d,%d)", x, y)
	}
}

func TestCalcBottomRightExactFit(t *testing.T) {
	// 工作区恰好等于窗口尺寸：贴左上角（差值 0，不减边距到负）
	wa := Rect{Left: 0, Top: 0, Right: 380, Bottom: 160}
	x, y := CalcBottomRight(wa, Size{W: 380, H: 160}, 16)
	if x != 0 || y != 0 {
		t.Fatalf("want (0,0), got (%d,%d)", x, y)
	}
}
