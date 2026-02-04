# broker
用于连接交易所 进行http/ws的public/private访问

## 文件说明

base 基础功能函数

ex 各交易所接口具体实现

client 通用http ws类

base_config broker的配置文件

## 开发要求
1、区分public和private 可以n个public和m个private 数量可以不一样 

2、n的数量可以大于m 比如4个bbo行情实例 1个交易实例

3、减少运行时创建新的变量 初始化时把所有需要用到的东西都创建好 

4、启动时获取本交易所所有可交易品种的信息 以备不时之需

5、禁止原汁原味获取所有字段 取消exchange和framework拆分 直接在解析json的地方仅获取需要用的信息 不用的一律丢弃 需要用再加

6、默认优先调整为 单向持仓模式 全仓保证金模式 杠杆拉到允许范围的最高

7、ticker一定要用该盘口ws最快的channel来更新

## 接口划分


1、 private 接口比较多 需要 区分2类 初始化的时候需传入pair 并获取所有pair的交易规则 pair转换为symbol 作为该broker实例的主要关注pair
    
    3.1 下单、查单、撤单、仓位 和symbol相关的 不传symbol 使用默认symbol 传symbol 使用传入的symbol
    
    3.2 全部仓位 全部撤单 账户资金 全部平仓 等和symbol无关的或作用到全部symbol的
    交易所推送过来涉及到主要关注pair的信息 需要单独用变量保存 以便高频访问 其他信息放在map里面 以被偶发性使用


