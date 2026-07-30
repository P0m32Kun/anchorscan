# AnchorScan SSL/TLS 覆盖研究

- 日期：2026-07-29
- 范围：只读盘点；不修改扫描规则，不对任何目标执行 TLS、证书或漏洞探测。
- 判定口径：只有“项目存在调度入口 + 当前配置会选中规则 + 可复核的工具/模板版本支持”同时成立，才称为默认覆盖。上游存在脚本或模板不等于当前覆盖。

## 结论

当前 AnchorScan 有 HTTPS/Web 识别与 Web 产品路由；TLS 协议/密码套件存在**外部模板集相关的条件路径**，但没有项目锁定、可重复的默认覆盖；证书与具体 TLS 漏洞也没有可证明的默认覆盖。不要把 `httpx` 成功访问 HTTPS、Nmap 的 `-sV`，或 Nuclei 上游模板库中存在 TLS 模板，表述为“已支持 SSL/TLS 检测”。

本研究**不建议创建或实现通用 `tls`/`ssl` Nuclei tag 规则**。模板集未由项目锁定，tag 不表达模板的连接次数或主动风险，且 TLS 配置/证书/漏洞三类需要不同证据、授权和停止条件。若未来获批准，应按单一目标拆分，并先固定工具与模板输入。

## 当前流水线与版本边界

1. `scanTarget` 使用 Nmap 指纹；`fingerprint.Classify` 仅在服务/产品含 HTTP 特征时设置 `IsWeb`，并在 `Tunnel == "ssl"`、`https` 或 `ssl/http` 服务名时生成 `https://IP:port` URL。
2. 只有 `fp.IsWeb` 才会执行 `EnrichWebWithOutput`。当前固定 httpx 参数是 `-json -silent -status-code -title -tech-detect -u <URL>`；配置 profile 只追加速率/线程，不追加 `-tls-grab` 或 `-jarm`。
3. Web 服务被 NSE 阶段显式跳过；NSE 仅按 `nse.yaml` 的 `Normalized` 服务名精确匹配。`nse.yaml` 没有 `ssl`、`https`、`http` 或任意 `ssl-*` 条目。
4. Nuclei 仅由 `service-tags.yaml` 的服务、产品或 httpx 技术栈匹配后，以 `nuclei_tags` 选模板；仓库没有 `tls` 或 `ssl` 路由。`http-generic` 的 `http,exposure,misconfig` 只是 HTTP Web 兜底，不能证明 TLS 模板被选中。
5. 本机工具版本：`/opt/homebrew/bin/nmap` 为 Nmap 7.99，`/Users/kun/.pdtm/go/bin/httpx` 为 v1.10.0，`/Users/kun/.pdtm/go/bin/nuclei` 为 v3.11.0；Nmap 数据目录包含 `ssl-cert`、`ssl-date`、`ssl-enum-ciphers`、`ssl-heartbleed`。项目没有锁定这些 PATH 二进制，也未提交或锁定 `nuclei-templates`。
6. 本机可见的外部模板副本含 `ssl/weak-cipher-suites.yaml`，其 ID 为 `weak-cipher-suites`、tags 为 `ssl,tls,misconfig,vuln`、`max-request: 4`，且未在可见 ignore 文件中出现。项目的 `http-generic` 会传入 `http,exposure,misconfig`，所以**只有在运行时选择了包含该模板的兼容、未忽略模板集时**，已分类 HTTPS/Web 服务可条件性选中此弱套件检查。模板副本无 Git commit；Nuclei 配置元数据仅报告 v10.4.6，且模板目录是外部用户配置，不是项目锁定输入。因此这不是稳定的默认覆盖，也不能扩大为一般 TLS 审计。

## 四类覆盖矩阵

| 类别 | 当前项目实际路径 | 当前结论 | 原生工具能力与限制 | 授权、风险与最小验证 |
| --- | --- | --- | --- | --- |
| HTTPS/Web 识别 | Nmap 指纹满足 Web 条件后，生成 URL；httpx 用 `-tech-detect` 获取技术栈；服务/产品/技术栈规则才路由 Nuclei。 | **部分覆盖：Web 识别/产品路由。** 不证明每个 TLS 服务都可被识别为 Web，也不证明 TLS 审计。 | Nmap `-sV` 识别服务；httpx 可做 HTTP(S) 请求与技术识别。 | 需要授权 IP、域名、Host/SNI；正常 HTTP 请求仍会产生访问日志或触发 WAF。优先非认证、低速、短超时。 |
| TLS 协议与密码套件 | 无 NSE 映射；httpx 未启用 TLS grab/JARM；HTTPS/Web 若归一化为 `http`，会走 `http-generic` 并传入 `http,exposure,misconfig`。 | **条件覆盖：仅外部模板集相关。** 可见外部副本中的 `weak-cipher-suites` 有 `misconfig` tag，若该模板集被运行时选中且未忽略，最多四个请求的弱套件检查可被条件选中；项目未锁定该输入，不能称稳定默认覆盖。 | Nmap `ssl-enum-ciphers` 可枚举协议与套件，`ssl-date` 可获取服务时间；httpx `-tls-grab` 是 TLS 握手信息采集，JARM 则主动发送 10 个不同 TLS ClientHello 生成指纹，二者都不能替代完整枚举。 | 完整枚举或 JARM 的负载高于一次握手，旧设备可能不稳定。需要书面配置审计授权、每主机低并发、明确握手/重试上限与失败不判弱。 |
| 证书 | 无 `ssl-cert` 映射；httpx 未启用 TLS 元数据采集；无可复核的 Nuclei 证书规则。 | **未接通/无默认覆盖。** | Nmap `ssl-cert` 可采集 X.509 主体、SAN、指纹和有效期；一次 HTTP 成功不等于链、主机名、信任或吊销验证。 | 单次握手通常低风险，但 SNI、URL host 与 IP 不同会改变证书。证据必须记录 SNI、端口、时间、链与验证策略，并按资产数据规则保存。 |
| 具体 TLS 漏洞 | 没有 `ssl-heartbleed` 或其他 TLS 漏洞 NSE 映射；没有 TLS Nuclei tag，也没有项目锁定的模板清单。 | **未接通/无默认覆盖。** Heartbleed 不应宣称已覆盖。 | Nmap 有 `ssl-heartbleed`，但单脚本仅对应单类漏洞；Nuclei 是否执行取决于已安装模板、tag、ignore、协议和 code 开关。 | 某些检查会发送异常握手或带有利用性质的请求。要求 CVE/目标/端口白名单、书面漏洞验证授权、变更窗口、低速、短超时、停止条件与人工复核；禁止自动 exploit、DoS、fuzz、爆破。 |

## 可复现证据

### 本地代码与配置

- `internal/fingerprint/classify.go`：Web/TLS URL 分类条件。
- `internal/app/scan_target.go`：httpx 只对 `fp.IsWeb` 执行；Web 端口跳过 NSE；Nuclei 只按匹配 tags 调用。
- `internal/tools/httpx.go`：默认 httpx 参数不含 TLS grab/JARM。
- `config/default.yaml.example`：httpx 只有 rate-limit/threads，Nuclei 只有 rate-limit/concurrency/retries。
- `config/nse.yaml`：没有任何 `ssl-*` 脚本映射。
- `config/service-tags.yaml`：没有 `ssl`/`tls` 路由；`http-generic` 会传 `http,exposure,misconfig`，只形成依赖外部模板集的条件路径；文件明确默认仅按 tags，精确模板需单工具显式指定。
- `internal/vuln/nuclei_tags.go`：tag 是路由选择，而不是 TLS 检测能力的证明。

只读复核命令与预期：

```sh
command -v nmap httpx nuclei
nmap --version | head -1
httpx -version 2>&1 | tail -1
nuclei -version 2>&1 | rg 'Engine Version'
rg -n 'ssl|https|http' config/nse.yaml
rg -n -C 2 'http-generic|ssl|tls' config/service-tags.yaml
rg -n -C 3 '^(id:|tags:|metadata:|ssl:)' "$HOME/nuclei-templates/ssl/weak-cipher-suites.yaml"
cat "$HOME/Library/Application Support/nuclei/.templates-config.json"
rg -n -i 'weak-cipher-suites|ssl/weak-cipher-suites|tls/' \
  "$HOME/Library/Application Support/nuclei/.nuclei-ignore" || true
```

预期分别显示三个 PATH、Nmap 7.99/httpx v1.10.0/Nuclei v3.11.0、NSE 无 TLS 映射、`http-generic` 的 `misconfig` tag，以及外部副本的 `ssl,tls,misconfig,vuln`。templates 配置 JSON 必须记录运行时目录、templates 版本和 ignore hash；ignore 搜索无命中才可称该规则未被该运行时 ignore 文件排除。当前元数据目录与 `$HOME/nuclei-templates` 外部副本不是同一输入，且前者指向的目录可能不存在；所以这些命令仍不能将外部副本升级为当前默认覆盖，只能使条件边界可重复核验。

### 一手来源

- Nmap NSE 使用与脚本选择：<https://nmap.org/book/nse-usage.html>
- Nmap `ssl-cert`：<https://nmap.org/nsedoc/scripts/ssl-cert.html>
- Nmap `ssl-enum-ciphers`：<https://nmap.org/nsedoc/scripts/ssl-enum-ciphers.html>
- Nmap `ssl-date`：<https://nmap.org/nsedoc/scripts/ssl-date.html>
- Nmap `ssl-heartbleed`：<https://nmap.org/nsedoc/scripts/ssl-heartbleed.html>
- httpx 官方 Usage（`-tech-detect`、`-tls-grab`、`-jarm`）：<https://github.com/projectdiscovery/httpx#usage>
- Salesforce JARM（10 个 ClientHello 的主动指纹模型，固定本次查询 HEAD `2c0cf5ce8418c7a1d03edb219acea3c18e068289`）：<https://github.com/salesforce/jarm>
- Nuclei HTTP、network、code 协议与执行选择：<https://docs.projectdiscovery.io/templates/protocols/http>、<https://docs.projectdiscovery.io/templates/protocols/network>、<https://docs.projectdiscovery.io/templates/protocols/code>、<https://docs.projectdiscovery.io/opensource/nuclei/running>

## 允许的后续方向（尚未立项）

只有在以下前置条件全部完成后，才应创建单独的行为变更任务：

1. 固定 Nmap/httpx/Nuclei 版本及 Nuclei templates 的版本/commit，并保存实际选择到的模板清单、ignore 状态和命令。
2. 只选择一个目标：例如“授权范围内的证书采集”或“明确 CVE 的低风险验证”；不得合并为笼统的 SSL 支持。
3. 以 loopback TLS fixture 验证 SNI、非 TLS、过期/自签（仅在可控证书下）、连接失败和超时；不连接真实未授权资产。
4. 定义每主机握手/请求预算、并发、超时、重试、禁止模板类别、停止条件、provenance 和误报回退。
5. 漏洞类还必须有 CVE 白名单、变更窗口和人工复核；配置/证书类不能把不可协商、私有 CA 或名称不匹配直接判为漏洞。

在这些条件之前，正确状态是“覆盖未接通/未证实”，而不是缺省启用宽泛 TLS tags。

## 后续任务建议（不在本研究中创建）

| 优先级 | 候选独立任务 | 可验收范围 | 不能做什么 |
| --- | --- | --- | --- |
| P1 | 锁定外部 TLS 检测输入 | 固定 Nmap/httpx/Nuclei 二进制与 templates commit；记录运行时目录、tag/template 选择、ignore、版本和 provenance。验证时必须证明当前 `http-generic` 条件路径实际选中的模板列表。 | 不新增目标网络请求，不把上游模板自动接入默认扫描。 |
| P2 | 授权 TLS 证书采集 | 仅对明确 allowlist 的 TLS 服务运行一次 `ssl-cert` 或等价单握手采集；持久化 SNI/端口/时间/链证据，并用 loopback fixture 验证。 | 不把自签、私有 CA、名称错配或握手失败直接报为漏洞；不枚举密码套件。 |
| P3 | 授权 TLS 协议与套件基线 | 仅对已批准资产执行低并发 `ssl-enum-ciphers`，输出协议/套件事实和超时/停止记录。 | 不将扫描失败视为弱配置；不与证书或 CVE 检查混合上线。 |
| P4 | 单一 CVE TLS 验证 | 一个已明确的 CVE、模板/脚本版本、目标端口白名单、变更窗口和人工复核构成一个任务。 | 不创建“通用 TLS 漏洞扫描”或自动 exploit/DoS/fuzz 任务。 |

每一项均须在创建实施任务前重新取得用户批准；当前没有证据支持直接启用任何规则。

## 非范围

- 不包含 Spark 路由或规则。
- 不执行 `ssl-enum-ciphers`、`ssl-cert`、`ssl-heartbleed`、Nuclei TLS 模板或真实网络扫描。
- 不把本机安装工具或上游模板的存在当作项目默认覆盖。
