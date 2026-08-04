# bug记录

1. [2026-07-30 14:11:01.793 UTC+8] [info] nuclei: nuclei https://190.30.1.1:2224 tags=[http exposure misconfig ssl]
[2026-07-30 14:11:01.987 UTC+8] [error] nuclei: nuclei https://190.30.1.1:2224 failed: Could not run nuclei: no templates provided for scan，从日志来看，针对ssl的-tags有问题，导致找不到模板,所以我认为针对ssl的就只添加 -tags ssl，不要追加一堆别的
2. nuclei 默认模版位置怎么变了，在哪设置的，[ERR] Could not find template '/tmp/does-not-exist': no templates found in path "/tmp/does-not-exist"，应该默认位置是 ~/nuclei-templates/
