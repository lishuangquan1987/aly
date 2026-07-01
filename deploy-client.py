import subprocess, os, sys, json

WSL_GO     = "/usr/local/go/bin/go"
WSL_GOROOT = "/usr/local/go"
WSL_GOPATH = "/home/test/go"
WSL_SRC    = f"{WSL_GOPATH}/src/aly/client/aly-client"

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
SRC_LINUX  = "/mnt/e/Project2026/aly/client/aly-client"

CONFIG_PATH = os.path.join(
    os.environ.get("LOCALAPPDATA", ""),
    "AlyPublish", "config.json"
)

def wsl(cmd):
    r = subprocess.run(["wsl", "bash", "-c", cmd], capture_output=True, text=True, timeout=60)
    if r.returncode != 0 and r.stderr.strip():
        print(f"  [WSL] {r.stderr.strip()}")
    return r

def win_to_wsl(win_path):
    """Convert Windows absolute path (E:\foo\bar) to WSL mount path (/mnt/e/foo/bar)."""
    drive = win_path[0].lower()
    rest = win_path[2:].replace("\\", "/")
    return f"/mnt/{drive}{rest}"

def load_deploy_targets():
    """Read config.json, return deduplicated list of (win_path, wsl_path) deploy targets.
    
    For each project, two targets are produced:
      1) ProjectPath itself
      2) ProjectPath/../UpdateFolder
    """
    if not os.path.exists(CONFIG_PATH):
        print(f"  [WARN] config.json not found: {CONFIG_PATH}")
        return []

    with open(CONFIG_PATH, "r", encoding="utf-8") as f:
        projects = json.load(f)

    targets = []
    seen = set()
    for proj in projects:
        project_path = proj.get("ProjectPath", "")
        if not project_path or not os.path.isabs(project_path):
            continue

        # Target 1: ProjectPath itself
        if project_path not in seen:
            seen.add(project_path)
            targets.append((project_path, win_to_wsl(project_path)))

        # Target 2: ProjectPath/../UpdateFolder
        update_folder = os.path.normpath(os.path.join(project_path, "..", "UpdateFolder"))
        if update_folder not in seen:
            seen.add(update_folder)
            targets.append((update_folder, win_to_wsl(update_folder)))

    return targets

# ── Step 1: Copy to WSL ────────────────────────
print("=" * 50)
print("  [1/4] Copy client to WSL ...")
wsl(f"rm -rf {WSL_SRC}")
wsl(f"mkdir -p {WSL_GOPATH}/src/aly/client")
r = wsl(f"cp -r {SRC_LINUX} {WSL_SRC}")
if r.returncode != 0:
    sys.exit(1)
print("  Done")
print()

# ── Step 2: Build (XP 32-bit) ──────────────────
print("  [2/4] Building aly-client.exe (GOARCH=386) ...")
build = (
    f"cd {WSL_SRC} && "
    f"GOROOT={WSL_GOROOT} GOPATH={WSL_GOPATH} GOOS=windows GOARCH=386 "
    f"{WSL_GO} build -ldflags=\"-s -w\" -o aly-client.exe ."
)
r = wsl(build)
if r.returncode != 0:
    sys.exit(1)
print("  Build OK")
print()

# ── Step 3: Deploy to all targets ──────────────
targets = load_deploy_targets()
if not targets:
    print("  [3/4] No deploy targets found in config.json")
    print()
else:
    print(f"  [3/4] Deploying to {len(targets)} target(s) ...")
    fail = False
    for idx, (win_path, wsl_path) in enumerate(targets, 1):
        print(f"         [{idx}/{len(targets)}] {win_path}")
        r = wsl(f"mkdir -p {wsl_path}")
        if r.returncode != 0:
            print(f"         [FAIL] mkdir failed")
            fail = True
            continue
        r = wsl(f"cp -f {WSL_SRC}/aly-client.exe {wsl_path}/")
        if r.returncode != 0:
            print(f"         [FAIL] copy failed")
            fail = True
            continue
        # show file size
        r = wsl(f"stat -c%s {wsl_path}/aly-client.exe")
        if r.returncode == 0:
            size = int(r.stdout.strip())
            print(f"         OK ({size / 1024 / 1024:.1f} MB)")
        else:
            print(f"         OK")
    if fail:
        sys.exit(1)
    print()

# ── Step 4: Clean ──────────────────────────────
print("  [4/4] Clean WSL ...")
wsl(f"rm -rf {WSL_SRC}")
print("  Done")
print()

print("=" * 50)
if targets:
    for win_path, _ in targets:
        print(f"  Deployed: {win_path}\\aly-client.exe")
else:
    print("  No targets deployed (check config.json)")
print("=" * 50)
