# 鏇存柊瀹㈡埛绔璁℃枃妗?
## 鍩烘湰淇℃伅

- **瀹氫綅**锛氬懡浠よ宸ュ叿锛屾柟渚夸笌鍏朵粬瀹㈡埛绔簲鐢ㄩ泦鎴?
- **璇█**锛欸o 1.10锛堝吋瀹?XP锛屼笉浣跨敤 go modules锛?
- **鍏煎鎬?*锛歐indows XP 鍙婁互涓?
- **鏋勫缓**锛歚set GOARCH=386 && set GO111MODULE=off && go build -ldflags="-s -w"`

- **杈撳嚭绾﹀畾**锛氭爣鍑唈son锛岀姝㈡祦寮忚緭鍑猴紝鎵ц瀹屼箣鍚庯紝涓€娆℃€ц緭鍑?
  ```json
  {
  	"isSuccess":true,
  	"errorMsg":"",
  	"data":...
  }
  ```

- 鏇存柊绋嬪簭涓庝富绋嬪簭鐨勭洰褰曞叧绯?
  ```
  PackageFolder/
  鈹溾攢鈹€ ApplicationFolder    #涓荤▼搴忔枃浠跺す
  |    鈹斺攢鈹€ main_application.exe
  |    鈹斺攢鈹€ ...   
  鈹溾攢鈹€ UpdateFolder         #鏇存柊绋嬪簭鏂囦欢澶?  |    鈹斺攢鈹€ client_updator.exe
  |    鈹斺攢鈹€ version.json    #璁板綍褰撳墠鐗堟湰 姣斿鏄?.0.4
  |    鈹斺攢鈹€ client.yml      #閰嶇疆鏂囦欢
  |鈹€鈹€ ApplicationFolder_0.0.1    #涓荤▼搴忔枃浠跺す 0.0.1鐗堟湰
  |    鈹斺攢鈹€ main_application.exe
  |    鈹斺攢鈹€ ...
  |鈹€鈹€ ApplicationFolder_0.0.2    #涓荤▼搴忔枃浠跺す 0.0.2鐗堟湰
  |    鈹斺攢鈹€ main_application.exe
  |    鈹斺攢鈹€ ...
  |鈹€鈹€ ApplicationFolder_0.0.3    #涓荤▼搴忔枃浠跺す 0.0.3鐗堟湰
  |    鈹斺攢鈹€ main_application.exe
  |    鈹斺攢鈹€ ...
  ```

  

---

 

## 閰嶇疆鏂囦欢

### version.json

浣嶄簬 `client-updator.exe` 鍚岀骇鐩綍涓嬶紝璁板綍褰撳墠鐗堟湰淇℃伅锛?
```json
{
  "version_previouse":"0.0.9", //涓婁竴涓増鏈殑鐗堟湰鍙?鐢ㄤ簬澶囦唤
  "version": "1.0.0", // 褰撳墠鐗堟湰鐨勭増鏈彿
  "version_status": "applied"
}
```

**version_status 瀛楁璇存槑**锛?
| 鍊?| 鍚箟 |
| --- | --- |
| `downloaded` | 宸蹭笅杞斤紝寰呭簲鐢?|
| `applying` | 姝ｅ湪搴旂敤鏇存柊锛堝穿婧冨悗鐢ㄤ簬鎭㈠锛?|
| `applied` | 宸插簲鐢?|

### client.yaml

```yaml
project_name: "otdr_client"
url: "http://127.0.0.1:2000"
main_exe_relative_path: "../ApplicationFolder/main_application.exe"  # main_exe鐩稿浜巆lient-updator.exe鐨勭洰褰?un_copy_files:
  - xxx.log
un_copy_folders:
  - Logs
  - x86
must_close_process_name:
  - main_application
post_update_script: ""
```

**瀛楁璇存槑**锛?
| 瀛楁 | 璇存槑 |
| --- | --- |
| `url` | 鏈嶅姟鍣ㄥ湴鍧€锛堝唴缃?HTTP锛?|
| `main_exe_relative_path` | 涓荤▼搴忕浉瀵硅矾寰勶紙鐩稿浜?`client-updator.exe` 鎵€鍦ㄧ洰褰曪級 |
| `must_close_process_name` | 鏇存柊鏃跺繀椤诲叧闂殑杩涚▼鍚嶅垪琛?|
| `post_update_script` | 鏇存柊瀹屾垚鍚庢墽琛岀殑鑴氭湰璺緞锛堝彲閫夛紝鐩稿浜庝富绋嬪簭鏍圭洰褰曪級 |
| un_copy_files | 鎵ц鏇存柊鏃讹紝涓嶇敤澶嶅埗鐨勬枃浠讹紙鐩稿浜嶢pplicationFolder锛?|
| un_copy_folders | 鎵ц鏇存柊鏃讹紝涓嶇敤澶嶅埗鐨勬枃浠跺す锛堢浉瀵逛簬ApplicationFolder锛?|

**浼樺厛绾?*锛氬懡浠よ鍙傛暟 > client.yaml 閰嶇疆鍊笺€?
---

## 绾﹀畾

- `main_exe_relative_path_folder` = `main_exe_relative_path` 鎵€鍦ㄧ殑鏂囦欢澶癸紙鍗充富绋嬪簭鏍圭洰褰曪級
- `main_exe_folder_name` = main_exe_relative_path_folder 鐨勬枃浠跺す鍚嶇О锛屽嵆涓荤▼搴忔枃浠跺す鍚嶇О

---

## 椤圭洰缁撴瀯

```
client/
鈹溾攢鈹€ README.MD
鈹溾攢鈹€ main.go                    # 鍏ュ彛锛孋LI 鍙傛暟瑙ｆ瀽
鈹溾攢鈹€ config/
鈹?  鈹溾攢鈹€ config.go              # 閰嶇疆鍔犺浇锛坈lient.yaml + 鍛戒护琛屽弬鏁板悎骞讹級
鈹?  鈹斺攢鈹€ version.go             # version.json 璇诲啓
鈹溾攢鈹€ http_client/
鈹?  鈹斺攢鈹€ http_client.go         # 鏈嶅姟绔?HTTP API 璋冪敤
鈹溾攢鈹€ cmd/
鈹?  鈹溾攢鈹€ check_update.go        # 妫€鏌ユ洿鏂?鈹?  鈹溾攢鈹€ check_diff.go          # 鏂囦欢姣斿
鈹?  鈹溾攢鈹€ download_update.go     # 涓嬭浇鏇存柊
鈹?  鈹溾攢鈹€ apply_update.go        # 鎵ц鏇存柊
鈹?  鈹溾攢鈹€ list_rollback.go       # 鍒楀嚭鍙洖婊氱増鏈?鈹?  鈹溾攢鈹€ rollback.go            # 鐗堟湰鍥炴粴
鈹?  鈹斺攢鈹€ check_self_update.go   # 鑷洿鏂版鏌?鈹溾攢鈹€ util/
鈹?  鈹溾攢鈹€ file.go                # 鏂囦欢鎿嶄綔锛堝鍒躲€丼HA-256銆丮D5銆侀亶鍘嗭級
鈹?  鈹斺攢鈹€ process.go             # 杩涚▼绠＄悊锛堟煡鎵俱€佸叧闂€佺瓑寰咃級
鈹斺攢鈹€ model/
    鈹溾攢鈹€ dto.go                 # 涓庢湇鍔＄涓€鑷寸殑璇锋眰/鍝嶅簲 DTO
    鈹斺攢鈹€ diff.go                # 鏂囦欢宸紓缁撴瀯
```

### 渚濊禆搴?
| 鐢ㄩ€?| 搴?| 璇存槑 |
| --- | --- | --- |
| CLI 鍙傛暟瑙ｆ瀽 | 鏍囧噯搴?`flag` | Go 1.10 鍐呯疆 |
| YAML 瑙ｆ瀽 | `gopkg.in/yaml.v2` | 鍏煎 Go 1.10 |
| HTTP 璇锋眰 | 鏍囧噯搴?`net/http` | Go 1.10 鍐呯疆 |
| JSON 搴忓垪鍖?| 鏍囧噯搴?`encoding/json` | Go 1.10 鍐呯疆 |
| SHA-256 璁＄畻 | 鏍囧噯搴?`crypto/sha256` | Go 1.10 鍐呯疆 |
| MD5 璁＄畻 | 鏍囧噯搴?`crypto/md5` | Go 1.10 鍐呯疆 |
| 杩涚▼绠＄悊 | `golang.org/x/sys/windows` | Windows 杩涚▼鎿嶄綔 |

### 鏂囦欢瀹屾暣鎬?
- 鏈嶅姟绔繑鍥炴枃浠跺垪琛ㄦ椂闄勫甫姣忎釜鏂囦欢鐨?MD5 鍜?SHA-256
- 涓嬭浇鍚?*鍚屾椂鏍￠獙 MD5 鍜?SHA-256**锛屼换涓€涓嶅尮閰嶅垯閲嶈瘯锛堟渶澶?3 娆★級
- 鎵€鏈夋牎楠屽け璐ョ殑鏂囦欢璁板綍鍒?`update_{version}_fail.log`

---

## 涓庢湇鍔＄ API 瀵规帴

瀹㈡埛绔皟鐢ㄦ湇鍔＄宸叉湁鎺ュ彛锛?*涓嶉渶瑕佹柊澧?API**锛?
| 鍛戒护 | 璋冪敤鎺ュ彛 | 鏂规硶 |
| --- | --- | --- |
| check_update | `/api/project/get_project_change_logs/{id}` | GET |
| check_diff | `/api/file/get_all_files/{id}` | GET |
| download_update | `/api/file/get_all_files/{id}` + `/api/file/download_file?path=` | GET |
| check_self_update | 鏃狅紙绾湰鍦?SHA-256 姣斿锛?| 鈥?|

鏈嶅姟绔枃浠跺垪琛ㄨ繑鍥炵殑姣忎釜鏂囦欢闇€鍖呭惈 `sha256` 瀛楁鐢ㄤ簬瀹屾暣鎬ф牎楠屻€俢heck_update 鐨勫彉鏇存棩蹇楁潯鐩渶鍖呭惈 `force_update` 瀛楁銆?
---

## 鍔熻兘

### 1. 妫€鏌ユ洿鏂?
娉ㄦ剰锛氭姝ラ闇€妫€鏌?version.json 涓殑 status 鐘舵€併€傚鏋滀负 `downloaded` 鎴?`applying`锛屽垯鐩存帴杩斿洖鏈夋洿鏂帮紝鐗堟湰涓哄綋鍓?version.json 涓褰曠殑鐗堟湰鍙枫€?
```bash
client-updator.exe check_update [--url <url>] [--project-name <name>]
```

**璋冪敤鎺ュ彛**锛歚GET {url}/api/project/get_project_change_logs/{project_id}`

**姣斿鏂规硶**锛氳鍙栨湰鍦?version.json 鐨勭増鏈彿涓庢湇鍔＄鏈€鏂板彉鏇存棩蹇楃殑鐗堟湰鍙锋瘮杈冿紝涓嶄竴鑷村垯瑙嗕负鏈夋洿鏂般€?*鑻ユ湰鍦颁笉瀛樺湪 version.json**锛岃兘鑾峰彇鍒版湇鍔＄鏂扮増鏈俊鎭垯璁や负鏈夋洿鏂帮紙`current_version` 杩斿洖绌哄瓧绗︿覆锛夈€?
**杩斿洖鍊?*锛坰tdout JSON锛夛細

鏈夋洿鏂帮細
```json
{
	"isSuccess":true,
	"errorMsg":"",
	"data":{"has_update":true,"current_version":"1.0.0","new_version":"1.0.1","force_update":false}
}
```

鏃犳洿鏂帮細
```json
{
	"isSuccess":true,
	"errorMsg":"",
	"data":{"has_update":false,"current_version":"1.0.0"}
}
```

閿欒锛?```json
{
	"isSuccess":false,
	"errorMsg":"杩炴帴鏈嶅姟鍣ㄥけ璐? dial tcp: connect refused",
	"data":null
}
```

**瀛楁璇存槑**锛?
| 瀛楁 | 绫诲瀷 | 璇存槑 |
| --- | --- | --- |
| `has_update` | bool | 鏄惁鏈夋柊鐗堟湰 |
| `current_version` | string | 鏈湴褰撳墠鐗堟湰鍙?|
| `new_version` | string锛堝彲閫夛級 | 鏂扮増鏈彿锛屼粎 `has_update=true` 鏃跺嚭鐜?|
| `force_update` | bool锛堝彲閫夛級 | 鏄惁寮哄埗鏇存柊锛堟潵鑷湇鍔＄鏈€鏂扮増鏈殑 `force_update` 瀛楁锛夛紝浠?`has_update=true` 鏃跺嚭鐜般€俙true` 鏃惰皟鐢ㄦ柟搴旇嚜鍔ㄤ笅杞芥洿鏂板苟鎻愮ず閲嶅惎锛沗false` 鏃剁敱鐢ㄦ埛纭鏄惁鏇存柊 |

---

### 2. 鏂囦欢姣斿

娉ㄦ剰锛氭姝ラ璋冪敤鑰呭彲璺宠繃锛屽綋璋冪敤鑰呮娴嬪埌鏈夋柊鐗堟湰鏃讹紝鍙互鐩存帴璋冪敤绗?姝ヨ繘琛屾洿鏂般€傛姝ラ鍙槸涓轰簡璁╄璁″彲瑙嗗寲鏇存柊瀹㈡埛绔煡鐪嬪樊寮傛枃浠?
```bash
client-updator.exe check_diff [--url <url>] [--project-name <name>]
```

**璋冪敤鎺ュ彛**锛歚GET {url}/api/file/get_all_files/{project_id}`

**鍔熻兘**锛氬垪鍑洪渶瑕佷粠鏈嶅姟绔笅杞界殑鏂囦欢鍒楄〃銆傚彧杩斿洖鏈嶅姟绔瓨鍦ㄤ笖涓庢湰鍦?MD5涓嶅悓鐨勬枃浠讹紙鍖呮嫭鏈湴涓嶅瓨鍦ㄧ殑鏂囦欢锛夈€?*涓嶅垪鍑?*浠呮湰鍦板瓨鍦ㄣ€佹湇鍔＄涓嶅瓨鍦ㄧ殑鏂囦欢銆?
**杩斿洖鍊?*锛坰tdout JSON Array锛夛細

```json
{
	"isSuccess": true,
	"errorMsg": "",
	"data": {
		"new_version": "1.0.1",
		"files": [{
				"path": "app.exe",
				"local_md5": "abc123",
				"local_size": 43284,
				"local_sha256": "849384",
				"server_md5": "def456",
				"server_size": 5242880,
				"server_sha256": "884374"
			},
			{
				"path": "new.dll",
				"local_md5": "",
				"local_size": 0,
				"local_sha256": "",
				"server_md5": "xyz789",
				"server_size": 102400,
				"server_sha256": "88437486"
			}
		]
	}
}
```

**瀛楁璇存槑**锛?
| 瀛楁 | 绫诲瀷 | 璇存槑 |
| --- | --- | --- |
| `new_version` | string | 鏈嶅姟绔渶鏂扮増鏈彿 |
| `files` | array | 宸紓鏂囦欢鍒楄〃 |
| `files[].path` | string | 鏂囦欢鐩稿璺緞 |
| `files[].local_md5` | string | 鏈湴鏂囦欢 MD5锛坄new` 鏃朵负绌猴級 |
| files[].local_size | int | 鏈湴鏂囦欢澶у皬锛屼笉瀛樺湪鍒欎负0 |
| files[].local_sha256 | string | 鏈湴鏂囦欢鐨剆ha256,涓嶅瓨鍦ㄥ垯涓?" |
| `files[].server_md5` | string | 鏈嶅姟绔枃浠?MD5 |
| `files[].server_size` | int | 鏈嶅姟绔枃浠跺ぇ灏忥紙瀛楄妭锛?|
| files[].server_sha256 | string | 鏈嶅姟绔枃浠剁殑sha256锛屼笉瀛樺湪鍒欎负鈥濃€?|

---

### 3. 涓嬭浇鏇存柊

娉ㄦ剰锛氭姝ラ鍙笅杞芥洿鏂帮紝涓嶅鍒舵枃浠躲€傚綋鏈湴鏂囦欢澶筦./{main_exe_folder_name}_{new_version}`涓瓨鍦ㄨ涓嬭浇鐨勬枃浠讹紝md5鍜宻ha256鏍￠獙閫氳繃锛堝彲鑳芥槸浠ュ墠涓嬭浇浜嗛儴鍒嗘枃浠讹級锛屽垯璺宠繃姝ゆ枃浠朵笅杞姐€?
```bash
client-updator.exe download_update [--url <url>] [--project-name <name>] [--main-exe-path <path>]
```

**璋冪敤鎺ュ彛**锛?- `GET {url}/api/file/get_all_files/{project_id}` 鑾峰彇鏂囦欢鍒楄〃
- `GET {url}/api/file/download_file?path={file_path}` 閫愭枃浠朵笅杞?
**鎵ц姝ラ**锛?
1. 鑾峰彇鏈嶅姟绔枃浠惰鎯呭垪琛紙鏈嶅姟绔嚜鍔ㄥ拷鐣ラ厤缃腑鎺掗櫎鐨勬枃浠跺す鍜屾枃浠讹級

2. 瀵规瘮鏈嶅姟绔枃浠朵笌**鏈湴 ApplicationFolder** 涓枃浠剁殑 MD5 宸紓锛屽緱鍑洪渶瑕佷笅杞界殑鏂囦欢鍒楄〃锛坉iff 鍩哄噯涓哄綋鍓嶈繍琛岀殑 ApplicationFolder锛岃€岄潪 `ApplicationFolder_{version}` 鍗婃垚鍝佺洰褰曪級

3. 閬嶅巻宸紓鏂囦欢鍒楄〃锛岄€愭枃浠朵笅杞借嚦 `./{main_exe_folder_name}_{new_version}` 鏂囦欢澶逛腑(涓嶶pdateFolder鏄钩绾ф枃浠跺す)锛?   - **涓嬭浇鍓嶅厛妫€鏌ョ洰鏍囩洰褰?*锛氳嫢 `ApplicationFolder_{version}` 涓凡瀛樺湪鍚屽悕鏂囦欢涓?MD5/SHA-256 涓庢湇鍔＄涓€鑷达紝鍒欒烦杩囨鏂囦欢
   - **澶ф枃浠舵柇鐐圭画浼?*锛氭枃浠?> 100MB 鏃讹紝涓嬭浇鍓嶆鏌ョ洰鏍囩洰褰曟槸鍚﹀瓨鍦ㄥ悓鍚嶆枃浠舵垨 `.part` 涓存椂鏂囦欢
     - 鑻ュ瓨鍦ㄤ笖澶у皬 < 鏈嶅姟绔ぇ灏忥紝鍙戦€?`Range: bytes={size}-` 浠庢柇鐐圭户缁笅杞?     - 涓嬭浇涓啓鍏?`.part` 涓存椂鏂囦欢锛堝湪鐩爣鐩綍涓級锛屽畬鎴愬悗閲嶅懡鍚嶄负姝ｅ紡鏂囦欢鍚?   - 鏂囦欢 鈮?100MB 鐩存帴鍏ㄩ噺涓嬭浇
   
4. 涓嬭浇瀹屾垚鍚庨€愭枃浠舵牎楠?MD5 + SHA-256锛屼笌鏈嶅姟鍣ㄨ繑鍥炲€间笉涓€鑷村垯閲嶈瘯锛堟渶澶?3 娆★級锛屼粛鏈€氳繃鍒欎腑姝紝骞惰繑鍥為敊璇?
5. 鏇存柊 version.json锛?*浠呭湪 status 闈?`downloaded` 鏃舵墽琛岋紝闃叉閲嶅璋冪敤瑕嗙洊 `version_previouse`**锛夛細

   - `version_previouse` 璁句负褰撳墠 `version` 鐨勫€硷紙璁板綍鏈鏇存柊鍓嶇殑鐗堟湰鍙凤級
   - `version` 璁句负鏂扮増鏈彿锛堜粠鎺ュ彛涓幏鍙栵級
   - `version_status` 璁句负 `downloaded`

**杩斿洖鍊?*锛坰tdout JSON锛夛細

鎴愬姛锛?```json
{
	"isSuccess":true,
	"errorMsg":"",
	"data":{"version":"1.0.1"}
}
```

澶辫触锛?```json
{
	"isSuccess":false,
	"errorMsg":"app.exe 鏍￠獙澶辫触锛屽凡閲嶈瘯 3 娆?,
	"data":null
}
```

---

### 4. 搴旂敤鏇存柊

```bash
client-updator.exe apply_update [--main-exe-path <path>] [--must-close-process-name <name1,name2>] [--close-timeout <seconds>]
```

**鎵ц姝ラ**锛?
1. 璇诲彇 version.json锛屾鏌?`version_status`锛?   - 鑻ヤ负 `applied`锛氳繑鍥為敊璇紙娌℃湁寰呭簲鐢ㄧ殑鏇存柊锛?   - 鑻ヤ负 `applying`锛氳鏄庝笂娆″簲鐢ㄤ腑閫斿穿婧冿紝灏濊瘯鎭㈠鈥斺€旀鏌ュ綋鍓嶆枃浠剁郴缁熺姸鎬?     - 鑻?`ApplicationFolder`锛堜富绋嬪簭鐩綍锛夊瓨鍦ㄤ笖瀹屾暣锛氳鏄庢楠?3 灏氭湭鎵ц鍒伴噸鍛藉悕锛岄噸鏂版墽琛屾楠?3
     - 鑻?`ApplicationFolder` 涓嶅瓨鍦ㄤ絾 `ApplicationFolder_{version}` 瀛樺湪锛氳鏄庢棫鐩綍宸茶閲嶅懡鍚嶈蛋銆佹柊鐩綍灏氭湭鏀硅繃鏉ワ紝鐩存帴灏?`ApplicationFolder_{version}` 閲嶅懡鍚嶄负 `ApplicationFolder`锛岀劧鍚庢墽琛屾楠?4
   - 鑻ヤ负 `downloaded`锛氭甯告墽琛屽悗缁楠?
2. 灏?`version_status` 璁句负 `applying`锛堟爣璁板紑濮嬪簲鐢級

3. 鍏抽棴 `must_close_process_name` 涓垪鍑虹殑鎵€鏈夎繘绋嬶細
   - 鍏堝彂閫?`WM_CLOSE` 娑堟伅璇锋眰姝ｅ父閫€鍑?   - 绛夊緟瓒呮椂锛堥粯璁?30 绉掞紝鍙€氳繃 `--close-timeout` 閰嶇疆锛?   - 瓒呮椂鍚庤皟鐢?`TerminateProcess` 寮哄埗缁堟
   
4. **鍘熷瓙鏇挎崲**锛?
   - 灏?/{main_exe_relative_path_folder}鐨勫唴瀹瑰叏閮紙闄ゅ幓un_copy_files/un_copy_folders锛夊鍒跺埌`./{main_exe_folder_name}_{version}`涓紝纭繚`./{main_exe_folder_name}_{version}`涓槸涓€涓畬鏁村彲浠ヨ繍琛岀殑绋嬪簭

   - 灏?/{main_exe_folder_name}閲嶅懡鍚嶄负./{main_exe_folder_name}_{version_previouse}
   - 灏哷./{main_exe_folder_name}_{version}`閲嶅懡鍚嶄负./{main_exe_folder_name}

   **鑻ヤ换涓€姝ュけ璐?*锛氬皢 `version_status` 鍥炶涓?`downloaded`锛堝厑璁搁噸璇曪級锛岃繑鍥為敊璇€?
5. 鏇存柊 version.json锛歚version` 璁句负鏂扮増鏈彿锛宍version_previouse` 璁句负鏃х増鏈彿锛宍version_status` 璁句负 `applied`

6. 鎵ц `post_update_script`锛堝宸查厤缃級锛岃剼鏈け璐ヤ笉褰卞搷鏇存柊缁撴灉锛屼粎璁板綍鏃ュ織鍒?`update.log`

7. **鍚姩涓荤▼搴?*锛氫互 `main_exe_relative_path` 涓鸿矾寰勫惎鍔ㄤ富绋嬪簭 exe

**杩斿洖鍊?*锛坰tdout JSON锛夛細

鎴愬姛锛?```json
{
	"isSuccess":true,
	"errorMsg":"",
	"data":null
}
```

澶辫触锛?```json
{
	"isSuccess":false,
	"errorMsg":"鍘熷瓙鏇挎崲澶辫触锛屽凡鍥炴粴",
	"data":null
}
```

---

### 5. 鍒楀嚭鍙洖婊氱増鏈?
```bash
client-updator.exe list_rollback_versions [--main-exe-path <path>]
```

**鍔熻兘**锛氭壂鎻忔湰鍦?`{main_exe_folder_name}`鍚岀骇 鐩綍锛屽垪鍑烘墍鏈夊彲鐢ㄧ殑鍥炴粴鐗堟湰鍙枫€?
**杩斿洖鍊?*锛坰tdout JSON锛夛細

```json
{
	"isSuccess":true,
	"errorMsg":"",
	"data":{"current_version":"1.0.1","versions":["1.0.0","1.0.1"]}
}
```

---

### 6. 鐗堟湰鍥炴粴

```bash
client-updator.exe rollback --version 1.0.1 [--main-exe-path <path>] [--must-close-process-name <name1,name2>] [--close-timeout <seconds>]
```

**鎵ц姝ラ**锛?
鍚屾楠?锛堝簲鐢ㄦ洿鏂帮級涓€鏍锋搷浣?
**杩斿洖鍊?*锛坰tdout JSON锛夛細

鎴愬姛锛?```json
{
	"isSuccess":true,
	"errorMsg":"",
	"data":"version":"1.0.1"
}
```

澶辫触锛?```json
{
	"isSuccess":false,
	"errorMsg":"鍥炴粴鐩爣鐗堟湰鐩綍涓嶅瓨鍦?,
	"data":null
}
```

### 7. 鑷洿鏂版鏌?
```bash
client-updator.exe check_self_update [--main-exe-path <path>]
```

**鍔熻兘**锛氭瘮瀵硅嚜韬笌 ApplicationFolder/check-updator.exe 鐨?SHA-256锛屽垽鏂?updator 鏄惁闇€瑕佹洿鏂般€?
**鍘熺悊**锛氬彂甯冭€呮瘡娆℃墦鍖呮椂锛屽皢褰撳墠鐗堟湰鐨?client_updator.exe 澶嶅埗涓€浠芥斁鍏ヤ富绋嬪簭鐩綍锛屽懡鍚嶄负 check-updator.exe銆傚綋涓荤▼搴忔洿鏂板悗锛宑heck-updator.exe 鍙樻垚浜嗘柊鐗堬紝鑰?UpdateFolder/client_updator.exe 浠嶆槸鏃х増锛屾瘮瀵逛笉涓€鑷村嵆涓洪渶瑕佹洿鏂般€?
**琛屼负**锛?
- 鑻?ApplicationFolder/check-updator.exe 涓嶅瓨鍦紙棣栨閮ㄧ讲锛夛紝瑙嗕负涓嶉渶瑕佹洿鏂?- 鑻ュ瓨鍦ㄤ笖 SHA-256 涓€鑷达紝涓嶉渶瑕佹洿鏂?- 鑻ュ瓨鍦ㄤ笖 SHA-256 涓嶄竴鑷达紝闇€瑕佹洿鏂?
**杩斿洖鍊?*锛坰tdout JSON锛夛細

闇€瑕佹洿鏂帮細
```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": { "need_update": true }
}
```

涓嶉渶瑕佹洿鏂帮細
```json
{
  "isSuccess": true,
  "errorMsg": "",
  "data": { "need_update": false }
}
```

**璋冪敤鏂癸紙main app锛夊悗缁姩浣?*锛?
鏀跺埌 need_update: true 鍚庯紝灏?ApplicationFolder/check-updator.exe 澶嶅埗鍒?UpdateFolder/client_updator.exe 瑕嗙洊鏃х増锛堟鏃?updator 宸查€€鍑猴紝鏂囦欢閿侀噴鏀撅紝鍙畨鍏ㄨ鐩栵級銆?
---

## 鐩綍缁撴瀯绀轰緥

```
PackageFolder/
鈹溾攢鈹€ ApplicationFolder          #涓荤▼搴忔枃浠跺す
|    鈹斺攢鈹€ main_application.exe
|    鈹斺攢鈹€ check-updator.exe     #updator 鍓湰锛岀敤浜庤嚜鏇存柊姣斿
|    鈹斺攢鈹€ ...
鈹溾攢鈹€ UpdateFolder               #鏇存柊绋嬪簭鏂囦欢澶?|    鈹斺攢鈹€ client_updator.exe
|    鈹斺攢鈹€ version.json
|    鈹斺攢鈹€ client.yml
|鈹€鈹€ ApplicationFolder_0.0.1    #涓荤▼搴忔枃浠跺す 0.0.1鐗堟湰
|    鈹斺攢鈹€ main_application.exe
|    鈹斺攢鈹€ ...
|鈹€鈹€ ApplicationFolder_0.0.2    #涓荤▼搴忔枃浠跺す 0.0.2鐗堟湰
|    鈹斺攢鈹€ main_application.exe
|    鈹斺攢鈹€ ...
|鈹€鈹€ ApplicationFolder_0.0.3    #涓荤▼搴忔枃浠跺す 0.0.3鐗堟湰
|    鈹斺攢鈹€ main_application.exe
|    鈹斺攢鈹€ ...
```

---

## 寰呬紭鍖栭」 (Future Improvements)

浠ヤ笅涓鸿璁℃柟妗堣瘎瀹′腑璇嗗埆鐨勯棶棰橈紝褰撳墠鏆備笉澶勭悊锛岀暀寰呭悗缁凯浠ｄ紭鍖栥€?
### 鏂囦欢涓庣洰褰曠鐞?
| # | 闂 | 璇存槑 |
|---|------|------|
| 1 | 宸插垹闄ゆ枃浠舵棤娉曡鏇存柊鎺?| check_diff / download_update 鍙笅杞芥湇鍔＄鏈夎€屾湰鍦?MD5 涓嶅悓鐨勬枃浠讹紝涓嶅鐞嗘湇鍔＄宸插垹闄ゃ€佹湰鍦颁粛娈嬬暀鐨勬枃浠躲€傝繖浼氬鑷村簾寮?DLL / 閰嶇疆鏂囦欢涓€鐩存畫鐣欙紝鍙兘寮曞彂鐗堟湰鍐茬獊鎴栧畨鍏ㄦ紡娲?|
| 2 | 鏃犳棫鐗堟湰娓呯悊鏈哄埗 | list_rollback_versions 鍙兘鍒楀嚭锛屾病鏈夊搴旂殑鍒犻櫎/娓呯悊鍛戒护銆傞暱鏈熶娇鐢ㄥ悗 ApplicationFolder_0.0.x 浼氭棤闄愮Н绱紝娴垂纾佺洏绌洪棿 |
| 3 | 鍥炴粴鏃犳枃浠跺畬鏁存€ф牎楠?| rollback 鍙壂鎻忕洰褰曞悕锛屼笉鏍￠獙鐩爣鐩綍鍐呮枃浠剁殑 MD5/SHA-256銆傚鏋滅敤鎴锋墜鍔ㄥ垹闄や簡鏌愪釜鐗堟湰鐩綍涓殑鏂囦欢锛屽洖婊氬悗浼氬緱鍒颁竴涓畫缂虹殑搴旂敤 |
| 4 | 闈炵増鏈洰褰曞彲鑳借璇瘑鍒?| list_rollback_versions 鎵弿 ApplicationFolder_* 鏃讹紝濡傛灉鐢ㄦ埛鍒涘缓浜?ApplicationFolder_backup 绛夐潪鐗堟湰鐩綍锛屼篃浼氳褰撴垚鐗堟湰鍙峰垪鍑恒€傚簲鍔犵増鏈彿鏍煎紡鏍￠獙 |
| 5 | version.json 鍐欏叆闈炲師瀛?| 鐩存帴鍐欐枃浠惰€岄潪鍏堝啓涓存椂鏂囦欢鍐?rename锛屼腑閫斿穿婧冨彲鑳藉鑷存枃浠舵崯鍧忔垨涓虹┖ |

### 瀹夊叏涓庣鍚?
| # | 闂 | 璇存槑 |
|---|------|------|
| 6 | 绾?HTTP + 鏃犵鍚嶆牎楠?| 鍙牎楠?MD5/SHA-256锛屾棤鏁板瓧绛惧悕鎴栦唬鐮佺鍚嶃€傚鏋滄敾鍑昏€呮帶鍒朵簡鏈嶅姟鍣ㄦ垨瀹炴柦 MITM锛屽彲浠ュ悓鏃舵浛鎹㈡枃浠跺拰瀵瑰簲鐨勫搱甯屽€硷紝鏍￠獙褰㈠悓铏氳銆傚缓璁嚦灏戝姞 RSA/ECDSA 绛惧悕鏍￠獙锛堟湇鍔＄绉侀挜绛惧悕鏂囦欢鍝堝笇锛屽鎴风鍐呯疆鍏挜楠岀锛?|

### 杩涚▼涓庡苟鍙?
| # | 闂 | 璇存槑 |
|---|------|------|
| 7 | WM_CLOSE 瀵规帶鍒跺彴绋嬪簭鏃犳晥 | 鍙彂 WM_CLOSE 娑堟伅锛屽鏃犳秷鎭惊鐜殑鎺у埗鍙扮▼搴?/ Windows 鏈嶅姟鏃犳晥锛屽彧鑳界瓑瓒呮椂鍚庡己鏉€銆傚缓璁鎺у埗鍙拌繘绋嬩娇鐢?AttachConsole + CTRL_C_EVENT |
| 8 | 鏃犲苟鍙戦攣/浜掓枼鏈哄埗 | 娌℃湁鏂囦欢閿併€丮utex 绛夋満鍒堕槻姝㈠悓鏃惰繍琛屼袱涓?client_updator.exe 瀹炰緥锛屽彲鑳界牬鍧?version.json 鍜屾枃浠剁郴缁熺姸鎬?|
| 9 | 鍏抽棴杩涚▼鍚庡彲鑳借鐪嬮棬鐙楅噸鍚?| 鍏抽棴杩涚▼鍚庛€佹枃浠舵搷浣滃墠鐨勬椂闂寸獥鍙ｅ唴锛屽鏋滄湁瀹堟姢杩涚▼鎴栨湇鍔¤嚜鍔ㄩ噸鍚簡鐩爣杩涚▼锛屽悗缁噸鍛藉悕浼氬洜鏂囦欢琚崰鐢ㄨ€屽け璐?|

### 涓嬭浇瀹夊叏

| # | 闂 | 璇存槑 |
|---|------|------|
| 10 | 鏃犳枃浠跺ぇ灏?/ 鎬讳笅杞介噺涓婇檺 | 娌℃湁瀵瑰崟涓枃浠跺ぇ灏忓拰鎬讳笅杞介噺鍋氶檺鍒躲€傚鏋滄湇鍔＄琚叆渚佃繑鍥炰吉閫犵殑宸ㄥぇ鏂囦欢鍒楄〃锛屽鎴风纾佺洏浼氳鍐欐弧 |

### 鑷洿鏂?
| # | 闂 | 璇存槑 |
|---|------|------|
| ~~11~~ | ~~鏇存柊绋嬪簭鑷韩涓嶈兘鏇存柊~~ | 鉁?宸查€氳繃 `check_self_update` 鍛戒护瑙ｅ喅锛堣瑙佸姛鑳?7锛?|
