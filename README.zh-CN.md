# NIIMBOT N1 — Linux 标签打印机驱动

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**其他语言：** [English](README.en.md) · [Русский](README.md) · [Nederlands](README.nl.md) · [Pirate 🏴‍☠️](README.pirate.md)

通过蓝牙 LE 驱动热转印标签打印机 **NIIMBOT N1**。使用 Go 编写，在命令行
中运行：打印文字和图片、显示打印机状态，还能在打印前预览版面。

不需要厂商 App，不需要手机，不需要账号 —— 只要有 Linux、蓝牙和一条命令。

```console
$ niimbot text "水泵" "12A-5"
```

## 功能

| 命令 | 作用 |
|---|---|
| `niimbot info` | 打印机状态**及已装入的耗材**：型号 id、序列号、固件、电量、标签类型、标签卷和碳带的剩余量 |
| `niimbot rfid` | 读取标签卷和碳带的 RFID 标签 |
| `niimbot text "一行" "..."` | 打印文字；每一行文字作为一个命令行参数 |
| `niimbot image file.png` | 打印图片 |
| `niimbot preview "一行"` | **只生成版面，不打印** |
| `niimbot testpage` | 打印机内置测试页（用于检查连接） |
| `niimbot scan` | 在附近搜索打印机 |

## 环境要求

- 装有 **BlueZ 的 Linux**（在 Debian 和 Ubuntu 上验证；与 BlueZ 的通信通过 D-Bus）；
- **Go 1.24+** 用于编译；
- 支持西里尔字母的字体（DejaVu、Noto 或 Liberation，会自动查找）；
- **NIIMBOT N1** 打印机（其他型号需要各自的打印头参数，见「协议」）。

## 安装

```bash
go install github.com/petrovich811/niimbot@latest
```

或者从源码编译：

```bash
git clone https://github.com/petrovich811/niimbot.git
cd niimbot
go build -o niimbot .
```

## 用法

```bash
# 查看打印机状态
./niimbot info

# 只生成版面（保存在 /tmp/niimbot_label_preview.png）
./niimbot preview "水泵" "12A-5"

# 打印文字
./niimbot text "水泵" "12A-5"

# 打印图片
./niimbot image logo.png

# 加浓一档，打印两份
./niimbot text "ГРК" "编号 4210" --density 3 --copies 2
```

### 参数

| 参数 | 含义 | 默认值 |
|---|---|---|
| `--address MAC` | 打印机地址；不填则按名称 `N1-` 搜索 | 自动 |
| `--length mm` | 标签在进纸方向上的长度 | `30` |
| `--font 点数` | 字号；`0` 表示按标签尺寸自动选择 | `0` |
| `--density 1..3` | 打印浓度 | `2` |
| `--label` | 标签类型；**留空表示按标签卷的 RFID 标签自动识别** | 自动 |
| `--copies N` | 打印份数 | `1` |
| `--flip` | 把内容旋转 180° | 关闭 |
| `-v=false` | 不输出详细的协议日志 | 开启 |

## 标签与方向

### 标签类型

NIIMBOT 协议定义了八种标签类型，但 **N1 只支持其中五种**
（[类型说明](https://printers.niim.blue/other/label-types/)）：

| 代码 | 类型 | N1 |
|---|---|---|
| 1 | 带间隙的普通标签（`withgaps`） | ✅ |
| 2 | 黑色热敏标签（`black`） | ❌ |
| 3 | 连续纸带（`continuous`） | ✅ |
| 4 | 打孔标签（`perforated`） | ❌ |
| 5 | 透明标签（`transparent`） | ✅ |
| 6 | PVC 挂牌（`pvctag`） | ❌ |
| 10 | 带黑色标记的标签（`blackmarkgap`） | ✅ |
| 11 | 热缩管（`heatshrink`） | ✅ |

`black`、`perforated` 和 `pvctag` 在协议中存在，但 N1 的固件不支持。
驱动会在**连接打印机之前**检查类型，型号不支持时直接拒绝打印。

适用于 N1 的标签宽度**不超过 15 mm**（可打印区域为中间 12 mm）：白色哑光
14×30、14×40、14×50 mm，哑光银色和透明 14×30 mm，以及 12,5×109 mm 的
四种颜色线缆标签。采用热转印方式，因此需要碳带。

### 耗材自动识别

每一卷耗材 —— 标签卷和碳带 —— 都带有 **RFID 标签**。打印机自己会读取，
驱动也可以查询：

```console
$ niimbot rfid

=== 标签卷 ===
  UUID:                  881dcce946121080
  条形码:                12242117
  序列号:                PC0H902384002378
  标签类型:              withgaps (1) — N1 支持
  用量:                  共 228，已用 43，剩余 185

=== 碳带 ===
  UUID:                  881d8209fa900000
  序列号:                PZ1GA06304000391
  用量:                  共 1600，已用 901，剩余 699
```

**标签类型会自动确定。** 如果没有给出 `--label`，驱动会读取标签卷的 RFID
标签并从中取得类型：打印机比任何猜测都更清楚里面装的是什么。如果标签中的
类型该型号不支持，打印不会开始。如果显式给出了 `--label` 且与标签不符，
驱动会给出提示：

```
提示：打印机中装的是 withgaps 标签，却按 continuous 打印
```

RFID 标签中也保存了标签尺寸，但打印机**不会把它传给主机**（厂商 App 是
按卷号从服务器获取尺寸的），所以长度仍然要用 `--length` 指定。

### 方向

N1 的打印头宽 **96 点** —— 在 203 dpi 下即 **12 mm**。本驱动使用
**EW14×30** 标签：宽 14 mm，进纸方向长 30 mm。可打印区域为标签中间的 12 mm。

**文字沿标签方向排列。** 为此，内容按「阅读方向」绘制：图像的宽度是标签
长度，高度是打印头宽度。然后把图像转换成打印机的行数据：

- **做转置** —— 图像的 X 轴变成进纸方向；
- **打印头轴要反向** —— 打印头第 `c` 点取图像第 `across-1-c` 行
  （在 NiimBlueLib 中即 `idx = (height-1-col)*width + row`）。

这两处细节都在真机上打印验证过，而且两处都曾是作者的错误：

1. 若按「屏幕上看到的样子」绘制，文字会**横跨**标签；
2. 不反转打印头轴，标签会打出来**镜像**。

如果标签打出来上下颠倒，加上 `--flip`，即旋转 180°。

## 协议

数据包：

```
55 55 | 命令 | 长度 | 数据 | XOR 校验 | AA AA
```

多字节整数为 **大端序**。校验和是命令、长度和所有数据字节的异或值。

打印流程：

```
浓度 (0x21) → 标签类型 (0x23) → 开始打印 (0x01) →
开始页 (0x03) → 页尺寸 (0x13) → 位图行 (0x85) →
结束页 (0xE3) → 查询状态 (0xA3) → 结束打印 (0xF3)
```

位图行：`位置(2) | 黑点计数(3) | 重复次数(1) | 数据(12)`，其中第一个字节的
第 7 位是打印头最外侧的点。全白行用单独的命令 `0x84` 发送 —— 更短也更快。

打印状态（响应 `0xB3`）：`页号(2) | 打印进度 % | 走纸进度 %`。当页号达到
目标且两个进度都是 100 时，打印完成。

### 两个坑

**1. 打印机有两个 BLE 服务，只有一个是可用的。**

| 服务 | 说明 |
|---|---|
| `e7810a71-73ae-499d-8c15-faa9aef0c3f2` | **NIIMBOT 原生服务** —— 固件监听的就是它，特征值 `bef8d6c9-9c21-4c9e-b632-bd58c1009f9f` |
| `49535343-fe7d-4ae5-8fa9-9fafd205e455` | 透传 UART —— 属性上符合要求（`notify` + `write`），但**完全不响应命令** |

连到 UART 看起来是成功的，但打印机毫无反应：命令都发进了「空气」。
调试时最耗时间的就是这一点。

**2. 需要先让 BlueZ「看到」设备。**

没有预先扫描，连接会以 D-Bus 错误失败：

```
Method "Get" with signature "ss" on interface "org.freedesktop.DBus.Properties" doesn't exist
```

另外对 Go 有一个重要细节：`tinygo.org/x/bluetooth` 的 Linux 后端中
`Adapter.Scan` **会阻塞直到 `StopScan`** —— 必须放在单独的 goroutine 中，
否则程序会卡死。

### 其他型号

参数是针对 N1 的：型号 id `3586`、打印头 `96` 点、浓度 `1..3`。
换用其他型号时，需在 `protocol.go` 中修改这些值（「型号 → 参数」对照表在
[NiimBlueLib](https://github.com/MultiMote/niimbluelib/blob/master/src/printer_models.ts) 中）。

## 测试

```bash
go test ./...
```

测试会核对数据包的组装 —— **与从真机抓取的字节逐一比对**（握手
`5555c10101c1aaaa`、型号查询 `555540010849aaaa`），还包括响应解析、
不完整数据包的拼接、黑点计数、行的方向和画布白底。

## 已验证与未验证

**已在真机 N1（GA24110447）上验证：**

- 搜索、连接和握手；
- 型号 id `3586`、序列号、固件 `3.13`、电量；
- **打印文字** —— 完整流程：`第 1 页，打印 100%，走纸 100%`；
- **打印图片** —— 边框、文字和角上的标记都与版面一致；
- **方向** —— 打印结果正确：文字沿标签方向，不是镜像。经过上面两处修正后
  在纸上确认；
- 两处方向错误都只有在打印时才暴露 —— 没有实物打印件是发现不了的。

**尚未验证：**

- 在 `withgaps` 以外的标签类型上打印；
- 其他 NIIMBOT 型号（需要各自的打印头参数）。

## 致谢

协议是参照开源库 **[NiimBlueLib](https://github.com/MultiMote/niimbluelib)**
（MIT）逆向得到的 —— 它是最精确的 NIIMBOT 协议开源实现。数据包格式、
命令码、黑点计数的拆分方式和型号参数都来自那里。
本驱动的代码是用 Go 从零编写的。

## 免责声明

本项目与 NIIMBOT 无关，也未获得厂商认可。「NIIMBOT」及各型号名称归其
各自所有者所有。驱动通过逆向得到的协议与打印机通信 —— 使用风险自负。

## 许可证

[MIT](LICENSE)。
