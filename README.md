# ecspresso

ecspresso is a deployment tool for Amazon ECS.

(pronounced same as "espresso")

# Usage

```
$ ecspresso
  -cluster string
    	ECS cluster name(required)
  -service string
    	ECS service name(required)
  -task-definition string
    	task definition path(required)
  -timeout int
    	timeout (sec) (default 300)
```

ecspresso works as below.

- Register a new task definition from JSON file.
  - JSON file is same format as `aws ecs describe-task-definition` output.
  - Replace `{{ env "FOO" "bar" }}` syntax in the JSON file to environment variable "FOO".
    - If "FOO" is not defined, replaced by "bar"
  - Replace `{{ must_env "FOO" }}` syntax in the JSON file to environment variable "FOO".
    - If "FOO" is not defined, abort immediately.
- Update a service definition.
- Wait a service stable.

## Example

```
$ ecspresso -cluster default -service myService -task-definition app.json
2017/11/07 09:07:12 myService/default Starting ecspresso
2017/11/07 09:07:12 myService/default Creating a new task definition by app.json
2017/11/07 09:07:12 myService/default Registering a new task definition...
2017/11/07 09:07:15 myService/default Task definition is registered myService:2
2017/11/07 09:07:15 myService/default Updating service...
2017/11/07 09:07:16 myService/default Waiting for service stable...(it will takea few minutes)
2017/11/07 09:10:02 myService/default Service is stable now. Completed!
```

# Requirements

- aws-cli

# LICENCE

MIT

# Author

KAYAC Inc.