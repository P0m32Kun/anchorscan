# bug记录

## 前端bug

1. github action构建的时候运行的测试都是必要的么？有些测试我们本地测试就好了吧，github action主要负责的是打包问题吧。
2. 
```bash
[2026-07-29T07:18:40.71416Z] [info] dameng-probe: dameng-probe 190.20.1.19:15026 (nmap service="" product="")
[2026-07-29T07:18:43.716985Z] [info] dameng-probe: dameng-probe 190.20.1.19:15027 (nmap service="" product="")
[2026-07-29T07:18:46.720265Z] [info] dameng-probe: dameng-probe 190.20.1.19:15030 (nmap service="" product="")
[2026-07-29T07:18:49.723594Z] [info] dameng-probe: dameng-probe 190.20.1.19:19802 (nmap service="" product="")
```
这种日志太多了，是不是没有展示的必要
3. 日志时间也是不对的，应该使用上海时间

## 扫描逻辑

1. 现在有ssl相关的规则么，目前看到命中的多的都是ssh，tomcat，其他的几乎没怎么见到
2. spark 似乎没有相应的规则

## 需求增加
1. 扫描报告应该添加反向筛选，当很多端口都无法识别出服务的时候，会存在大量服务名称为"tcpwrapped"的记录，如果没有过滤功能将淹没真正被识别到的服务，导致用户只能靠手动翻页来找出现了哪些服务；或者能够自动识别出出现了哪些类型的服务，让用户可以手动筛选想看哪些识别出来的服务名。
2. 项目内置nuclei模板是什么时候引入的错误决策，还有没有残留的测试或者配置存在？
3. 添加nmap无法识别的服务指纹用nuclei进行通用指纹识别增强，效仿现在的达梦数据库识别方案
