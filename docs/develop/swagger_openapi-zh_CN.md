# SWAGGER OPENAPI

[**English**](./swagger_openapi.md) | **简体中文**

Spiderpool 使用 go-swagger 生成 open api 源码。'agent' 和 'controller' 各有一个 swagger yaml，请参考
[agent-swagger spec](https://github.com/spidernet-io/spiderpool/blob/main/api/v1/agent/openapi.yaml) 和
[controller-swagger spec](https://github.com/spidernet-io/spiderpool/blob/main/api/v1/controller/openapi.yaml)。

## 功能

* 校验 spec
* 生成 C/S 代码
* 校验 spec 与当前源码是否一致
* 清理代码
* 使用 swagger-ui 分析给定的 spec

## 使用方法

有两种方式使用上述功能。

* 使用 `makefile`，这是最简单的方式。
* 使用 shell 脚本 `swag.sh`，其用法格式为 `swag.sh $ACTION $SPEC_DIR`。

### 校验 spec

校验当前 spec，只需将 spec 目录作为第二个参数传入。

```shell
./tools/scripts/swag.sh validate ./api/v1/agent
```

也可以使用 `makefile`，通过以下命令同时校验 spiderpool agent 和 controller。  

```shell
make openapi-validate-spec
```

### 根据给定的 spec 生成源码

生成 agent 源码：

```shell
./tools/scripts/swag.sh generate ./api/v1/agent
```

也可以使用 `makefile` 同时为 agent 和 controller 生成源码：

```shell
make openapi-code-gen
```

### 校验 spec 与当前源码，确认当前源码是否过期

校验给定的 spec：

```shell
./tools/scripts/swag.sh verify ./api/v1/agent
```

也可以使用 `makefile` 同时校验 agent 和 controller：

```shell
make openapi-verify
```

### 清理生成的源码

清理生成的 agent 代码：

```shell
./tools/scripts/swag.sh clean ./api/v1/agent
```

也可以使用 `makefile` 同时清理 agent 和 controller 的代码：

```shell
make clean-openapi-code
```

### 使用 swagger-ui

在本地使用 docker 分析定义的 spec：

```shell
make openapi-ui
```

然后通过 8080 端口访问网页，在网页中切换 './agent-swagger.yaml' 和 './controller-swagger.yaml'。

## 开发者步骤

1. 修改 spec：[agent-swagger spec](https://github.com/spidernet-io/spiderpool/blob/main/api/v1/agent/openapi.yaml) 和
   [controller-swagger spec](https://github.com/spidernet-io/spiderpool/blob/main/api/v1/controller/openapi.yaml)
2. 校验修改后的 spec
3. 在本地使用 docker 通过 swagger-ui 检查效果
4. 根据修改后的 spec 重新生成源码
5. 提交 PR。
