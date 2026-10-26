# SeeAll 设计与接口约定

SeeAll 是「鼠标电量显示」的完全重写：同一个托盘小工具，但内核换成了插件化监控。
用 Go + [MyGo](https://mygo.egoist.dev/)（原生 UI，无 webview、无前端构建）。

## 一句话

托盘点开一块面板，左边是**监控对象**（插件的实例），右边是它的当前值和读数列表；点「设置」进配置页。
新增一种「可监控的东西」= 写一个实现 `plugin.Plugin` 的包，加进 `internal/app/builtin.go` 的切片。

## 分层

```
core    ← 与插件无关的读数模型（Reading / Snapshot / Status / Severity / Ring / 单位格式化）
plugin  ← 插件契约（Plugin / Instance / Manifest / Env / Settings），只依赖 core
plugins/* ← 五个具体插件，依赖 core + plugin + hid（需要时）
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
`Context()` 在 Stop 时被取消、`Notify()` 弹系统通知、`Logf()` 写日志。
插件不碰 UI、不碰文件路径、不管生命周期。

`Manifest` 声明：ID、显示名、一句话说明、分组（设备/本机/服务/Agent）、图标、
设置字段（`Setting{Key,Label,Type,Default,...}`）、能否多实例、默认实例名、是否实验状态。

设置页完全由 `Manifest.Settings` 生成：加一个字段就多一个控件，UI 代码不用动。

## 宿主

```go
host := app.NewHost(dataDir)   // 带内置插件
host.Load()                    // 读 seeall.json；没有就按默认插件各建一个实例
host.Instances() []*Instance   // 侧栏顺序
host.Instance(id) *Instance
host.Summaries() []Summary     // 托盘菜单用
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
	Unit      Unit      // %, ms, bit/s, °C, 天……
	Text      string    // 有它就显示文字，不显示数字
	Severity  Severity  // none / ok / info / warn / critical / offline
	Detail    string    // 副行：「2 秒前」「212 GB 可用 / 953 GB」
	At        time.Time
}
```

`Snapshot.Primary` 指定「这页的大数字用哪项」——电量页要环、CPU 页要百分比、
服务页要延迟、Agent 页要数量。不指定就退化成「最严重的那项」。

## 面板

**贴着屏幕右边缘、坐在任务栏上方**——`tray.Dock(work, w, h)` 把窗口放到工作区的右下角（工作区已经排除了任务栏，所以「贴任务栏」不需要额外算高度）。

尺寸由 `panel.Fit(workW, workH)` 算，不是写死的：

```go
Width  = clamp(workW*42%, 560, 920)
Height = clamp(workH*58%, 560, 680)
Sidebar = 184  // 面板窄于 660 时按比例缩到 128
```

一次算定，切页、关掉重开都不变；内容装不下时页内滚动。**这是从老项目继承的硬约束**：
老版本用 `SizeToContent` 让窗口贴着内容长，结果设置页一开就长出屏幕外。
现在右侧容器是 `Grow(1) + Shrink(1) + Basis(0)`，标题永远在顶部。

- 侧栏 184 DIP：按分组列监控项，第二行是最关键的读数，右侧状态点。
- 详情页是仪表盘：主读数一张大卡（环 + 趋势折线），其余读数两张一行的小卡（`ui.Grid`），页脚钉在底部。
- 设置页：应用级开关 + 每个实例一张卡（设置字段 + 移除）+ 添加卡片。

`internal/panel/fit_test.go` 把四个监控页按「桌面」和「笔记本」两种工作区尺寸各渲染一遍，
断言页脚没被裁掉——这是唯一能自动发现「内容撑破固定高度」的手段，因为失败方式是画面被切，
数值上全绿。

## 托盘与定位

`tray.Icon(State)` 画 32×32 PNG：数字居中、外圈按百分比点亮。
`tray.Dock` 是个纯函数，表驱动测试覆盖多显示器与超大面板两种边界。

所有配色走 `palette`，跟随系统深浅色；自定义形状（环、折线、进度条）都用
`ui.Element.Draw` 画，没有图片资源。

## 托盘

`tray.Icon(State)` 画 32×32 PNG：数字居中、外圈按百分比点亮。
数字是手写的 4×7 点阵，缩放后落进环的空心里（系统字体在 32px 下渲染不稳）。
深浅任务栏各一套墨色，`Tray.Dark()` 与 `OnAppearanceChange` 跟着变。

`tray.Place(anchor, work, w, h)` 算面板位置：居中于图标下方，放不下就翻到上方，
再夹进工作区。这是个纯函数，有表驱动测试。

## 环境与构建

- Go 1.27+，`go.mod` 只依赖 `github.com/egoist/mygo` 与 `golang.org/x/sys`。
- 无 cgo：Windows API 全走 `syscall.NewLazyDLL`，HID 用 setupapi/hid/kernel32。
- `mygo.json` 是应用配置（名字、identifier、版本、图标）。
- `go tool mygo build` 出 `build/windows-amd64/SeeAll.exe` 与安装包。

## 诚实边界

- 雷柏接收器**没有读回通道**：除状态帧里的 DPI，写进去的参数无法确认。
  SeeAll 这一版只读不写，不做「你以为写进去了」的显示。
- 键盘插件标着「实验」：HID 电池字段与雷柏帧两条路都实现了，但没实测硬件。
- HID 读取只有 Windows 实现，其他平台编译通过但设备插件报没有通道。
- Agent 插件只读进程表，不读对话内容、不读文件。

## 验证过的事

- `go run ./cmd/diag -capture` 在本机雷柏接收器上收到 `BB B0 42 B0 04 01 5F` 帧，
  解析出 95% / 1200 DPI / 无线，与老 .NET 版的抓包结果一致。
- 面板在真实窗口里跑起来（`PrintWindow` 截图），深色主题下五个页面都正常渲染。
