# 07 — 打通命令、工具页与 Evidence 返回链路

**What to build:** 让现有生成命令/带参数打开工具页同时携带 Project、Zone、Verification 和安全返回地址，工具完成后回到原验证项上传证据。

**Blocked by:** 05 — 正向工作台；06 — 负向工作台。

**Status:** draft

- [ ] 复用现有单条/批量 Nuclei、Nmap、MSF 命令构建器，不建第二套命令逻辑。
- [ ] 工具页自动选择 Project/Zone，并显示验证项摘要。
- [ ] Tool Run 保存 kind=tool、Zone 和验证来源，默认不纳入正式报告。
- [ ] return_to 只接受本站 Project 相对路径，拒绝外部跳转。
- [ ] 完成页提供返回、复制输出和粘贴/上传 Evidence。
- [ ] 工具退出成功不能自动确认漏洞存在或未发现。
