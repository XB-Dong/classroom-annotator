# P0.1 Linux 发布产物

Windows 上 Linux 交叉编译的两次受控尝试分别达到300秒和600秒上限，未获得发布产物；没有把超时写成编译通过，也没有据此认定业务代码缺陷。停止重复同环境构建，改由现有 GitHub Actions Linux 验证流程保存已经通过全套验证的产物。

Common verification 的功能、集成、竞态与浏览器检查继续执行，不跳过失败。仅在整个脚本成功后，把四个业务 CLI、前端构建结果、完整源码提交号及二进制摘要复制到 runner 临时目录，并通过固定版本的官方 upload-artifact Action 保留7天。没有视频、数据库、认证文件或测试 DSN。

Leader 下载产物后必须核对成功 workflow 的 headSha、产物内 source-sha.txt 与 SHA256SUMS，再与相同提交的 Git archive 组成发布包。只有通过既定发布门槛才进行数据库切换。仓库私有不改变这条路径，但云端和内网搬运仍需要获授权的仓库访问权限。
