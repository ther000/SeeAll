# AGENTS.md

给在本仓库工作的 AI agent 的说明。写给"刚接手、上下文为零"的人：
看完应当知道怎么改、改完怎么验证、哪些地方一碰就错。

面向用户的话术、UI 取舍等通用偏好见 `~/.dsh/AGENTS.md`，这里只写本项目的具体事实。

## 这是什么

竖版托盘监控面板。一个 Go 二进制，用 [MyGo](https://mygo.egoist.dev/) 的原生 `ui` 包
把界面直接画在 GPU 上——**没有 webview、没有前端构建、没有 JavaScript**。
不要引入 Vite、不要写 `.tsx`、不要在 `mygo.json` 里配 `frontendDist`。

结构是插件化的：一个监控对象 = 一个实现 `plugin.Plugin` 的包。自带两个
（`mouse` 鼠标电量、`system` 本机状态）。

## 命令

```powershell
go build ./...                    # 编译，最先跑这个
go vet ./...                      # 标准 vet
go tool mygo vet                  # 额外检查原生 UI 的 build 生命周期，必须跑
go test ./...                     # 全部测试
go tool mygo build                # 出 build/windows-amd64/SeeAll.exe 与安装包
go build -o seeall.exe .          # 快速编译（带控制台，启动时自己藏起来）
```

打包后要手动把 `build/windows-amd64/SeeAll.exe` 复制到仓库根目录（`SeeAll.exe` 被 gitignore）。
**复制前先停掉正在运行的实例**，否则 Windows 报 "Device or resource busy"：

```powershell
Get-Process -Name SeeAll -ErrorAction SilentlyContinue | Stop-Process -Force
cp build/windows-amd64/SeeAll.exe ./SeeAll.exe
```

开发辅助：

```powershell
go run ./cmd/preview -out docs              # 无窗口渲染三页 PNG（docs/ 里的文档图就是这么来的）
go run ./cmd/icon                           # 重新生成 resources/icon.png
go test ./internal/tray -run TestIconSheet  # 托盘图标各状态 → .preview/probe-icons.png
go run ./cmd/diag -list                     # 列 HID collection，* 标记电量通道
go run ./cmd/diag -capture -seconds 10      # 抓鼠标状态帧并解析
```

`CGO_ENABLED=0`，所以 **`-race` 用不了**（`go test -race` 直接报错）。并发正确性靠
代码结构（见下）而不是竞态检测器。

## 目录与依赖方向

```
core       读数模型（Reading/Snapshot/Severity/Ring/单位格式化），不依赖任何东西
plugin     插件契约（Plugin/Instance/Manifest/Env/Settings），只依赖 core
plugins/*  具体插件，依赖 core + plugin（+ hid / syscall）
app        宿主：设置落盘、实例生命周期、快照与历史、通知
panel      视图，依赖 app + core + plugin + mygo/ui
tray       托盘图标与面板定位，只依赖 mygo
main       组装：窗口、托盘、菜单、自启
cmd/*      开发工具（preview / icon / diag），不参与打包
```

**依赖方向是单向的**：插件不知道面板存在，面板不知道 HID 存在。
要加一层反向依赖，先想想是不是该用回调（`Host.OnChange`、`App.OnOpacity` 就是这么做的）。

## 会踩的坑

按"踩到的次数"排序。

### 1. `Padding` 的参数顺序是 CSS 的

```go
Padding(14)             // 四边都是 14
Padding(12, 14)         // 上下 12，左右 14  ← 两参数是 (垂直, 水平)
Padding(12, 14, 12, 14) // 上右下左
Padding(1, 2, 3)        // 上 1，左右 2，下 3
```

写 `Padding(14, 12)` 想表达"左右 14、上下 12"是**错的**，卡片会横向缩一圈。
这个项目里正确写法统一叫 `Padding(cardPadY, cardPadX)`，就为了别再写反。
真实实现见 `ui` 包的 `edges()`。

### 2. 面板尺寸的单位是物理像素，不是 DIP

```go
const DesignWidth, DesignHeight = 660, 840   // 像素
Width = round(660 / scale)                   // 窗口要的 DIP：1x→660，1.5x→440，2x→330
```

设计稿是像素，窗口只收 DIP。`panel.Fit` 与 `panel.MarginDIP` **都必须**传
`display.ScaleFactor`。把 660×840 当 DIP 会在 150% 的机器上大一倍半。
`fit_test.go` 有一条测试专门钉这个换算，改这里先看它。

### 3. `ui.Element` 只活一个 build pass

视图每帧重建，元素值不能存进 `App`、不能跨 goroutine 传。
要跨帧保住的是 `ui.GridState`、`ui.Handle` 这类状态对象，或者干脆存进 `App` 的字段。
`go tool mygo vet` 会抓这一类误用（实测：把元素存进包级变量后它报
`native UI build value stored in a package variable; keep ui.Handle or ui.Services instead`
并以非零码退出），**改了 panel 就跑它**。`go vet` 抓不到，必须用 `mygo vet`。

视图是"状态 → 界面"的纯函数，只在主线程跑。`App.View` 不写任何跨线程状态。

### 4. 图标按钮要写 `Label`

`ui.Tester.Find(s)` 先找 `Label`，找不到才退回内层文字盒。实测同一个 400px 宽的窗口里：

| 元素 | Find 量到的盒子 |
| --- | --- |
| `ui.Button(c, "文字按钮")` | 56×18.7（只是文字，不含按钮 padding） |
| `ui.Button(c, "").Label("图标按钮")` | 360×29（整个控件） |
| `ui.Text(c, "裸文字")` | 360×18.7（整行文字） |

所以**要量控件本身就必须写 `.Label()`**，纯图标按钮尤其（不写连找都找不到）。
项目里所有图标按钮都带 `.Label("刷新")` / `.Label("打开设置")`。断言位置时先想清楚
量的是控件还是文字，两者差一圈 padding。

### 5. `GridView` 的单元格四周有 2 DIP

来源不是 padding，是 `gridview.go` 里每个 cell 的 `Border(2, Color{})`——一条透明的
2 DIP 描边，留给选中环（选中时换成实色，内容不会跳）。边框算在盒子内部，所以内容
四周各缩 2 DIP。

卡片网格页要把这 2 DIP 从页面 padding 里扣掉（`detail.go` 的 `gridInset`），否则卡片
文字和 `Combined` 页差 2px。`align_test.go` 抓的就是这条：三页的卡片边缘必须落在
同一条竖线上。**找这 2 DIP 时别去 GridView 里搜 padding，它在那儿没有。**

行间距则来自 `t.Space(2)`（主题 `Spacing=4`，即 8 DIP）与行上的 `PaddingY(gap/2)`，
`cardsize_test.go` 按 `ReadingCardH + 2*gridGap` 断言行距。

### 6. PDH 计数器要两次 collect 才有值

性能计数器的速率型计数器第一次 `PdhCollectQueryData` 之后没有值，实例列表也是空的。
`sampler.initPerf` 里先 `refresh()` 一次；枚举显卡引擎时 `engineLuids` 会重试最多三次
（每次间隔 120ms）直到拿到实例。**别把这个重试当成冗余删掉**，删了显卡列表就是空的。

### 7. 显卡是按 LUID 分开的

性能计数器把所有卡的引擎放在一个计数器里，实例名是
`pid_12772_luid_0x00000000_0x0000E3CD_phys_0_eng_0_engtype_3D`。
直接 `sum()` 得到的是"所有卡加起来"。必须先 `parseLuid` 过滤。
`gpu_test.go` 钉住了 LUID 解析的偏移量——那段下标写错是静默的（数据还在，只是错的）。

显卡列表走 DXGI（`CreateDXGIFactory1` + `EnumAdapters1` + `GetDesc1`），**不是**注册表。
只有"有引擎计数"的适配器才出现在设置里，因为 DXGI 会把同一块卡列两次（一个没有引擎）。

### 8. `Manifest.Interval` 目前没有实际作用

`plugin.Manifest.Interval` 的注释写着"宿主按这个间隔轮询"，**但宿主没有定时器**
（`internal/app` 里没有 `time.NewTicker`）。两个插件都是自己的 goroutine 里起
ticker（mouse 3s、system 2s），`Interval` 只被声明、没人读。

改采样频率要去插件自己的 `Start()` 里改，改 `Interval` 不会有任何效果。
哪天让宿主真的按它轮询，记得同步改两边的注释和这里的说明。

### 9. 没有 cgo

Windows API 全走 `syscall.NewLazyDLL` + `.Call`。不要引入 `golang.org/x/sys/windows`
之外的第三方绑定，也不要用 `//go:build windows` 之外的构建标签去绕。

## 插件规则

新插件 = 实现两个接口，加进 `internal/app/builtin.go` 的切片，**UI 代码一行都不用改**：

```go
type Plugin interface {
	Manifest() Manifest
	New(env *Env) Instance
}
type Instance interface {
	Start()   // 起 goroutine，立刻返回
	Stop()    // 等 goroutine 全停
	Poll()    // 立刻取一次快照，任意 goroutine 可调
}
```

- 插件只能用 `Env`：`Settings()` / `Publish()` / `Context()` / `Notify()` / `Logf()`。
  **不碰 UI、不碰文件路径、不管生命周期**。
- 插件的 goroutine 只调 `Publish`。宿主用 `OnChange` 通知界面重绘。
- 设置页完全由 `Manifest.Settings` 生成。加一个字段 = 加一个 `Setting`，不改 UI。
  - 只有运行时才知道的选项（显卡列表）用 `Setting.OptionsFor`，每帧会被问一次，要缓存。
  - **只有一项可选的 choice 不画**——一个只有一个答案的问题不是问题。
  - 选中的值在机器上不存在时，那一项显示"未找到"并留在列表里，别偷偷换成别的。
- 读数 ID 一旦发布就是持久化键（`order` 按 ID 存），**不要改已有的 ID**。

## 配置文件规则

`%AppData%\SeeAll\seeall.json`：

| 字段 | 含义 | 坑 |
| --- | --- | --- |
| `version` | 0 = 文件不存在 | **只有 version==0 才按默认插件播种实例**，否则用户删空的监控项会自己长回来 |
| `instances[].settings` | 每项取值 | `Settings.Load` 只收 manifest 还声明着的键，删掉的设置下次保存就没了 |
| `removed` | 被移除实例的设置 | 加回来时用，加回成功后清掉 |
| `order` | 卡片顺序，按实例分 | 插件新增的读数接在后面，不再报的跳过（`arrangeOrders`） |
| `ui.opacity` | 面板不透明度 | 没有就是 100%，老配置不用迁移 |

## UI 约定

- **一套间距刻度**：`Gutter=14`、`CardGap=10`、`cardPadX/Y=14/12`、`cardRadius=12`。
  别在页面里写魔法数字，跨页对齐全靠它们。
- **卡片等大**：`ReadingCardH` + `Height` + `MaxHeight` 三件套。只给 `MinHeight`
  会让行距不一致（踩过）。
- **配色只走 `palette`**：跟随系统深浅色。自定义形状（环、折线、圆点）用
  `ui.Element.Draw` 画，**没有图片资源**，别加 PNG。
- **两种页面排法**由 `Manifest.Combined` 决定：默认每个读数一张卡（GridView 两列，
  可拖拽排序）；`Combined` 整页一张卡（设备名 + 左侧环 + 右侧读数，不画折线不写 caption）。
- 面板里没有状态栏。日志和退出在托盘右键菜单。
- 名词要正着说：`不透明度` 而不是 `透明度`（100% 必须是"面板一直是的样子"）。
  这类反向语义在本项目里一律避免。

## 测试

**先想清楚这条测试要能发现什么，写不出来就别写。** 只照抄实现逻辑的测试等于没测。

- 布局类：`ui.NewTester(p.View, DesignWidth, DesignHeight)` 无头渲染后 `Find` / `Texts` /
  `Press`-`Move`-`Release`。这是**唯一可靠**的路径：靠 `SetCursorPos` + `mouse_event`
  驱动真窗口在远程/无焦点会话里会静默失效（点下去没反应，也不报错），搭在上面的验证
  等于没验证。真机只能靠人点，或者 `cmd/preview` 出图用眼睛核对。
- 写测试之后要**证明它会失败**：临时把修复撤掉跑一遍，看它红。这个仓库里每条关键测试都这么验过。
- 需要硬件的测试用 `t.Skip` 而不是 `t.Fatal`（没有显卡的机器不该挂 CI）。
- 共用 fixture：`internal/panel/layout_test.go` 的 `fixture(t)`、
  `internal/plugins/system/system_test.go` 的 `newTestInstance(t)`。别每篇测试自己搭一套。

## 文档

改行为就要同步，**这是硬要求**，不是建议：

| 文件 | 什么时候改 |
| --- | --- |
| `README.md` | 用户可见的行为、命令、产物表 |
| `docs/design.md` | 架构、约定、诚实边界；解释"为什么这么做" |
| `CHANGELOG.md` | 每次行为变更记到 `[未发布]` 下（Keep a Changelog 格式） |
| `docs/*.png` | 面板变了就 `go run ./cmd/preview -out docs` 重出 |

`docs/design.md` 的"诚实边界"一节要如实维护：没在真实硬件上验证的、
只有 Windows 实现的、做不到的，都写在那里，别让文档比代码自信。

## 提交与版本

- 提交信息：`type: 中文简述`（`feat` / `fix` / `refactor` / `chore` / `docs`），
  body 讲**为什么**和取舍，不是复述 diff。看看 `git log` 最近的几条就知道调子。
- 版本号在 `mygo.json`，安装包名跟着它走。语义化版本，破坏性变更才动大版本。
- 发版时：改 `mygo.json` → 更 `CHANGELOG.md` → `go tool mygo build` → 打 tag → 复制 exe。
- 仓库有 `.gitattributes` 强制 LF，别提交 CRLF。

## 不要做的事

- 不要给 `ui` 层加"功能开关"或抽象层——这个项目小，直接写就是最好的抽象。
- 不要在视图里做 I/O、起 goroutine、或者读文件。
- 不要把 `ui.Element` 存进结构体字段（见坑 3）。
- 不要为了让测试过而放宽断言，先判断是代码错了还是测试写错了。
- 不要在没验证的情况下声称"已在真实硬件上验证"。