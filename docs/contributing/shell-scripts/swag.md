# swagger openapi

## Features

* Validate spec
* Generate C/S codes
* Verify spec with current source codes
* Clean codes

## Usages

The format usage for swag.sh is `swag.sh $ACTION $SPEC_DIR`

### validate spec

validate the current spec just give the second param with the spec dir.
> ./tools/scripts/swag.sh validate ./api/v1beta/spiderpool-agent

Or you can use `makefile` to validate the spiderpool agent and controller with the following command.  
> make openapi-validate-spec

### generate source codes with the given spec

To generate agent source codes:
> ./tools/scripts/swag.sh generate ./api/v1beta/spiderpool-agent

Or you can use `makefile` to generate for both of agent and controller two:
> make openapi-code-gen

### verify the spec with current source codes to make sure whether the current source codes is out of date

To verify the given spec whether valid or not:
> ./tools/scripts/swag.sh verify ./api/v1beta/spiderpool-agent

Or you can use `makefile` to verify for both of agent and controller two:
> make openapi-verify

### clean the generated source codes

To clean the generated agent codes:
> ./tools/scripts/swag.sh verify ./api/v1beta/spiderpool-agent

Or you can use `makefile` to clean for both of agent and controller two:
> make clean-openapi-code
