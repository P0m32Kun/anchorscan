<!-- anchorscan-catalog
version: 1
-->

### SMB 签名未启用（中危）

<!-- anchorscan-entry
id: smb-signing
aliases: [SMB signing]
match:
  nuclei: [smb-signing]
  nse: []
  manual-review: []
  cve: [CVE-2024-0001]
-->

#### 漏洞描述

描述。

#### 扩展知识

该小节不属于固定展示字段。

#### 验证命令

##### Nuclei

```bash
nuclei -t network/smb.yaml -u {{host}}:{{port}}
```

#### 修复建议

启用签名。
