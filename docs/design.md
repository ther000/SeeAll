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
app     ← 宿主：加载/保存设置、创建与停止实例、快照与历史、通知，只依赖 core + plugin + plugins
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
`Notify()` 弹系统通知、`Logf()` 写日志。
插件不碰 UI、不碰文件路径、不管生命周期——取消也是：由 `Start()` 的 goroutine
自己看退出信号，宿主只调 `Instance.Stop()`。

`Manifest` 声明：ID、显示名、一句话说明、分组（设备/本机/服务）、图标、
设置字段（`Setting{Key,Label,Type,Default,...}`）、能否多实例、默认实例名，
以及 `Combined`——这个插件的读数描述同一个东西，页面用一张卡装下（鼠标：电量/连接/DPI）。

**Manifest 只放有人在读的字段。** 采样频率写在各插件的 `Start()` 里（宿主没有定时器），
「未验证协议」的标记在它真的有一个未验证协议时才加回来。

设置页完全由 `Manifest.Settings` 生成：加一个字段就多一个控件，UI 代码不用动。

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

**读数 ID 是插件与面板的契约**：仪表盘把 `cpu/gpu/mem/disk/net/temp` 摆成六张卡，
`vram` 并进 GPU 的说明行，`uptime` 只进头部的胶囊，`battery` 单独一张整宽卡；
不认识的读数兜底成同一套卡片，追加在六张后面。`Badge` 只在这一层用得上，
经典外观不画它。

**Health 由插件算，不由面板算**：什么算「健康」是那个插件的事（鼠标拿电量当分数，
本机按 CPU/内存/GPU/温度/磁盘/电池各自的免罚区间扣分），读不到的维度不扣分。
`0` 是保留值：面板画设备名而不是一个假分数。

**Processes 按程序名在插件里合并**：浏览器是十几个进程，五行 chrome 不如一行「chrome」。

## 面板

停在**屏幕右下角，离边缘和任务栏各 8 像素**——`tray.Inset(work, marginDIPs)` 把工作区
内缩，`tray.Dock(area, w, h)` 再右下对齐。工作区本身已排除任务栏，边距是额外留的浮起感。

**尺寸的单位是物理像素，不是 DIP。** 设计稿画的是 660×840 px，而窗口只收 DIP，
所以 `panel.Fit` 和 `panel.MarginDIP` 都要显示器的 `ScaleFactor`：

```go
const DesignWidth, DesignHeight = 660, 840   // 像素
Width  = round(660 / scale)                  // DIP：1x→660，1.5x→440，2x→330
Height = round(840 / scale)
```

这样面板在任何缩放下都占同样多的屏幕像素。写反了（把 660×840 当 DIP）在 150% 的机器上
会变成 990×1260 px，大一倍半——`fit_test.go` 有一条测试专门钉这个换算。

放不下时按高度等比缩，宽度再兜一次底。屏幕再大也不跟着长——托盘面板不该铺满屏幕，
这一点和老版本「贴内容长」的错误正好相反。

一次算定，切页、关掉重开都不变；内容装不下时页内滚动。**这是从老项目继承的硬约束**：
老版本用 `SizeToContent` 让窗口贴着内容长，结果设置页一开就长出屏幕外。
现在页面容器是 `Grow(1) + Shrink(1) + Basis(0)`，标题栏永远在顶部。

### 两套外观

外观存在配置文件的 `ui.theme`（`Host.Theme` / `Host.SetTheme`，默认 `dashboard`）：

- `dashboard`（深蓝仪表盘）：`internal/panel/dashboard.go`，默认外观。
- `classic`：下面那三层。

两套共用同一份读数、同一批页面函数（设置页、设备页复用），只换调色板和版式：
`App.View` 先问主题，`classic()` 是原来那条路径一个字没动，`dashboard()` 另起一块画布。
仪表盘每帧把 `ui.Theme` 改成深色（`dashFor`），控件、滚动条、开关都跟着深——它是设计成深色的
面板，不跟随系统外观。

仪表盘是**一页**：`Selected` 只在「是不是设置页」上有意义，其余时候画的是所有监控项的聚合页。
自上而下：画布（左暖紫到右蓝的渐变 + 页脚一层暖光，`Painter.FillGradient` 直接画）
→ 头（健康分、评语、`Facts` 胶囊，右上角刷新与设置两个图标按钮）
→ 滚动内容（机器六卡两列 → 设备整宽卡 → 电池整宽卡 → 进程卡）→ 16 的底部留白。
**没有页脚栏**：内容和窗口底边之间只有这点留白，切换监控项靠一张张卡本身，不靠标签。
每块高度写死（`dashCardH`、`dashDeviceH`、`dashBatteryH`、`dashProcRowH`），因为
「660×840 里刚好放下」是设计的一部分：卡片一长就会挤出屏幕，窄窗口就退化成页内滚动。

设备卡不是网格里的一格，而是**整宽**一张：标题是实例名，大数字是主读数（`Primary`，没有就
`Lead()`），徽标是连接方式，说明行是其余读数。设备掉线时卡还在，数字写「—」、徽标写「未连接」。
机器与设备的分界是 `Manifest().Combined`，**不是快照里有没有 Health**——鼠标插件也发 Health，
按快照特征判会把鼠标当成机器，整页反过来。

头在 `Health == 0` 时改画机器名——一个不打分的数据源不该顶着一个假分数。进程卡没有数据就整块不出现。

设置页的页头换成「‹ 返回 + 设置」：返回目标用 `Host.LastInstance()`（`SetSelected(SettingsPage)`
本来就不写它），所以从哪一页进来就回哪一页。

经典外观的三层：

- **标题栏**（`header`）：图标 + 应用名 + 刷新 + 设置。
- **标签页**（`tabs`）：`ui.TabsBase` 自绘，每项 `Grow(1)` 等宽；选中项白底描边，
  状态非「正常」时标签上带彩点——一次只看得到一页，别页的异常得有地方露。
- **页面**：`detail` 有两种排法，由 `Manifest.Combined` 决定。
  - 默认：每个读数一张卡（`ui.GridView` 两列），**全部等高**（`ReadingCardH`，`Height` +
    `MaxHeight` 夹死，所以有没有说明行都是同一个高度）。卡里标签和折线共用顶行
    （折线居右，`trendWidth`×`trendHeight` 的固定位置，没有折线也占位），下面依次是值与说明。
    **卡片可以拖着排序**：`GridView` 的 `GridState.Reorder` 给出新的顺序，`Host.SetCardOrder`
    按实例存进配置文件的 `order`。插件后来新增的读数接在排过的后面，不再报的自动跳过
   ——**跳过是在画的时候做的，不是把 ID 从文件里删掉**。快照只说明「这一秒报了什么」，
   一台没电池的机器不会报 battery，按快照裁剪会把用户排好的位置连同卡片一起丢掉。
   只有实例被移除时才清掉它的顺序（`Host.Remove`）
    （`arrangeOrders`）——插件升级不该冲掉用户排好的部分。
    `GridView` 会在单元格四周各留 2 DIP，页面把这个宽度从 padding 里扣掉，卡片文字才和
    别的页落在那条统一的竖线上（`align_test.go` 抓的就是这 2 px）。
    插件指定的主读数排第一张，字号大一号。适合读数在量不同东西的页面（本机：CPU、内存、
    电池、网络上下行、GPU、显存、开机时长）。
  - `Combined`：整页一张卡——设备名（`Snapshot.Title`）横在顶部，主读数（`headline`，只有环或数字）
    在左，其余读数（`inlineReading`，「标签 … 值」一行一条）在右。不画折线、不写 caption：
    环和数字已经说完了。适合读数描述同一个东西的页面（鼠标：电量、连接、DPI）。
  - `settings` 是长表单，滚动。

设置项的 `Options` 是静态的；`Setting.OptionsFor` 留给只有运行时才知道的列表
（显卡那张卡），设置页每帧问一次。**只有一项可选的 choice 不出现**——一个只有一个答案的
问题不是问题，没有显卡的机器就不该有「监控哪张显卡」这一行。

设置页的「添加」卡片列出**当前没有实例的插件**，所以移除掉的那一项从这里加回来。
`Host.Remove` 把它当时的设置写进配置文件的 `removed`，`Host.Add` 拿到 `nil` 取值时
读这块，加回来的是原来那个监控项而不是一个全新的；加回成功后这块清掉。

配置文件里 `version` 为 0（即文件不存在）才按默认插件播种实例。文件写了但没有实例，
表示用户把监控项全删了，重启不该替他把它们加回来。

`Settings.Load` 只收 manifest 还声明着的键：插件不再提供的设置（比如本机那行已经删掉的「盘符」）
读进来就丢掉，配置文件里不会留一个没人认识的值，下次保存也就没了。

### 通知分两层

每个插件有自己的「提醒」开关（鼠标的「低电量提醒」、本机的「电池提醒」），它决定
**这个监控项在什么情况下会想通知**；设置页「应用」卡里的「通知」是**总开关**，
决定**有没有通知能出去**。两层都要开才会弹。

总开关拦在 `Host.Notify` 里而不是各个插件里：插件只说自己想通知什么，能不能真弹是
宿主的判断——插件多一个，就少改一处。存在配置文件的 `ui.notify`，没这个字段就是开。

`Host.Notify` 和 `Host.OnNotify` 是**两层**，不是一回事：`Notify` 是入口（插件和
宿主自己的提醒都走它，总开关在这里生效），`OnNotify` 是出口（`main` 在桌面就绪后
把它设成真的发系统通知）。`Load` 曾经把 `OnNotify` 指向一个又调 `Notify` 的函数，
于是第一次通知就无限递归到栈溢出——两个名字像一对，语义却是一个管入一个管出。

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

- **报错的失败**——`hid.Enumerate` 失败、`OpenReader` 失败、读报文被打断。走
  `mouse.fail()`，它同时写面板和日志。
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

## 验证过的事

- `go run ./cmd/diag -capture` 在本机雷柏接收器上收到 `BB B0 42 B0 04 01 5F` 帧，
  解析出 95% / 1200 DPI / 无线，与老 .NET 版的抓包结果一致。
- 面板在真实窗口里跑起来（`PrintWindow` 截图），深色主题下各页面都正常渲染。