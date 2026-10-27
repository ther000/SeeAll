# AGENTS.md

本仓库的开发流程。通用偏好见 `~/.dsh/AGENTS.md`，这里只说这个项目怎么改、怎么验。

## 技术前提

Go + MyGo 原生 UI，**没有 webview、没有前端构建、没有 JS**。别引入 Vite、别写 `.tsx`。
无 cgo，Windows API 走 `syscall.NewLazyDLL`。

## 改一个东西的流程

1. **先看测试**。布局相关先跑 `go test ./internal/panel`，它会把现状钉出来。
2. **改代码**，别加抽象层。这个项目小，直接写就是最好的抽象。
3. **跑这四条**，缺一不可：

   ```powershell
   go build ./...
   go vet ./...
   go tool mygo vet          # 原生 UI 的 build 生命周期，go vet 抓不到
   go test ./...
   ```

4. **改了界面就出图看**：`go run ./cmd/preview -out docs`，它会渲染三页 PNG。
   数值全绿但画面被切是这类固定尺寸布局的典型失败方式，必须看。
5. **重新打包**，每一次改动都要，没有例外：

   ```powershell
   go tool mygo build
   ```

   出 `build/windows-amd64/`（exe + 安装包）。再同步根目录那个双击即用的
   `SeeAll.exe`（见下）。产物不进版本库，所以**不打包就等于没交付**——
   提交里说得再好，`build/` 里还是上一版。

6. **同步文档**：用户可见的行为改 `README.md`，架构或约定改 `docs/design.md`，
   行为变更记到 `CHANGELOG.md` 的 `[未发布]`。改了面板就重出 `docs/*.png`。
7. **提交**：`type: 中文简述`（feat / fix / refactor / chore / docs），
   body 讲为什么这么改，不复述 diff。

## 打包

`go tool mygo build` 出 `build/windows-amd64/`：`SeeAll.exe` 与
`SeeAll Setup <版本> amd64.exe`。**每次改动后都要跑**，见流程第 5 步。

根目录那个 `SeeAll.exe` 是给人双击的副本，跟 `build/` 里的不是同一个文件，
**必须手工复制过去**，否则它还是上一版：

```powershell
Get-Process SeeAll -ErrorAction SilentlyContinue | Stop-Process -Force
Copy-Item build/windows-amd64/SeeAll.exe SeeAll.exe -Force
```

先停实例，否则 Windows 报 "Device or resource busy"。复制完两边对一下 md5，
确认是同一份。

`build/` 和根目录的 exe 都在 `.gitignore` 里，不会进提交——所以打包是**交付**，
提交只是记录。

## 测试的要求

**只照抄实现逻辑的测试等于没测。** 写完问自己：这条测试能发现什么错误的改动？
答不上来就别写。

写完还要**证明它会失败**：临时把修复撤掉跑一遍，看它红。撤掉还是绿的，说明测试没咬住东西。

- 布局断言一律用 `ui.NewTester(p.View, DesignWidth, DesignHeight)` 无头渲染。
  用 `SetCursorPos` + `mouse_event` 驱动真窗口在远程/无焦点会话里会**静默失效**
  （点了没反应也不报错），搭在上面的验证等于没验证。
- 需要硬件的用 `t.Skip`，别用 `t.Fatal`。

## 几条会踩的坑

- `Padding` 是 CSS 顺序：两参数是 **（垂直, 水平）**，写反卡片就缩一圈。
- 面板尺寸的设计单位是**物理像素**（660×840），窗口收 DIP，`Fit` 必须传 `ScaleFactor`。
- `ui.Element` 只活一个 build pass，别存进结构体或跨 goroutine 传（`mygo vet` 会抓）。
- 要量控件本身就得写 `.Label()`，否则 `Tester.Find` 量到的是内层文字盒。
- 宿主**没有定时器**：采样频率写在各插件自己的 `Start()` 里，别在 Manifest 上找。
- 显卡计数器按 LUID 区分，直接 `sum()` 得到的是"所有卡加起来"。

## 不要做

- 不要在视图里做 I/O 或起 goroutine。
- 不要为了让测试过而放宽断言——先判断是代码错还是测试错。
- 不要在没验证的情况下说"已上真机验证"。
