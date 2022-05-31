# SWAGGER OPENAPI

Spiderpool uses go-swagger to generate open api source codes. There are two swagger yaml for 'agent' and 'controller'. Please check
with [agent-swagger spec](https://github.com/spidernet-io/spiderpool/blob/main/api/v1/agent/openapi.yaml) and
[controller-swagger spec](https://github.com/spidernet-io/spiderpool/blob/main/api/v1/controller/openapi.yaml).
source codes.

## Features

* Validate spec
* Generate C/S codes
* Verify spec with current source codes
* Clean codes
* Use swagger-ui to analyze the given specs.

## Usages

There are two ways for you to get access to the features.

* Use `makefile`, it's the simplest way.
* Use shell `swag.sh`. The format usage for 'swag.sh' is `swag.sh $ACTION $SPEC_DIR`.

### validate spec

Validate the current spec just give the second parameter with the spec directory.

```shell
./tools/scripts/swag.sh validate ./api/v1/agent
```

Or you can use `makefile` to validate the spiderpool agent and controller with the following command.  

```shell
make openapi-validate-spec
```

### generate source codes with the given spec

To generate agent source codes:

```shell
./tools/scripts/swag.sh generate ./api/v1/agent
```

Or you can use `makefile` to generate for both of agent and controller two:

```shell
make openapi-code-gen
```

### verify the spec with current source codes to make sure whether the current source codes is out of date

To verify the given spec whether valid or not:

```shell
./tools/scripts/swag.sh verify ./api/v1/agent
```

Or you can use `makefile` to verify for both of agent and controller two:

```shell
make openapi-verify
```

### clean the generated source codes

To clean the generated agent codes:

```shell
./tools/scripts/swag.sh verify ./api/v1/agent
```

Or you can use `makefile` to clean for both of agent and controller two:

```shell
make clean-openapi-code
```

### Use swagger-ui

To analyze the defined specs in your local environment with docker:

```shell
make openapi-ui
```

Then you can visit the web with port 8080. Switch the yaml with './agent-swagger.yaml' and './controller-swagger.yaml' in the web.

## Steps For Developers

1. Modify the specs: [agent-swagger spec](https://github.com/spidernet-io/spiderpool/blob/main/api/v1/agent/openapi.yaml) and
   [controller-swagger spec](https://github.com/spidernet-io/spiderpool/blob/main/api/v1/controller/openapi.yaml)
2. Validate the modified specs
3. Use swagger-ui to check the effects in your local environment with docker
4. Re-generate the source codes with the modified specs
5. Commit your PR.
