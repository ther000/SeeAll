# SeeAll

竖版托盘面板，用来监视**万物**：现在自带鼠标电量和本机状态，多一种监控对象只是多一个插件。

用 [MyGo](https://mygo.egoist.dev/) 重写，原生 UI 直接画在 GPU 上：没有 webview、没有前端构建，一个 Go 二进制。

![鼠标页](docs/panel-mouse.png)

## 理念：一切皆插件

面板对每个监控对象是同一块：顶部一排标签选监控项，下面是它的当前值、读数列表和刷新时间。

新增一种「可以监视的东西」= 写一个实现 `plugin.Plugin` 的包，放进 `internal/app/builtin.go` 的切片里。
列表、设置页、持久化、托盘、通知全都会自动出现，不需要改 UI。

自带两个插件：

| 插件 | 监视什么 | 状态 |
| --- | --- | --- |
| 鼠标 | 雷柏 2.4G 鼠标的电量、连接方式、DPI（读状态广播帧），设备名取自接收器的产品字符串 | 已在真实硬件上验证 |
| 本机 | CPU、内存、磁盘剩余、电池、开机时长 | 可用 |

## 用法

```powershell
go tool mygo build        # 打包：build/windows-amd64/SeeAll.exe + 安装包
go tool mygo dev          # 开发模式，改 .go 文件自动重启
go build -o seeall.exe .  # 快速编译（带控制台，会自动隐藏）
```

已经打好的产物：

| 文件 | 用途 |
| --- | --- |
| `SeeAll.exe`（仓库根目录） | 双击即用，免安装 |
| `build/windows-amd64/SeeAll.exe` | 同上，打包目录里的原件 |
| `build/windows-amd64/SeeAll Setup 2.0.0 amd64.exe` | 安装包：开始菜单、卸载项、开机自启 |

> **关于终端窗口**：`go build` 默认产出控制台程序，双击会带一个黑窗口。
> SeeAll 启动时会把**只属于自己的**那个控制台藏起来（`internal/console`）；
> 如果是从 cmd / PowerShell 起的，控制台是 shell 的，就不动它。
> `go tool mygo build` 打出来的 exe 加了 `-H=windowsgui`，本来就没有控制台。

- 左键托盘图标开关面板，右键是监控项菜单（刷新、设置、打开日志、退出都在那里）。
- 首次运行会打开面板，并把开机自启打开（设置页可关）。
- 配置在 `%AppData%\SeeAll\seeall.json`，日志在 `%AppData%\SeeAll\logs\seeall.log`。
- 设置页可以移除监控项；移除的那一项用「添加」加回来，设置照旧（移除时的取值存在配置文件的 `removed` 里）。

快捷键：`Ctrl+R` 或 `F5` 刷新全部，`Esc` 收起面板。

## 面板

面板停在**屏幕右下角**，离边缘和任务栏各留 8 **像素**，像一块浮起来的卡片，而不是焊在任务栏上。位置由 `tray.Inset` + `tray.Dock` 算：工作区（本身已排除任务栏）先内缩边距，再右下对齐；换显示器或改缩放会重算。

面板是**一列竖版，尺寸是 660×840 物理像素**——设计稿的单位，不是 DIP。窗口按显示器的缩放换算成 DIP 去要尺寸：

```go
const DesignWidth, DesignHeight = 660, 840   // 像素
Width  = round(660 / scale)                  // DIP：1x→660，1.5x→440，2x→330
```

这样它在任何缩放比例下都占同样多的屏幕像素：

| 屏幕 | 窗口（DIP） | 实际占用 |
| --- | --- | --- |
| 2560×1440 @150%（工作区 1706×912 DIP） | 440 × 560 | 660 × 840 px |
| 1920×1080 @100% | 660 × 840 | 660 × 840 px |
| 4K @200% | 330 × 420 | 660 × 840 px |

放不下（工作区比它小）时按高度等比缩，宽度再兜一次底；屏幕再大也不跟着长——托盘面板不该铺满屏幕。

尺寸一次算定，切页/关掉重开都不变；内容装不下时页内滚动。`internal/panel/fit_test.go` 会把每个页面按桌面和笔记本两种尺寸渲染一遍，检查标题栏、标签页、页面三层的相对位置和边界；另外三条测试盯着「像素换算成 DIP」这件容易写反的事（150% 下 660×840 px 必须等于 440×560 DIP）。

从上到下三层：

- **标题栏**：应用名 + 刷新 + 设置。
- **标签页**：每个监控项一个，等宽铺满，选中项是白底描边的卡片；状态不是「正常」时标签上是彩点，因为一次只看得到一页。
- **页面**：默认每个读数一张同样大小的卡，两张一行；卡里标签在左上、趋势折线在右上、值在中间、说明在下面。插件标出的主读数排第一张，值用大一号的字号。插件声明 `Combined` 时整页只有一张卡：设备名横在顶部，左边是电量环，右边是连接与 DPI 各占一行（鼠标就是这样——电量、连接、DPI 是同一只鼠标的三件事）。合卡页不画折线，也不写「电量充足」这类说明，环和数字已经说完了。读数多了整页滚动。

页面里没有状态栏，也没有日志和退出按钮：托盘右键菜单里两样都有，面板只管看。

托盘图标按电量画环：数字居中、外面一圈按百分比点亮（>30% 青绿 / 11–30% 琥珀 / ≤10% 红；离线画灰环，没数据画三个点）。深浅色任务栏各用一套墨色。

![托盘图标各状态](docs/tray-icon.png)

## 结构

```
main.go                     进程入口：单实例、托盘、面板窗口、自启
internal/console/           藏掉自定义的控制台窗口（只有 Windows 有这问题）
internal/core/              读数模型：Reading / Snapshot / Severity / 环形历史 / 单位格式化
internal/plugin/            插件契约：Plugin / Instance / Manifest / Env / Settings
internal/hid/               Windows HID 枚举与只读接收（无 cgo）；其他平台是空实现
internal/plugins/mouse/     雷柏 2.4G 协议（frame.go 里有逐字节的帧格式）
internal/plugins/system/    CPU / 内存 / 磁盘 / 电池 / 开机时长
internal/app/               宿主：设置落盘、实例生命周期、通知
internal/panel/             面板视图（原生 UI，Go 写）
internal/tray/              托盘图标与面板定位
cmd/preview/                不用窗口把各页渲染成 PNG（文档图就是这么来的）
cmd/icon/                   生成 resources/icon.png
cmd/diag/                   HID 列设备与抓包（协议验证用）
tools/shot.ps1              截活动窗口（开发核对用）
```

架构约定见 [docs/design.md](docs/design.md)。

## 诊断 HID

```powershell
go run ./cmd/diag -list                    # 列出所有 HID collection，* 标记电量通道
go run ./cmd/diag -capture -seconds 10     # 同时监听所有电量通道，逐帧打印并解析
```

雷柏的状态帧（7 字节，实测 VT9 PRO）：

```
BB B0 42 B0 04 01 5F
│  │  │  └─┬─┘ │  └─ 电量百分比 0..100
│  │  │    │   └──── 连接标志：0x01 无线 / 0x02 有线
│  │  │    └──────── 当前 DPI（u16 LE，本机唯一可信的 DPI 来源）
│  │  └───────────── 机型 / 通道标识
│  └──────────────── 帧类型：状态广播
└─────────────────── report ID
```

## 重新生成文档图

```powershell
go run ./cmd/preview -out docs      # 各页的 PNG
go run ./cmd/icon                   # 应用图标
go test ./internal/tray -run TestIconSheet   # .preview/probe-icons.png，托盘各状态
```

## 测试

```powershell
go test ./...
```

面板的三个布局测试挡着老项目踩过的坑：设置页必须从顶部开始（长内容靠滚动，不是把标题推出去）、详情页要用插件指定的主读数、点侧栏能切页。

## 已知边界

- **雷柏接收器没有读回通道**：除状态帧里的 DPI，写进去的参数没法确认。SeeAll 这一版只读不写，不显示任何「你以为写进去了」的状态。
- HID 读取目前只有 Windows 实现；其他平台能编译，鼠标插件会报告没有通道。