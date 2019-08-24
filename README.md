# sock-cuppa

cuppa是一种简单的tcp转发器，你只需要配置一下你的参数，即可任意帮你把数据转发到目的地

### 1. 安装

```
go get github.com/heshaobo/sock-cuppa
cd ~/go/src/github.com/heshaobo/sock-cuppa
go build cuppa.go
```

### 2. 配置文件修改

```
{
    "remote_addr": "IP:Port",   //请修改为你需要转发的目的IP和端口
    "local_port": 10000,        //请修改为你本地监听的端口，默认10000
    "enable_report": false,     //报告推送，使用server酱微信推送，默认关闭
    "monitor_interval": 3,      //目的地址探测间隔时间（单位：秒），默认每3秒会探测一次目的IP
    "monitor_timeout": 3,       //目的IP探测超时时间（单位：秒）, 默认3秒超时
    "report_interval": 300,     //探测报告汇总时间（单位：秒）,默认每5分钟分析一次探测的失败率，超过设定的阈值会推送报告
    "report_threshold": 1,      //失败率阈值（单位：百分比），汇总失败率超过此阈值推送报告 ，默认1%
    "log_path": "cuppa.log",    //日志路径，默认当前目录
    "push_url": ""              //server酱推送地址，不如使用推送功能可不填，server酱官网：http://sc.ftqq.com/3.version
}
```

### 3. 启动
你可以前config.json保存在别的目录下进行修改，修改后使用启动命令,-c参数指定配置文件路径，不指定默认在当前目录下查找config.json, 使用&符号使进程在后台运行
```
./cuppa -c /youtpath/config.json &
```

### 4. 使用
现在你可以在任何地方使用TCP连接这个服务了，它将会帮你自动转发到remote_addr

enjoy it！
