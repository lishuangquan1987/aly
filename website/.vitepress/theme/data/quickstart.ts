export type QuickStartGoal = 'publish' | 'integrate' | 'server'
export type QuickStartOs = 'windows' | 'linux' | 'macos'
export type QuickStartArch = 'amd64' | 'arm64'
export type QuickStartLang = 'csharp' | 'cli'

export interface QuickStartStep {
  title: string
  body?: string
  code?: string
  note?: string
  link?: { text: string; to: string }
}

export interface QuickStartCopy {
  kicker: string
  title: string
  lead: string
  goalLabel: string
  osLabel: string
  archLabel: string
  langLabel: string
  goals: Record<QuickStartGoal, string>
  oss: Record<QuickStartOs, string>
  archs: Record<QuickStartArch, string>
  langs: Record<QuickStartLang, string>
  docLink: Record<string, string>
  windowsXpNote: string
  macNote: string
}

const REPO = 'https://github.com/lishuangquan1987/aly'
const RELEASE = `${REPO}/releases/latest/download`

export const quickStartCopy: Record<'zh' | 'en', QuickStartCopy> = {
  zh: {
    kicker: '快速开始',
    title: '选系统与接入方式，三步跑通',
    lead: '选项会即时切换下面的命令。每一页都只保留最短可用路径，完整参数去对应模块查。',
    goalLabel: '我要做什么',
    osLabel: '操作系统',
    archLabel: '架构',
    langLabel: '宿主语言',
    goals: { publish: '发布版本', integrate: '接入更新器', server: '部署服务端' },
    oss: { windows: 'Windows', linux: 'Linux', macos: 'macOS' },
    archs: { amd64: 'x64 / amd64', arm64: 'arm64' },
    langs: { csharp: 'C# SDK', cli: '命令行 / 任意语言' },
    docLink: {
      publish: '/docs/publish',
      server: '/docs/server/deploy',
      csharp: '/docs/sdk',
      cli: '/docs/commands'
    },
    windowsXpNote: 'Windows 客户端是 32 位构建，兼容 Windows XP → Windows 11，64 位系统可直接运行。',
    macNote: 'macOS 二进制未签名，首次运行可能被 Gatekeeper 拦截，需在系统设置中允许。'
  },
  en: {
    kicker: 'Quick start',
    title: 'Pick your OS and integration path',
    lead: 'The commands below update instantly. Each path keeps only the shortest working route; the module reference has every flag and field.',
    goalLabel: 'What do you want to do?',
    osLabel: 'Operating system',
    archLabel: 'Architecture',
    langLabel: 'Host language',
    goals: { publish: 'Publish a release', integrate: 'Integrate the updater', server: 'Deploy the server' },
    oss: { windows: 'Windows', linux: 'Linux', macos: 'macOS' },
    archs: { amd64: 'x64 / amd64', arm64: 'arm64' },
    langs: { csharp: 'C# SDK', cli: 'CLI / any language' },
    docLink: {
      publish: '/docs/publish',
      server: '/docs/server/deploy',
      csharp: '/docs/sdk',
      cli: '/docs/commands'
    },
    windowsXpNote: 'The Windows client is a 32-bit build that runs from Windows XP through Windows 11, including 64-bit systems.',
    macNote: 'macOS binaries are unsigned. Gatekeeper may block the first run; allow it in System Settings.'
  }
}

function packageAsset(os: QuickStartOs, arch: QuickStartArch): string {
  // macOS 没有 GUI 安装包：按文档从 Linux 包里取 publish-cli / client 目录。
  if (os === 'windows') return 'AlyPublish-Setup.exe'
  return `AlyPublish-linux-${arch === 'arm64' ? 'arm64' : 'x64'}.zip`
}

/** Linux 包内没有顶层目录，解压时自己建一个。 */
function packageDir(arch: QuickStartArch): string {
  return `AlyPublish-linux-${arch === 'arm64' ? 'arm64' : 'x64'}`
}

function portableAsset(): string {
  return 'AlyPublish-win-x64.zip'
}

function serverAsset(os: QuickStartOs, arch: QuickStartArch): string {
  if (os === 'windows') return 'aly-server-windows-amd64.exe'
  const platform = os === 'macos' ? 'darwin' : 'linux'
  return `aly-server-${platform}-${arch}`
}

function psDownload(asset: string, out: string): string {
  return `Invoke-WebRequest -Uri "${RELEASE}/${asset}" -OutFile "${out}"`
}

function shDownload(asset: string, out: string): string {
  return `curl -fL -o ${out} "${RELEASE}/${asset}"\nchmod +x ${out}`
}

function publishBinary(os: QuickStartOs, arch: QuickStartArch): string {
  const rid = os === 'windows' ? 'windows-amd64' : `linux-${arch === 'arm64' ? 'arm64' : 'amd64'}`
  return os === 'windows' ? `.\\publish-cli\\${rid}\\aly-publish.exe` : `./publish-cli/${rid}/aly-publish`
}

function serverBinary(os: QuickStartOs): string {
  return os === 'windows' ? '.\\aly-server.exe' : './aly-server'
}

function clientBinary(os: QuickStartOs): string {
  // 更新器已放进 UpdateFolder/，命令在该目录内执行。
  return os === 'windows' ? '.\\aly-client.exe' : './aly-client'
}

function buildPublishSteps(isEn: boolean, os: QuickStartOs, arch: QuickStartArch): QuickStartStep[] {
  const asset = packageAsset(os, arch)
  const dir = packageDir(arch)
  const bin = publishBinary(os, arch)
  const copy = quickStartCopy[isEn ? 'en' : 'zh']
  return [
    {
      title: isEn ? 'Install AlyPublish (one package)' : '安装 AlyPublish（一个包）',
      body: isEn
        ? 'The installer carries the GUI, the publish CLI and the terminal updater. Windows: run the setup, or unzip the portable zip. Linux: unzip the package and run install.sh.'
        : '安装包里同时有发布 GUI、发布 CLI 与终端更新器。Windows：运行安装包，或解压便携 zip；Linux：解压后运行 install.sh。',
      code:
        os === 'windows'
          ? `${psDownload(asset, 'AlyPublish-Setup.exe')}\nStart-Process ".\\AlyPublish-Setup.exe"`
          : `curl -fL -o ${asset} "${RELEASE}/${asset}"\nmkdir -p ${dir}\nunzip -q ${asset} -d ${dir}       # zip 内没有顶层目录\ncd ${dir}\nchmod +x install.sh\n./install.sh`,
      note:
        os === 'macos'
          ? copy.macNote
          : isEn
            ? 'After install the folder holds AlyPublish, payload.json, publish-cli/<rid>/ and client/<rid>/.'
            : '装好后目录里有 AlyPublish、payload.json、publish-cli/<rid>/ 与 client/<rid>/。',
      link: { text: isEn ? 'Download & install' : '下载与安装', to: '/docs/download' }
    },
    {
      title: isEn ? 'Initialize the project in the GUI' : '在 GUI 里初始化项目',
      body: isEn
        ? 'Start AlyPublish → “Initialize project” → pick the server project and the local release folder → set the main executable. The GUI writes .updator/, creates the sibling UpdateFolder/ and copies the packaged updater into it.'
        : '启动 AlyPublish → “初始化项目” → 选服务端项目与本地产物目录 → 填主程序。GUI 会写好 .updator/、在同级建出 UpdateFolder/ 并把包内更新器复制进去。',
      code: 'AlyPublish(.exe)',
      note: isEn
        ? 'The server must already be running. A new server project starts at V1.0.0, so the first push must be higher.'
        : '服务端需已启动；新服务端项目初始版本是 V1.0.0，所以首次发布必须更高。',
      link: { text: isEn ? 'Publish GUI' : '发布 GUI', to: '/docs/publish/gui' }
    },
    {
      title: isEn ? 'Stage and publish' : '暂存并发布',
      body: isEn
        ? 'Refresh → “Stage all” → fill the version and the change note → Publish. The same steps through the packaged CLI:'
        : '刷新 → “全部暂存” → 填版本号与变更说明 → 发布。等价的命令行（用包内 CLI）：',
      code: `${bin} config init --server http://127.0.0.1:2000 --project myapp\n${bin} project create --server http://127.0.0.1:2000 --name myapp --title "My App"\n${bin} status\n${bin} add --all\n${bin} push --version V1.0.1 --message "${isEn ? 'First release' : '首个版本'}"`,
      note: isEn
        ? 'Every later release repeats status → add → push. Large files use chunked upload automatically.'
        : '之后每次发版重复 status → add → push 即可；大文件会自动走分片上传。',
      link: { text: isEn ? 'Publish commands' : '发布命令参考', to: '/docs/publish' }
    }
  ]
}

function buildServerSteps(isEn: boolean, os: QuickStartOs, arch: QuickStartArch): QuickStartStep[] {
  const asset = serverAsset(os, arch)
  const bin = serverBinary(os)
  const publishBin = publishBinary(os, arch)
  return [
    {
      title: isEn ? 'Download the server' : '下载服务端',
      body: isEn
        ? 'One statically linked binary. No runtime, no external database service.'
        : '静态编译的单文件，不需要运行时或外部数据库服务。',
      code: os === 'windows' ? psDownload(asset, 'aly-server.exe') : shDownload(asset, 'aly-server'),
      note: os === 'macos' ? quickStartCopy[isEn ? 'en' : 'zh'].macNote : undefined,
      link: { text: isEn ? 'Download matrix' : '完整下载矩阵', to: '/docs/download' }
    },
    {
      title: isEn ? 'Start it' : '启动服务端',
      body: isEn
        ? 'The first run creates `aly.db` and `data/` next to the executable. Use `-db` to move the database file.'
        : '首次运行在程序旁创建 `aly.db` 与 `data/`；`-db` 可以改数据库文件路径。',
      code: `${bin} -p 2000`,
      note: isEn
        ? 'Uploaded files always live in `data/{project}` next to the executable. Back up both the database and that directory.'
        : '上传文件固定放在程序旁的 `data/{project}`；备份时数据库与 `data/` 都要覆盖。',
      link: { text: isEn ? 'Deployment guide' : '部署指南', to: '/docs/server/deploy' }
    },
    {
      title: isEn ? 'Create a project and publish' : '创建项目并发布首个版本',
      body: isEn
        ? 'In your release directory, initialize the config, create the server project, then push the first version.'
        : '在产物目录里初始化配置、在服务端创建同名项目，然后推送第一个版本。',
      code: `${publishBin} config init --server http://127.0.0.1:2000 --project myapp\n${publishBin} project create --server http://127.0.0.1:2000 --name myapp --title "My App"\n${publishBin} add --all\n${publishBin} push --version V1.0.1 --message "${isEn ? 'First release' : '首个版本'}"`,
      note: isEn
        ? '`config init` writes the client-side config; `project create` registers the project on the server.'
        : '`config init` 写客户端配置，`project create` 在服务端注册项目，两者项目名要一致。',
      link: { text: isEn ? 'Publish workflow' : '发布工作流', to: '/docs/publish' }
    }
  ]
}

function buildIntegrateCSharpSteps(isEn: boolean, os: QuickStartOs): QuickStartStep[] {
  return [
    {
      title: isEn ? 'Reference the SDK and take the updater from the package' : '引用 SDK，并从安装包取更新器',
      body: isEn
        ? 'The SDK targets net40 and netstandard2.0 and is not on the public NuGet feed yet, so use a project reference. The updater is no longer downloaded by the SDK: the AlyPublish GUI writes UpdateFolder/ (or copy client/<rid>/ out of the package by hand).'
        : 'SDK 同时支持 net40 与 netstandard2.0，当前未上架公共 NuGet，用项目引用即可。更新器不再由 SDK 下载：AlyPublish GUI 会生成 UpdateFolder/，也可以从包里直接取 client/<rid>/ 手工放。',
      code: `dotnet add reference client/aly-client-sdk/AlyClient.CSharpSDK/AlyClient.CSharpSDK/AlyClient.CSharpSDK.csproj`,
      note: isEn
        ? 'The SDK targets only create ../UpdateFolder/ and a default client.json now; the binaries come from the package.'
        : 'SDK 的 targets 现在只建 ../UpdateFolder/ 与默认 client.json，二进制来自安装包。',
      link: { text: isEn ? 'SDK integration' : 'SDK 集成指南', to: '/docs/sdk' }
    },
    {
      title: isEn ? 'Write client.json' : '填写 client.json',
      body: isEn
        ? '`main_exe_relative_path` is resolved relative to `UpdateFolder/`, not the package root.'
        : '`main_exe_relative_path` 相对于 `UpdateFolder/` 解析，不是包根目录。',
      code: `{
  "main_exe_relative_path": "../ApplicationFolder/YourApp.exe",
  "must_close_process_name": ["YourApp"]
}`,
      note: isEn
        ? 'Put the server URL and project name in `ApplicationFolder/.updator/shared.json`. `apply_update` restarts the executable above.'
        : '服务端地址与项目名写在 `ApplicationFolder/.updator/shared.json`；`apply_update` 完成后会拉起上面的主程序。',
      link: { text: isEn ? 'Client configuration' : '客户端配置参考', to: '/docs/client/config' }
    },
    {
      title: isEn ? 'Launch the updater UI at startup' : '在宿主启动时拉起更新窗口',
      code: `using AlyClient.CSharpSDK;

var ui = new AlyUpdateUIClient(); // ..\\UpdateFolder\\aly-client-ui.exe
ui.LaunchAuto();                  // silent unless an update is available`,
      note: isEn
        ? 'This needs `aly-client.exe` and `aly-client-ui.exe` in the same folder. For a custom in-app UI, use `AlyUpdateClient` and its events instead.'
        : '这要求 `aly-client.exe` 与 `aly-client-ui.exe` 同目录。若要自绘更新 UI，改用 `AlyUpdateClient` 的事件循环。',
      link: { text: isEn ? 'SDK API reference' : 'SDK API 参考', to: '/docs/sdk/api' }
    }
  ]
}

function buildIntegrateCliSteps(isEn: boolean, os: QuickStartOs, arch: QuickStartArch): QuickStartStep[] {
  const bin = clientBinary(os)
  const rid = os === 'windows' ? 'windows-x86' : `linux-${arch === 'arm64' ? 'arm64' : 'amd64'}`
  const pkg = os === 'windows' ? portableAsset() : packageAsset(os, arch)
  const dir = packageDir(arch)
  // 更新器不再作为独立 Release 资产发布：从 AlyPublish 包的 client/<rid>/ 里取。
  const download =
    os === 'windows'
      ? `# 更新器在 AlyPublish 便携包的 client\\windows-x86\\ 里\n${psDownload(pkg, portableAsset())}\nExpand-Archive -Path ${portableAsset()} -DestinationPath AlyPublish -Force\nCopy-Item "AlyPublish\\client\\windows-x86\\aly-client*.exe" "UpdateFolder\\" -Force`
      : `# 更新器在 AlyPublish 包的 client/${rid}/ 里\ncurl -fL -o ${pkg} "${RELEASE}/${pkg}"\nmkdir -p ${dir}\nunzip -q ${pkg} -d ${dir}\ncp ${dir}/client/${rid}/aly-client* UpdateFolder/\nchmod 0755 UpdateFolder/aly-client*`
  return [
    {
      title: isEn ? 'Take the updater from the AlyPublish package' : '从 AlyPublish 包里取更新器',
      body: isEn
        ? `Standalone updater binaries are no longer published as Release assets. Download the AlyPublish package and copy both files from \`client/${rid}/\` into \`UpdateFolder/\`. The Windows build is 32-bit and XP-compatible; the Linux build is arch-specific.`
        : `更新器不再作为独立 Release 资产发布。下载 AlyPublish 安装包，把 \`client/${rid}/\` 里的两个文件复制进 \`UpdateFolder/\`。Windows 构建为 32 位、兼容 XP；Linux 按架构选择。`,
      code: download,
      note: isEn ? quickStartCopy.en.windowsXpNote : quickStartCopy.zh.windowsXpNote,
      link: { text: isEn ? 'Download matrix' : '完整下载矩阵', to: '/docs/download' }
    },
    {
      title: isEn ? 'Create the package layout' : '准备包目录与配置',
      body: isEn
        ? '`UpdateFolder/` holds the updater and `client.json`; `ApplicationFolder/` holds your app and `.updator/shared.json`.'
        : '`UpdateFolder/` 放更新器与 `client.json`；`ApplicationFolder/` 放应用本体与 `.updator/shared.json`。',
      code: `{
  "main_exe_relative_path": "../ApplicationFolder/YourApp.exe",
  "must_close_process_name": ["YourApp"]
}`,
      note: isEn
        ? 'Also set `server_url` and `project_name` in `ApplicationFolder/.updator/shared.json`.'
        : '同时在 `ApplicationFolder/.updator/shared.json` 写入 `server_url` 与 `project_name`。',
      link: { text: isEn ? 'Client configuration' : '客户端配置参考', to: '/docs/client/config' }
    },
    {
      title: isEn ? 'Run the three commands' : '调用三条命令',
      code: `${bin} check_update\n${bin} download_update\n${bin} apply_update`,
      note: isEn
        ? 'Run these inside `UpdateFolder/`. A business failure can still exit 0; rely on `isSuccess` in the JSON. `apply_update` closes the listed processes, replaces the folder and restarts the app.'
        : '在 `UpdateFolder/` 目录里执行。业务失败仍可能退出码 0，请以 JSON 的 `isSuccess` 为准。`apply_update` 会关闭列出的进程、替换目录并拉起主程序。',
      link: { text: isEn ? 'Command reference' : '命令参考', to: '/docs/commands' }
    }
  ]
}

export function buildQuickStartSteps(
  isEn: boolean,
  goal: QuickStartGoal,
  os: QuickStartOs,
  arch: QuickStartArch,
  hostLang: QuickStartLang
): QuickStartStep[] {
  if (goal === 'publish') return buildPublishSteps(isEn, os, arch)
  if (goal === 'server') return buildServerSteps(isEn, os, arch)
  return hostLang === 'csharp' ? buildIntegrateCSharpSteps(isEn, os) : buildIntegrateCliSteps(isEn, os, arch)
}