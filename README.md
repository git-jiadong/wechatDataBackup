# wechatDataBackup
PC微信聊天记录数据导出工具

* 基于wails开发 + React前端，实现PC端微信聊天记录一键导出功能。
* 导出后数据可以做永久化保存，即使微信停止支持，聊天记录也可以随时查看。
* 前端界面尽量与微信界面保持一致，减少使用成本。
* 理论上支持所有Windows 32/64位微信版本。

效果图如下：

![](./res/result.png)

## 使用方法
1. 下载release可执行文件直接打开
2. 下载源码自行编译可执行文件 [安装wails环境](https://wails.io/zh-Hans/docs/gettingstarted/installation)
```shell
git clone https://github.com/git-jiadong/wechatDataBackup.git
cd wechatDataBackup
wails build
```
编译成功后在可执行二进制文件路径`build\bin\wechatDataBackup.exe`

3. 导出聊天记录
电脑登陆微信，然后打开`wechatDataBackup.exe`后按照如图提示导出
![](./res/tips.png)

## 功能
本项目目前的规划与实现进度：
- [x] 支持图片消息
- [x] 支持视频消息
- [x] 支持链接消息
- [x] 支持文件消息
- [x] 支持原始表情显示
- [x] 支持按类型检索
- [x] 支持日期检索
- [x] 支持按群成员检索
- [x] 支持增量式导出
- [ ] 支持更多消息类型显示
- [ ] 图片查看器重绘
- [ ] 实现头像或表情预先下载（实现完全离线查看）
- [ ] 聊天报告
- [ ] AI本地模型应用
- [ ] 导出数据本地加密
- ...
如果遇到什么问题，或者有更好的建议与优化点欢迎给作者提 [ISSUE](https://github.com/git-jiadong/wechatDataBackup/issues)


### 常见问题
Q: 支持手机端的聊天记录备份吗？<br>
A: 手机端可以使用聊天数据迁移功能，将手机的数据迁移到电脑后再将数据导出 [迁移聊天记录](https://mp.weixin.qq.com/s?src=11&timestamp=1724572247&ver=5465&signature=j1TNxZAx48TdBzc6KJIHInIvXlBhwSAlQ4XGowKeyijZ2gsmXyOb2Zpg9qfVyMdGrte0SwU9kT8xCDgFBI7CR7fqCHpHuZzpv3O77gSkV3mbxmFdPKfW7Fq89CzHPQr0&new=1)<br>
Q: 导出的数据比PC微信里面看到的少,数据不完整<br>
A: 这是由于可能数据存在于内存中还没有回写到磁盘导致的，退出微信时会将内存的数据全部回写到磁盘，导出数据时最好退出重新登陆一次微信，保证数据都在磁盘中再导出即可。<br>

## 免责声明
**⚠️ 本项目仅供学习、研究使用，严禁商业使用**<br/>
**⚠️ 用于网络安全用途的，请确保在国家法律法规下使用**<br/>
**⚠️ 本项目完全免费，问你要钱的都是骗子**<br/>
**⚠️ 使用本项目初衷是作者研究微信数据库的运行使用，您使用本软件导致的后果，包含但不限于数据损坏，记录丢失等问题，作者不承担相关责任。**<br/>
**⚠️ 因软件特殊性质，请在使用时获得微信账号所有人授权，你当确保不侵犯他人个人隐私权，后果自行承担**<br/>

## 前端代码
由于前端代码不成熟，前端界面代码暂时不公开。

## 参考/引用
- 微信数据库解密和数据库的使用 [PyWxDump](https://github.com/xaoyaoo/PyWxDump/tree/master)
- silk语音消息解码 [silk-v3-decoder](https://github.com/kn007/silk-v3-decoder)
- PCM转MP3 [lame](https://github.com/viert/lame.git)
- Dat图片解码 [wechatDatDecode](https://github.com/liuggchen/wechatDatDecode)