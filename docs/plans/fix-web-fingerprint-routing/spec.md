# Web 指纹路由与高危端口补全

## 目标

- 高危端口预设包含用户漏洞清单中缺失的 14 个端口：25、80、111、161、443、513、514、901、1099、6666、8009、8161、9092、12345。
- HTTP 服务先按 Nmap 产品或 httpx 技术栈选择具体 Web 产品规则；未识别产品时才使用通用 HTTP 规则。
- 非 Tomcat Web 服务不得命中 Tomcat 标签，真实 Tomcat 仍须命中 Tomcat 标签。
- 使用本机真实 Nmap、httpx、Nuclei 和本地容器靶场完成验证。

## 验收

1. 默认高危端口文件包含上述端口，且不删除原有端口。
2. 默认规则下，IIS 指纹选择 `iis`，Tomcat 指纹选择 `tomcat`，未知 HTTP 选择 `http-generic`。
3. 本地普通 Web 靶场不产生 Tomcat 路由，Tomcat 靶场正确选择 Tomcat 路由并能运行 Tomcat 标签的 Nuclei 模板。

## 非目标

- 本工单不扩展原漏洞清单的 NSE/Nuclei 模板矩阵。
- 本工单不修改 Nmap、httpx 或 Nuclei 本身的识别逻辑。
