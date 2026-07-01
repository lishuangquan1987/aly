import paramiko
import os, sys, getpass, subprocess

HOST = "10.96.115.14"
USER = "quartecs"
REMOTE_DIR = "/home/quartecs/lishuangquan/aly"
BINARY = "aly-server-linux-amd64"
PORT = 7000

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
SERVER_DIR = os.path.join(SCRIPT_DIR, "server")
LOCAL_PATH = os.path.join(SERVER_DIR, BINARY)

# ── Step 1: Build ────────────────────────────
print("=" * 50)
print(f"  [1/5] Building {BINARY} (linux/amd64) ...")
env = os.environ.copy()
env["GOOS"] = "linux"
env["GOARCH"] = "amd64"
ret = subprocess.run(["go", "build", "-o", BINARY, "."],
                     cwd=SERVER_DIR, env=env).returncode
if ret != 0:
    print("[ERROR] Build failed!")
    sys.exit(1)
print(f"       Build OK ({os.path.getsize(LOCAL_PATH)/1024/1024:.1f} MB)")
print()

# ── Step 2-5: SSH once ───────────────────────
try:
    password = sys.argv[1]
except IndexError:
    password = getpass.getpass(f"Password for {USER}@{HOST}: ")

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=password, timeout=30)
password = None
print("       Connected.\n")

# Step 2: Kill old process FIRST so file is not locked
# 优雅关闭：先 SIGTERM，等待 5s，仍存活则 SIGKILL
print(f"[2/5] Stopping port {PORT} ...")
_, stop_out, _ = ssh.exec_command(
    f"pids=$(lsof -ti :{PORT}); "
    f"if [ -n \"$pids\" ]; then "
    f"  kill -TERM $pids 2>/dev/null; "
    f"  for i in $(seq 1 5); do "
    f"    sleep 1; "
    f"    if ! kill -0 $pids 2>/dev/null; then break; fi; "
    f"  done; "
    f"  kill -9 $pids 2>/dev/null; "
    f"fi; "
    f"sleep 1; echo OK",
    timeout=15)
stop_out.channel.recv_exit_status()
print("       done\n")

# Step 3: Upload (file is now unlocked)
print("[3/5] Uploading ...")
sftp = ssh.open_sftp()
sftp.put(LOCAL_PATH, f"{REMOTE_DIR}/{BINARY}")
sftp.close()
print("       Upload OK\n")

# Step 4: chmod
print("[4/5] chmod +x ...")
ssh.exec_command(f"chmod +x {REMOTE_DIR}/{BINARY}", timeout=5)
print("       done\n")

# Step 5: Start (setsid: detach from SSH session so process survives after ssh.close)
print("[5/5] Starting server (detached) ...")
ssh.exec_command(
    f"cd {REMOTE_DIR} && setsid ./{BINARY} -p {PORT} </dev/null >>nohup.out 2>&1 &",
    timeout=5)
print("       done\n")

# Verify
stdin, stdout, stderr = ssh.exec_command(
    f"lsof -i :{PORT} 2>/dev/null | head -2", timeout=5)
out = stdout.read().decode().strip()
if "LISTEN" in out:
    print("=" * 50)
    print("  Deploy success! Port 7000 is running.")
    print("=" * 50)
else:
    print("[WARN] Port 7000 not detected — check manually.")

ssh.close()
