# dufw
根据DDNS的记录去动态调整UFW的规则，用于某些特定场景的需求。其他防火墙直接修改命令即可！

效果如下所示：
![1.png](img%2F1.png)

ufw status 如下：
![2.png](img%2F2.png)

正好我测试的域名是由CDN接管的，所以每次IP可能不一样，正好满足测试需求！

