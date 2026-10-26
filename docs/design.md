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
`Context()` 在 Stop 时被取消、`Notify()` 弹系统通知、`Logf()` 写日志。
插件不碰 UI、不碰文件路径、不管生命周期。

`Manifest` 声明：ID、显示名、一句话说明、分组（设备/本机）、图标、
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

`Snapshot.Primary` 指定「这页的大数字用哪项」——电量页要环、CPU 页要百分比。
不指定就退化成「最严重的那项」。

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

三层结构：

- **标题栏**（`header`）：图标 + 应用名 + 刷新 + 设置。
- **标签页**（`tabs`）：`ui.TabsBase` 自绘，每项 `Grow(1)` 等宽；选中项白底描边，
  状态非「正常」时标签上带彩点——一次只看得到一页，别页的异常得有地方露。
- **页面**：`detail` 是主读数大卡 + 其余读数的小卡（`ui.Grid` 两列）。小卡里标签和折线共用顶行
  （折线居右，`trendWidth`×`trendHeight` 的固定位置，没有折线也占位），下面依次是值与说明；
  `settings` 是长表单，滚动。

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

## 诚实边界

- 雷柏接收器**没有读回通道**：除状态帧里的 DPI，写进去的参数无法确认。
  SeeAll 这一版只读不写，不做「你以为写进去了」的显示。
- HID 读取只有 Windows 实现，其他平台编译通过但鼠标插件报没有通道。
- 自带只有鼠标和本机两个插件：插件化是给后续监控对象留的口子，不是一次塞满托盘。

## 验证过的事

- `go run ./cmd/diag -capture` 在本机雷柏接收器上收到 `BB B0 42 B0 04 01 5F` 帧，
  解析出 95% / 1200 DPI / 无线，与老 .NET 版的抓包结果一致。
- 面板在真实窗口里跑起来（`PrintWindow` 截图），深色主题下各页面都正常渲染。
