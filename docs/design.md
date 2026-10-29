# SeeAll 设计与接口约定

SeeAll 是「鼠标电量显示」的完全重写：同一个托盘小工具，但内核换成了插件化监控。
用 Go + [MyGo](https://mygo.egoist.dev/)（原生 UI，无 webview、无前端构建）。

## 一句话

托盘点开一块竖版面板：顶部标签选**监控对象**（插件的实例），下面是它的当前值和读数列表；标题栏的齿轮进配置页。
新增一种「可监控的东西」= 写一个实现 `plugin.Plugin` 的包，加进 `internal/app/builtin.go` 的切片。

## 分层

```
core    ← 与插件无关的读数模型（Reading / Snapshot / Status / Severity / Ring / 单位格式化）
plugin  ← 插件契约（Plugin / Instance / Manifest / Env / Settings），只依赖 core
plugins/* ← 具体插件（mouse、system），依赖 core + plugin + hid（需要时）
app     ← 宿主：加载/保存设置、创建与停止实例、快照与历史，只依赖 core + plugin + plugins
panel   ← 视图，依赖 app + core + plugin + mygo/ui
tray    ← 图标绘制与面板定位，只依赖 mygo（无 UI 依赖）
main    ← 组装：窗口、托盘、菜单、自启
```

依赖方向单一，插件不知道面板存在，面板不知道 HID 存在。

## 插件契约

```go
type Plugin interface {
	Manifest() Manifest
	New(env *Env) Instance
}

type Instance interface {
	Start()          // 起 goroutine，立刻返回
	Stop()           // 停干净
	Poll()           // 立刻取一次快照，任意 goroutine 可调
}
```

`Env` 是插件能用的全部：`Settings()` 读自己那几项、`Publish(core.Snapshot)` 上报、
`Logf()` 写日志。
插件不碰 UI、不碰文件路径、不管生命周期——取消也是：由 `Start()` 的 goroutine
自己看退出信号，宿主只调 `Instance.Stop()`。

`Manifest` 声明：ID、显示名、一句话说明、分组（设备/本机/服务）、图标、
设置字段（`Setting{Key,Label,Type,Default,...}`）、默认实例名，
以及 `Combined`——这个插件的读数描述同一个东西，页面用一张卡装下（鼠标：电量/连接/DPI）。

**Manifest 只放有人在读的字段。** 采样频率写在各插件的 `Start()` 里（宿主没有定时器），
「未验证协议」的标记在它真的有一个未验证协议时才加回来。

设置页完全由 `Manifest.Settings` 生成：加一个字段就多一个控件，UI 代码不用动。
反过来也成立：**一个没有任何设置字段的插件不在这页上出卡片**——这页的每一条都是能改的东西，
一个只写着自己名字和一句话的卡片不是设置。

## 宿主

```go
host := app.NewHost(dataDir)   // 带内置插件
host.Load()                    // 读 seeall.json；没有就按默认插件各建一个实例
host.AdoptLegacySettings()     // 只有真程序调：接管旧版留在 %AppData% 的设置
host.Instances() []*Instance   // 侧栏顺序
host.Instance(id) *Instance
host.Refresh(); host.RefreshOne(id)
host.Add(pluginID, name, values) *Instance
host.Remove(id)
host.StartAtLogin() bool; host.SetStartAtLogin(bool)
host.LastInstance() string; host.SetLastInstance(id)
host.Inject(id, snap) bool; host.InjectHistory(id, reading, values)
```

`Instance` 对外只有 `Snapshot()`、`History(readingID)`、`Manifest()`、`Settings()`。
快照由 `Env.Publish` 写进来，宿主顺手把它塞进每项读数的环形历史（120 点），面板的小折线就是这么来的。

并发约定：插件的 goroutine 只调 `Publish`；宿主用 `OnChange` 通知界面重绘；
界面只在主线程读。`App.View` 不写任何跨线程状态。

## 读数模型

```go
type Reading struct {
	ID, Label string
	Value     float64
	Unit      Unit      // %、B、bit/s、ms、s，或任意 Unit("W")
	Text      string    // 有它就显示文字，不显示数字
	Severity  Severity  // none / ok / info / warn / critical / offline
	Badge     string    // 卡片标题右边的短标签：「2.24 GHz」「C: · 159 GB」
	Detail    string    // 副行：「2 秒前」「2.1 GB / 5.9 GB」
	At        time.Time
}
```

`Snapshot.Primary` 指定「这页的大数字用哪项」——电量页要环、CPU 页要百分比。
不指定就退化成「最严重的那项」。`Snapshot.Title` 是设备的真名，鼠标取自接收器的产品字符串
（`HidD_GetProductString`），不是按 VID 猜的。

仪表盘还要两种读数模型之外的**关于机器而不是测量值**的东西，它们同样由插件发布，
面板不猜：

```go
type Fact struct{ Label, Value string }   // 头部胶囊；Text() 把两段拼成一句
type Process struct{ Name string; Mem, CPU float64 } // Mem 字节，CPU 整机百分比

Snapshot.Health      int    // 0–100，0 = 这个插件不打分
Snapshot.HealthNote  string // 「很好」
Snapshot.Facts       []Fact
Snapshot.Processes   []Process
```

**读数 ID 是插件与面板的契约**：面板把 `cpu/gpu/mem/disk/net/temp` 摆成六张卡，
`vram` 并进 GPU 的说明行，`uptime` 只进头部的胶囊，`battery` 与每个设备各占一张同样大小的卡；
不认识的读数兜底成同一套卡片，追加在六张后面。`Badge` 是卡片的标题行右边那个小标签，
写数字说不出来的那半句（频率、型号、卷标、健康度）。

**Health 由插件算，不由面板算**：什么算「健康」是那个插件的事（鼠标拿电量当分数，
本机按 CPU/内存/GPU/温度/磁盘/电池各自的免罚区间扣分），读不到的维度不扣分。
`0` 是保留值：面板画设备名而不是一个假分数。

**Processes 按程序名在插件里合并**：浏览器是十几个进程，五行 chrome 不如一行「chrome」。

## 面板

停在**屏幕右下角，离边缘和任务栏各 8 像素**——`tray.Inset(work, marginDIPs)` 把工作区
内缩，`tray.Dock(area, w, h)` 再右下对齐。工作区本身已排除任务栏，边距是额外留的浮起感。

**尺寸的单位是物理像素，不是 DIP。** 设计稿画的是 660×1110 px，而窗口只收 DIP，
所以 `panel.Fit` 和 `panel.MarginDIP` 都要显示器的 `ScaleFactor`：

```go
const DesignWidth, DesignHeight = 660, 1110  // 像素
Width  = round(660 / scale)                  // DIP：1x→660，1.5x→440，2x→330
Height = round(1110 / scale)
```

这样面板在任何缩放下都占同样多的屏幕像素。写反了（把 660×1110 当 DIP）在 150% 的机器上
会变成 990×1665 px，大一倍半——`fit_test.go` 有一条测试专门钉这个换算。

放不下时按高度等比缩，宽度再兜一次底。屏幕再大也不跟着长——托盘面板不该铺满屏幕。

一次算定，关掉重开都不变；内容装不下时页内滚动。**这是从老项目继承的硬约束**：
老版本用 `SizeToContent` 让窗口贴着内容长，结果设置页一开就长出屏幕外。
现在页面容器是 `Grow(1) + Shrink(1) + Basis(0)`，页头永远在顶部。

**1110 这个高度是量出来的**：150% 下面板是 440×740 DIP，整页（头、四行卡、进程卡、
16 的底部留白）量到 728 DIP，留 12 DIP 余量。卡高一改这条就破，`dashboard_test.go`
在三个窗口尺寸下断言所有内容都在窗口内。

### 一页

面板只有一套外观，也**只有一页**：`Selected` 只在「是不是设置页」上有意义，其余时候画的是
所有监控项的聚合页。自上而下：画布（左暖紫到右蓝的渐变 + 页脚一层暖光，`Painter.FillGradient`
直接画）→ 头（天气、退成胶囊的健康分、`Facts` 胶囊，右上角刷新与设置两个图标按钮）→ 卡片（机器六张
两列，电池与每个设备各一张，同样大小）→ 进程卡 → 16 的底部留白。没有页脚栏，也没有标签页。

面板每帧把 `ui.Theme` 改成深色（`dashFor`），控件、滚动条、开关都跟着深——它是设计成深色的
面板，不跟随系统外观。

**卡片一律同尺寸**（`dashCardH`，两列）。电池和鼠标不是横条也不是环，而是和 CPU 一样的一张卡：
数字是电量，图形是进度条。设备卡多一处别人没有的位置：**数字那行的另一端**是设备的另一个
数字——鼠标的 DPI 就摆在那儿，读作电量的属性；最下面一行只写设备名（`Rapoo Gaming Device`）。
`dashDeviceLines` 把设备的多条读数分成这两处，不止一种数字时先来的进右侧、其余还留在说明行。
机器与设备的分界是 `Manifest().Combined`，**不是快照里有没有 Health**——鼠标插件也发 Health，
按快照特征判会把鼠标当成机器，整页反过来。设备掉线时卡还在：数字「—」、徽标「未连接」。

徽标（`Reading.Badge`）是插件告诉卡片写什么的地方：GPU 写型号（面板不画温度，型号才是认得出
那张卡的东西），CPU 写频率，磁盘写卷标，电池写健康度。面板给徽标留了上限宽度（半张卡），
型号再长也压不掉标题；放不下的部分**按词收尾**（`fitBadge`：「RTX 3060…」而不是
「RTX 3060 La…」），原文留在 `Description` 里给读屏。

**放不下的单行文字，收尾靠 `MaxLines(1)`，不能加 `NoWrap`。** MyGo 的 shaper 遇到 `NoWrap`
会把排版宽度当成 0，一行照原样排完，于是 `MaxWidth` 只夹住了盒子、字形照样画到盒子外——
胶囊盖在隔壁胶囊上、型号压过卡片边缘，数值断言全绿，画面上却是坏的。所以这里的规矩是：
**可能被夹窄的文字一律 `MaxLines(1)` + `MaxWidth`，`NoWrap` 只留给不可能变窄的**（大数字、
单位、`设置` 这种标题）。头部的四个胶囊同理：一行放不下时按「水填」分配宽度（`factCaps`，
短的先按原样站住，剩下的平分给长的），再靠 `MaxLines(1)` 收尾。

头在 `Health == 0` 时改画机器名——一个不打分的数据源不该顶着一个假分数。进程卡没有数据就整块不出现。

### 天气

头顶那个位置本来只归健康分，现在归天气：`Snapshot.Weather` 非 nil 时，左边画天气的图形、温度和一句
「天气 · 城市」（`多云 · 重庆`，和原来的「太阳 + 大数字 + 评语」同一个节奏，说的是地方而不是机器）；
健康分退到第一行右端，做成和 `Facts` 一样的胶囊（`90 很好`，绿色描字）。`Weather` 为 nil 时画面回到
原样：大评分 + 评语。这和 `Health == 0` 改画机器名是同一条链：有什么画什么，都没有就退回下一档。

天气和城市是一个 `ui.RichText`、两个 `ui.Span`：天气第二档颜色，城市第三档（`dashNoteSize` 是同一个
字号），中间自己带分隔符——缺哪个就不拼哪个，不会留一个孤零零的「·」。城市不另起一行：头部没有第二行
的高度，多一行就要从整页那 12 DIP 余量里挤。取数时间放 tooltip（`天气 13:41 更新`）：头上没有地方再
放它，它每次刷新都在变。

**评分胶囊不在 `Facts` 那一行。** 四颗胶囊在 440 DIP 下正好排满（量出来字到 406，框宽 408），第五颗
塞进去就得把「已运行 2天18小时」裁掉两个字。所以评分胶囊画在第一行、挨着两个图标按钮；
`TestTheScoreNeverCrowdsTheFacts` 钉住这件事：天气在、四颗胶囊一个不裁、评分在它们上面那一行。

**天气不是读数**：不进 `Readings`，也不占一张卡。它来自插件之外的服务，可以没有，所以是 `Snapshot`
上的可选字段，而不是一条每天都会有的读数。

来源和节奏在 `internal/weather` 与 `system/weather.go`：

- **城市**：先问 `myip.ipip.net`（回答「当前 IP：… 来自于：中国 重庆 重庆」），城市名交给
  Open-Meteo 的 geocoding 换经纬度；这条不通就退到 `ipwho.is`（城市和经纬度一次给全）。
- **天空**：`api.open-meteo.com/v1/forecast?current=temperature_2m,weather_code`。WMO 码到中文
  和图形的映射在 `weather.Report` 的 `Text()` / `Kind()` 里，面板只认 `Kind` 的七个名字
  （sun / partly / cloud / fog / rain / snow / thunder），不认码。
- **节奏**：十五分钟一次，在插件自己的 goroutine 上跑；机器本身还是两秒一次，两者不该互相拖。
  点刷新会顺手叫醒天气。
- **失败**：保留上一次成功的结果，同一句错误只写一行日志；**超过 45 分钟**（三次刷新）的结果按没有
  处理——合盖睡一夜醒来，不该显示昨晚的温度。
- **城市可以手动指定**：`city` 设置非空就跳过定位服务，直接 `FetchCity` 走 geocoding；空（默认）就是
  问 IP 服务。这是全局代理 / TUN 模式下唯一成立的办法，因为出口 IP 那时候一定不是本地的。设置项每
  次取数都重新读，所以设置页保存时触发的那次 `Poll` 就够，不用重启。填的内容原样交给 geocoder，
  头上显示的是 geocoder 认得的名字（`chongqing` → 重庆）。

设置页的页头换成「‹ 返回 + 设置」：返回目标用 `Host.LastInstance()`（`SetSelected(SettingsPage)`
本来就不写它），所以从哪一页进来就回哪一页。

### 设置页

只有三样：开机自启、不透明度、各插件自己声明的设置项（现在只剩本机的显卡选择）。
**没有「添加/移除监控项」，也没有「通知」「主题」**——监控项就是内置的那些，`Host.Load`
每次都为每个插件建实例，配置文件只存它们的设置值，不存「跑哪几个」。文件里没有的插件也会
被建起来：一个旧文件不该让新插件永远不出现。

**接收器不靠用户报身份。** 鼠标插件没有设置项，因为它按 collection 找设备：usage page
`0xFF00` / usage `0x0002` 是谁的都读（`mouse.IsBatteryChannel`），VID 从前是一个要用户
自己查的十六进制数，现在只是注释里那句「验证过的硬件是 `24AE:185A`」。放宽到所有厂商之后，
`ParseFrame` 成了唯一的闸门——扫到但不是这套协议发不出状态帧，就永远是「等待接收器广播」，
不会有假读数。多个候选同时打开时，面板上的设备名只取**真的发过帧**的那个接收器的产品字符串，
不取先打开的那个。旧的配置文件里残留的 `vid` 会被 `Settings.Load` 丢掉（插件不再声明这个键）。

设置项的 `Options` 是静态的；`Setting.OptionsFor` 留给只有运行时才知道的列表
（显卡那张卡），设置页每帧问一次。**只有一项可选的 choice 不出现**——一个只有一个答案的
问题不是问题，没有显卡的机器就不该有「监控哪张显卡」这一行。

`Settings.Load` 只收 manifest 还声明着的键：插件不再提供的设置（比如本机那行已经删掉的「盘符」）
读进来就丢掉，配置文件里不会留一个没人认识的值，下次保存也就没了。

### 不透明度

设置页「应用」卡最下面一条滑块，把整个窗口调淡，范围 40%–100%，存在配置文件的
`ui.opacity`（没这个字段就是 100%，老配置文件因此不用迁移）。

几个刻意的选择：

- **叫「不透明度」不叫「透明度」**：100% 必须等于「面板一直是的样子」。叫透明度的话
  100% 就是一块看不见的玻璃，这页上没有任何别的控件是这种反向语义。
- **下限 40%**：面板没有边框也没有抓手，淡到看不见就找不回来了。`Host.MinPanelOpacity`
  是唯一的事实来源，滑块和文件读取都过 `clampOpacity`。
- **拖动时只动窗口，松手才落盘**：`panel.App.OnOpacity` 每帧调 `Window.SetOpacity`，
  但要等指针抬起才 `Host.SetPanelOpacity`。一秒六十次重写设置文件不是功能，是磁盘问题。
- **先有窗口还是先有值**：面板在读配置文件之前就建好了，所以读完之后调一次
  `SyncOpacity`，把值推给窗口。忘了这一步的表现是「文件里写着 40%，启动却是全实心」。

Windows 上 `SetOpacity` 走 `WS_EX_LAYERED` + `SetLayeredWindowAttributes`：整个窗口
（包括 GPU 画的那层）一起变淡，鼠标照常能点。

面板里没有状态栏：日志和退出留在托盘右键菜单，面板只负责看。

`internal/panel/fit_test.go` 把三个页面按「桌面」和「笔记本」两种可用区各渲染一遍，
断言标题栏在顶部、标签页在其下、页面内容在标签页之下且起点在窗口内——这是能自动发现
「内容撑破固定高度」的手段，因为失败方式是画面被切，数值上全绿。

## 托盘与定位

`tray.Icon(State)` 画 32×32 PNG：数字居中、外圈按百分比点亮。
数字是手写的 4×7 点阵，缩放后落进环的空心里（系统字体在 32px 下渲染不稳）。
深浅任务栏各一套墨色，`Tray.Dark()` 与 `OnAppearanceChange` 跟着变。

`tray.Inset` 与 `tray.Dock` 都是纯函数，表驱动测试覆盖多显示器、超大面板、
边距小于 1 像素三种边界。

所有配色走 `palette`，跟随系统深浅色；自定义形状（环、折线、圆点）都用
`ui.Element.Draw` 画，没有图片资源。

## 环境与构建

- Go 1.27+，`go.mod` 只依赖 `github.com/egoist/mygo` 与 `golang.org/x/sys`。
- 无 cgo：Windows API 全走 `syscall.NewLazyDLL`，HID 用 setupapi/hid/kernel32。
- `mygo.json` 是应用配置（名字、identifier、版本、图标）。
- `go tool mygo build` 出 `build/windows-amd64/SeeAll.exe` 与安装包。

## 日志

日志只记**真正失败**的事，不记流程：面板上看得到的东西（读数、状态、离线）不需要
再抄一份到文件里。

失败分两种，都要留痕：

- **报错的失败**——`hid.Enumerate` 失败、读报文被打断。走 `mouse.fail()`，它同时写面板和日志。
  打开某个候选 collection 失败不算：所有厂商的同类 collection 都是候选，别人家打不开的那个
  不该盖在正在用的鼠标上，只有**一个都没打开**时才把它写进面板和日志。
- **不报错的失败**——接收器没插时 `Enumerate` 返回空列表而不是错误；性能计数器某
  一项在这台机器上不存在。这类「静默降级」原本只在页面上表现为少一张卡，日志里
  什么都没有，所以每一处都显式 `Logf`。

同一条问题只在**变化时**记一次：`ensureReaders` 每三秒跑一次，不设防的话一个没插的
接收器会让日志变成一句话复制一千遍。判据是「这条和上一条不同」，问题消失时把记录清掉，
这样它再次出现才算新事件。

## 诚实边界

- 雷柏接收器**没有读回通道**：除状态帧里的 DPI，写进去的参数无法确认。
  SeeAll 这一版只读不写，不做「你以为写进去了」的显示。
- HID 读取只有 Windows 实现，其他平台编译通过但鼠标插件报没有通道。
- 网络速率、GPU 占用与显存只有 Windows 实现：它们走 `pdh.dll` 的性能计数器
  （`\Network Interface(*)\Bytes Received/sec`、`\GPU Engine(*)\Utilization Percentage`、
  `\GPU Adapter Memory(*)\Dedicated Usage`）。其他平台的 `sampler` 不填这几项，
  插件就不报对应的读数。
- **显卡是按 LUID 分开读的**。性能计数器把每张卡的每个引擎都放在同一个计数器里，实例名
  是 `pid_12772_luid_0x00000000_0x0000E3CD_phys_0_eng_0_engtype_3D`；不认这个 LUID，
  求和出来的是「所有卡加起来」。设置页选中哪张卡，就只累加那张卡的实例。
- 显卡列表走 DXGI（`CreateDXGIFactory1` + `EnumAdapters1` + `GetDesc1`），与任务管理器
  「性能」页同一份来源：名字、显存（`DedicatedVideoMemory`）、LUID 都在 `DXGI_ADAPTER_DESC1` 里。
  不用注册表 `HardwareInformation.qwMemorySize` 了——那是按驱动实例排的，和计数器的 LUID
  对不上，同一块卡在 DXGI 里还可能列出两次。
- 只有**有引擎计数**的适配器才出现在选择里：DXGI 会把同一块 3060 列两次（一个没有引擎），
  也可能列一张已禁用的卡，选进去就是一行永远 0% 的读数。
- GPU 占用是所选卡各引擎利用率的**求和**（与任务管理器一致），不是平均值；软件渲染适配器
  （`DXGI_ADAPTER_FLAG_SOFTWARE`）不列出来。
- 磁盘只读**系统盘**（`%SystemDrive%`）：`GetDiskFreeSpaceExW` 取余量、`GetVolumeInformationW`
  取卷标与总量。仪表盘那张「可用空间」卡说的就是这个盘，不做所有卷求和。
- 温度走 PDH 的 `\Thermal Zone Information(*)\Temperature`（开尔文），取最热的 zone，
  只接受 0–125°C。那是主板/机箱级读数，所以文案写「系统 59°」，不冒充 CPU 封装温度。
  GPU 温度没有任何通用 API，拿不到就不画那个徽标，也不编一个数字。
- 进程榜：Toolhelp32 枚举 + `GetProcessMemoryInfo`（工作集）+ `GetProcessTimes`（两次采样
  之间 CPU 时间的增量 ÷ 墙钟 ÷ 核数），按程序名合并后取内存前 5；打不开的受保护进程跳过，
  间隔不到 100ms 的两次采样不算 CPU%（否则连点刷新会得到天文数字）。
- 电池健康度：`CM_Get_Device_Interface_List` 找到电池设备，`IOCTL_BATTERY_QUERY_INFORMATION`
  读 `DesignedCapacity` 与 `FullChargedCapacity`；拿不到就不写「健康 78%」那个徽标。
- 自带只有鼠标和本机两个插件：插件化是给后续监控对象留的口子，不是一次塞满托盘。
- 天气靠 IP 定位：认的是**出口 IP**，挂着全局代理时会定到代理落地那座城市。这是免 key 方案的
  固有限制；服务不通时左上角回到健康分，不显示一个编出来的温度。

## 验证过的事

- `go run ./cmd/diag -capture` 在本机雷柏接收器上收到 `BB B0 42 B0 04 01 5F` 帧，
  解析出 95% / 1200 DPI / 无线，与老 .NET 版的抓包结果一致。
- 面板在真实窗口里跑起来（`PrintWindow` 截图），深色面板下都正常渲染。