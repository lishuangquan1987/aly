import subprocess, os, sys

WSL_GO     = "/usr/local/go/bin/go"
WSL_GOROOT = "/usr/local/go"
WSL_GOPATH = "/home/test/go"
WSL_SRC    = f"{WSL_GOPATH}/src/zap/client"

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
SRC_LINUX  = "/mnt/e/Project2026/zap/client"
DST_WIN    = "E:/Yofc/Code/OTDR/OTDR3001/YOFC.OTDR3001/YOFC.OTDR3001/bin/Debug/netcoreapp3.1-windows/UpdateFolder"
DST_LINUX  = "/mnt/e/Yofc/Code/OTDR/OTDR3001/YOFC.OTDR3001/YOFC.OTDR3001/bin/Debug/netcoreapp3.1-windows/UpdateFolder"

def wsl(cmd):
    r = subprocess.run(["wsl", "bash", "-c", cmd], capture_output=True, text=True, timeout=60)
    if r.returncode != 0 and r.stderr.strip():
        print(f"  [WSL] {r.stderr.strip()}")
    return r

# ── Step 1: Copy to WSL ────────────────────────
print("=" * 50)
print("  [1/4] Copy client to WSL ...")
wsl(f"rm -rf {WSL_SRC}")
wsl(f"mkdir -p {WSL_GOPATH}/src/zap")
r = wsl(f"cp -r {SRC_LINUX} {WSL_SRC}")
if r.returncode != 0:
    sys.exit(1)
print("  Done")
print()

# ── Step 2: Build (XP 32-bit) ──────────────────
print("  [2/4] Building zap-update.exe (GOARCH=386) ...")
build = (
    f"cd {WSL_SRC} && "
    f"GOROOT={WSL_GOROOT} GOPATH={WSL_GOPATH} GOOS=windows GOARCH=386 "
    f"{WSL_GO} build -ldflags=\"-s -w\" -o zap-update.exe ."
)
r = wsl(build)
if r.returncode != 0:
    sys.exit(1)
print("  Build OK")
print()

# ── Step 3: Copy output ────────────────────────
print(f"  [3/4] Copy to {DST_WIN} ...")
wsl(f"mkdir -p {DST_LINUX}")
r = wsl(f"cp -f {WSL_SRC}/zap-update.exe {DST_LINUX}/")
if r.returncode != 0:
    sys.exit(1)

r = wsl(f"stat -c%s {DST_LINUX}/zap-update.exe")
size = int(r.stdout.strip())
print(f"  Done ({size / 1024 / 1024:.1f} MB)")
print()

# ── Step 4: Clean ──────────────────────────────
print("  [4/4] Clean WSL ...")
wsl(f"rm -rf {WSL_SRC}")
print("  Done")
print()

print("=" * 50)
print(f"  Deployed: {DST_WIN}/zap-update.exe")
print("=" * 50)
