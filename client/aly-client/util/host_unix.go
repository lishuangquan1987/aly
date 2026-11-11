package util

// hos tProcessID Unix：父 PID 直接问系统（与 FilterKillablePIDs 用的是同一个来源）。
func hos tProcessID() uint32 {
	return uint32(os.Getppid())
}
