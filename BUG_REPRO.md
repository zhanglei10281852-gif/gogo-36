# Bug Reproduction

## 包的性质

当前 test_model_fix 保存的是被测模型修复后的结果源码，不是初始含 Bug 源码。要复现原始缺陷，必须检出下面固定的 parent SHA；不要在当前修复结果源码上期待重新出现修复前失败。生成系统使用的可信验证补丁和完整验证日志仅在本地留存，不提交到结果分支。

## 问题现象

父 capability grant 只允许 repos/*/private/**，却能委派出覆盖 repos/** 的子 grant，使子 grant 可访问父范围之外的仓库内容。请修复委派范围包含关系，拒绝扩大或语义不确定的 wildcard 子范围，同时保留真正收窄的 literal 和 glob 委派。修复后保证全量测试通过。

## 含 Bug 版本

- 仓库：zhanglei10281852-gif/gogo-36
- 仓库地址：https://github.com/zhanglei10281852-gif/gogo-36.git
- parent SHA：bca41421ff01bbe7ecf22679092c23dd8546a76f

## 复现步骤

```bash
git clone -- https://github.com/zhanglei10281852-gif/gogo-36.git bug-repro
cd bug-repro
git checkout --detach bca41421ff01bbe7ecf22679092c23dd8546a76f
go test ./identity -run "^TestDelegateGrantRejectsBroaderWildcardScope$" -count=1 -v
```

## 双架构完整错误信息

### linux/amd64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./identity -run "^TestDelegateGrantRejectsBroaderWildcardScope$" -count=1 -v
=== RUN   TestDelegateGrantRejectsBroaderWildcardScope
    grant_test.go:146: broader wildcard scope delegation should fail: <nil>
--- FAIL: TestDelegateGrantRejectsBroaderWildcardScope (0.00s)
FAIL
FAIL	agentguard/identity	0.002s
FAIL

```

stderr：

```text
(empty)
```

### linux/arm64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./identity -run "^TestDelegateGrantRejectsBroaderWildcardScope$" -count=1 -v
=== RUN   TestDelegateGrantRejectsBroaderWildcardScope
    grant_test.go:146: broader wildcard scope delegation should fail: <nil>
--- FAIL: TestDelegateGrantRejectsBroaderWildcardScope (0.02s)
FAIL
FAIL	agentguard/identity	0.144s
FAIL

```

stderr：

```text
(empty)
```

## 通过条件

repos/*/private/** 不得包含 repos/**；合法 literal/严格收窄范围保持可委派；双架构定向/全量/build/vet 通过。
